package persistence

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	cache2 "github.com/xsxdot/aio/internal/cache"
	"github.com/xsxdot/aio/pkg/common"
)

/*
 AOFManager 管理AOF持久化
 1. 优化重写逻辑，不能所有的命令都写为SET，2. 类似redis的双写机制
 2. 统一命令的转换，从Value转为command、command转为resp等等
*/

// AOFManager 管理AOF持久化
type AOFManager struct {
	config       *cache2.Config
	aofFile      *os.File
	aofBuf       *bufio.Writer
	aofLock      sync.Mutex
	aofRewriteCh chan struct{}
	wg           *sync.WaitGroup
	shutdownCh   chan struct{}
	cmdExecutor  CommandExecutor
	// 重写相关字段
	rewriteActive bool           // 是否正在进行重写
	rewriteBuffer *[][]byte      // 重写期间的命令缓冲区
	loaded        bool           // 标记AOF文件是否已加载
	logger        *common.Logger // 日志记录器
}

// CommandExecutor 命令执行器接口
type CommandExecutor interface {
	// ExecuteCommand 执行命令
	ExecuteCommand(cmd Command) error
	// GetAllData 获取数据库中的所有数据
	GetAllData() (map[string]cache2.Value, map[string]time.Time)
}

// Command 表示一个命令
type Command struct {
	Name string
	Args []string
}

// NewAOFManager 创建一个新的AOF管理器
func NewAOFManager(config *cache2.Config, executor CommandExecutor, wg *sync.WaitGroup) (*AOFManager, error) {
	am := &AOFManager{
		config:       config,
		aofRewriteCh: make(chan struct{}, 1),
		wg:           wg,
		shutdownCh:   make(chan struct{}),
		cmdExecutor:  executor,
		logger:       common.GetLogger(), // 初始化logger
	}

	// 如果不启用AOF，直接返回
	if !config.EnableAOF {
		return am, nil
	}

	// 确保AOF目录存在
	if err := EnsureDir(config.AOFFilePath); err != nil {
		return nil, fmt.Errorf("error ensuring AOF directory: %v", err)
	}

	// 打开AOF文件
	file, err := os.OpenFile(config.AOFFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("error opening AOF file: %v", err)
	}

	am.aofFile = file
	am.aofBuf = bufio.NewWriter(file)

	return am, nil
}

// Start 启动AOF管理器
func (am *AOFManager) Start() error {
	// 如果不启用AOF，不需要执行任何操作
	if !am.config.EnableAOF {
		return nil
	}

	// 不再自动加载AOF，这部分已经移到PersistenceManager的Start方法中处理
	// 避免重复加载

	// 如果AOF同步策略是每秒同步，启动同步goroutine
	if am.config.AOFSyncStrategy == 1 {
		am.wg.Add(1)
		go am.periodicAOFSync()
	}

	// 启动AOF重写goroutine
	am.wg.Add(1)
	go am.aofRewriteWorker()

	return nil
}

// WriteCommand 将命令写入AOF文件
func (am *AOFManager) WriteCommand(cmd Command) error {
	// 如果不启用AOF，不需要执行任何操作
	if !am.config.EnableAOF || am.aofFile == nil {
		return nil
	}

	// 只记录写命令
	if isReadOnlyCommand(cmd.Name) {
		return nil
	}

	am.aofLock.Lock()
	defer am.aofLock.Unlock()

	// 将命令转换为RESP格式
	respCmd := encodeCommandToRESP(cmd.Name, cmd.Args)

	// 如果正在进行AOF重写，将命令添加到重写缓冲区
	if am.rewriteActive && am.rewriteBuffer != nil {
		*am.rewriteBuffer = append(*am.rewriteBuffer, respCmd)
	}

	// 写入AOF文件
	_, err := am.aofBuf.Write(respCmd)
	if err != nil {
		return fmt.Errorf("error writing to AOF file: %v", err)
	}

	// 立即刷新缓冲区
	if err := am.aofBuf.Flush(); err != nil {
		return fmt.Errorf("error flushing AOF buffer: %v", err)
	}

	// 根据同步策略进行同步
	switch am.config.AOFSyncStrategy {
	case 2: // always
		if err := am.aofFile.Sync(); err != nil {
			return fmt.Errorf("error syncing AOF file: %v", err)
		}
	case 1: // everysec
		// 每秒同步由后台goroutine处理
		return nil
	default: // 0: no
		return nil
	}

	return nil
}

// syncAOF 同步AOF缓冲区到磁盘
func (am *AOFManager) syncAOF() error {
	if err := am.aofBuf.Flush(); err != nil {
		return fmt.Errorf("error flushing AOF buffer: %v", err)
	}
	return am.aofFile.Sync()
}

// Sync 提供外部接口同步AOF文件
func (am *AOFManager) Sync() error {
	if !am.config.EnableAOF || am.aofFile == nil {
		return nil
	}

	am.aofLock.Lock()
	defer am.aofLock.Unlock()

	return am.syncAOF()
}

// periodicAOFSync 定期同步AOF文件
func (am *AOFManager) periodicAOFSync() {
	defer am.wg.Done()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-am.shutdownCh:
			return
		case <-ticker.C:
			am.aofLock.Lock()
			if am.aofBuf != nil {
				if err := am.syncAOF(); err != nil {
					am.logger.Errorf("Error syncing AOF file: %v", err)
				}
			}
			am.aofLock.Unlock()
		}
	}
}

// aofRewriteWorker AOF重写工作协程
func (am *AOFManager) aofRewriteWorker() {
	defer am.wg.Done()

	ticker := time.NewTicker(time.Second * 10)
	defer ticker.Stop()

	for {
		select {
		case <-am.shutdownCh:
			return
		case <-ticker.C:
			// 检查是否需要重写
			if am.shouldRewriteAOF() {
				if err := am.RewriteAOF(); err != nil {
					am.logger.Errorf("Error rewriting AOF file: %v", err)
				}
			}
		case <-am.aofRewriteCh:
			// 手动触发重写
			if err := am.RewriteAOF(); err != nil {
				am.logger.Errorf("Error rewriting AOF file: %v", err)
			}
		}
	}
}

// shouldRewriteAOF 判断是否需要重写AOF文件
func (am *AOFManager) shouldRewriteAOF() bool {
	info, err := os.Stat(am.config.AOFFilePath)
	if err != nil {
		return false
	}

	// 如果文件大小超过配置的阈值，触发重写
	// 这里假设阈值为64MB
	return info.Size() > 64*1024*1024
}

// RewriteAOF 重写AOF文件
func (am *AOFManager) RewriteAOF() error {
	// 如果不启用AOF，不需要执行任何操作
	if !am.config.EnableAOF || am.aofFile == nil {
		return nil
	}

	// 确保AOF目录存在
	if err := EnsureDir(am.config.AOFFilePath); err != nil {
		return fmt.Errorf("error ensuring AOF directory: %v", err)
	}

	// 创建临时文件
	tempFile := am.config.AOFFilePath + ".temp"
	file, err := os.OpenFile(tempFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("error creating temp AOF file: %v", err)
	}
	defer file.Close()

	tempBuf := bufio.NewWriter(file)

	// 获取当前数据库的快照
	data, expires := am.cmdExecutor.GetAllData()

	// 创建一个缓冲区，用于在重写期间收集新的写入命令
	rewriteBuffer := make([][]byte, 0, 1024)

	// 设置重写标志，开始收集新命令
	am.aofLock.Lock()
	am.startRewriteBuffering(&rewriteBuffer)
	am.aofLock.Unlock()

	// 为每个键生成对应类型的命令
	now := time.Now()
	count := 0 // 记录写入的命令数
	for key, value := range data {
		// 如果键已过期，跳过
		if expireTime, ok := expires[key]; ok && expireTime.Before(now) {
			continue
		}

		// 使用统一的命令转换逻辑
		var expPtr *time.Time
		if expireTime, ok := expires[key]; ok && expireTime.After(now) {
			expPtr = &expireTime
		}

		// 将Value对象转换为Command对象
		cmds := valueToCommands(key, value, expPtr)
		if len(cmds) == 0 {
			// 如果无法生成命令，使用默认的SET命令
			encodedValue, err := value.Encode()
			if err == nil {
				cmds = append(cmds, Command{
					Name: "SET",
					Args: []string{key, string(encodedValue)},
				})
			}
		}

		// 将Command对象转换为RESP格式并写入
		for _, cmd := range cmds {
			respCmd := commandToRESP(cmd)
			if _, err := tempBuf.Write(respCmd); err != nil {
				// 停止重写缓冲
				am.aofLock.Lock()
				am.stopRewriteBuffering()
				am.aofLock.Unlock()
				return fmt.Errorf("error writing command: %v", err)
			}
			count++
		}
	}

	// 如果没有写入任何命令，创建一个空的注释
	if count == 0 {
		commentCmd := Command{
			Name: "SET",
			Args: []string{"__aof_rewrite_empty__", "1"},
		}
		respCmd := commandToRESP(commentCmd)
		if _, err := tempBuf.Write(respCmd); err != nil {
			am.aofLock.Lock()
			am.stopRewriteBuffering()
			am.aofLock.Unlock()
			return fmt.Errorf("error writing empty marker: %v", err)
		}
	}

	// 刷新缓冲区
	if err := tempBuf.Flush(); err != nil {
		// 停止重写缓冲
		am.aofLock.Lock()
		am.stopRewriteBuffering()
		am.aofLock.Unlock()
		return fmt.Errorf("error flushing temp AOF buffer: %v", err)
	}

	// 确保临时文件数据已写入磁盘
	if err := file.Sync(); err != nil {
		am.aofLock.Lock()
		am.stopRewriteBuffering()
		am.aofLock.Unlock()
		return fmt.Errorf("error syncing temp AOF file: %v", err)
	}

	// 关闭临时文件
	if err := file.Close(); err != nil {
		// 停止重写缓冲
		am.aofLock.Lock()
		am.stopRewriteBuffering()
		am.aofLock.Unlock()
		return fmt.Errorf("error closing temp AOF file: %v", err)
	}

	am.aofLock.Lock()
	defer am.aofLock.Unlock()

	// 刷新当前AOF缓冲区
	if err := am.aofBuf.Flush(); err != nil {
		am.stopRewriteBuffering()
		return fmt.Errorf("error flushing current AOF buffer: %v", err)
	}

	// 同步当前AOF文件，确保所有数据都写入磁盘
	if err := am.aofFile.Sync(); err != nil {
		am.stopRewriteBuffering()
		return fmt.Errorf("error syncing current AOF file: %v", err)
	}

	// 关闭当前AOF文件
	oldFile := am.aofFile
	if err := oldFile.Close(); err != nil {
		am.stopRewriteBuffering()
		return fmt.Errorf("error closing current AOF file: %v", err)
	}

	// 验证临时文件是否存在
	if _, err := os.Stat(tempFile); os.IsNotExist(err) {
		// 如果临时文件不存在，重新打开旧文件
		am.aofFile, _ = os.OpenFile(am.config.AOFFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if am.aofFile != nil {
			am.aofBuf = bufio.NewWriter(am.aofFile)
		}
		am.stopRewriteBuffering()
		return fmt.Errorf("temp file does not exist: %v", err)
	}

	// 原子地将临时文件重命名为AOF文件
	if err := os.Rename(tempFile, am.config.AOFFilePath); err != nil {
		// 如果重命名失败，尝试重新打开旧文件
		am.aofFile, _ = os.OpenFile(am.config.AOFFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if am.aofFile != nil {
			am.aofBuf = bufio.NewWriter(am.aofFile)
		}
		am.stopRewriteBuffering()
		return fmt.Errorf("error renaming temp file: %v", err)
	}

	// 重命名成功后，打开新的AOF文件进行追加
	newFile, err := os.OpenFile(am.config.AOFFilePath, os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		// 如果打开新文件失败，尝试创建新文件
		am.aofFile, _ = os.OpenFile(am.config.AOFFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if am.aofFile != nil {
			am.aofBuf = bufio.NewWriter(am.aofFile)
		}
		am.stopRewriteBuffering()
		return fmt.Errorf("error opening new AOF file: %v", err)
	}

	// 更新文件和缓冲区
	am.aofFile = newFile
	am.aofBuf = bufio.NewWriter(newFile)

	// 将重写期间收集的命令写入新的AOF文件
	for _, cmd := range rewriteBuffer {
		if _, err := am.aofBuf.Write(cmd); err != nil {
			am.stopRewriteBuffering()
			return fmt.Errorf("error writing buffered command: %v", err)
		}
	}

	// 刷新缓冲区
	if err := am.aofBuf.Flush(); err != nil {
		am.stopRewriteBuffering()
		return fmt.Errorf("error flushing AOF buffer after rewrite: %v", err)
	}

	// 确保数据已同步到磁盘
	if err := am.aofFile.Sync(); err != nil {
		am.stopRewriteBuffering()
		return fmt.Errorf("error syncing AOF file after rewrite: %v", err)
	}

	// 停止重写缓冲
	am.stopRewriteBuffering()

	am.logger.Infof("AOF rewrite completed successfully")
	return nil
}

// LoadAOF 从AOF文件加载命令
func (am *AOFManager) LoadAOF() error {
	// 如果不启用AOF，不需要执行任何操作
	if !am.config.EnableAOF {
		return nil
	}

	// 防止重复加载
	if am.loaded {
		return nil
	}

	originalEnableAOF := am.config.EnableAOF
	am.config.EnableAOF = false
	defer func() {
		am.config.EnableAOF = originalEnableAOF
		am.loaded = true // 标记为已加载
	}()

	// 检查文件是否存在
	_, err := os.Stat(am.config.AOFFilePath)
	if os.IsNotExist(err) {
		// 文件不存在，记录错误信息
		am.logger.Infof("No AOF file found at %s", am.config.AOFFilePath)
		return nil
	} else if err != nil {
		return fmt.Errorf("error checking AOF file: %v", err)
	}

	am.aofLock.Lock()
	defer am.aofLock.Unlock()

	// 打开文件
	file, err := os.Open(am.config.AOFFilePath)
	if err != nil {
		return fmt.Errorf("error opening AOF file: %v", err)
	}
	defer file.Close()

	// 创建RESP解析器
	reader := bufio.NewReader(file)

	// 创建一个命令集合，保存已执行的LPUSH/RPUSH命令的键名
	processedLists := make(map[string]bool)

	// 解析和执行命令
	var lastErr error
	for {
		cmd, err := parseRESPCommand(reader)
		if err != nil {
			if err == io.EOF {
				break
			}
			lastErr = fmt.Errorf("error parsing AOF command: %v", err)
			am.logger.Errorf("error parsing AOF command: %v", err)
			continue
		}

		// 执行命令（跳过读命令）
		cmdName := strings.ToUpper(cmd.Name)
		if !isReadOnlyCommand(cmdName) {
			// 特殊处理LPUSH/RPUSH命令，防止重复
			if (cmdName == "LPUSH" || cmdName == "RPUSH") && len(cmd.Args) >= 1 {
				key := cmd.Args[0]

				// 如果已经处理过这个列表，跳过命令
				if processedLists[key] {
					continue
				}

				// 标记为已处理
				processedLists[key] = true
			}

			// 执行命令
			if err := am.cmdExecutor.ExecuteCommand(cmd); err != nil {
				lastErr = fmt.Errorf("error executing command from AOF: %v", err)
				am.logger.Errorf("error executing command from AOF: %v", err)
				continue
			}
		}
	}

	if lastErr != nil {
		return lastErr
	}

	am.logger.Infof("Successfully loaded commands from AOF file: %s", am.config.AOFFilePath)
	return nil
}

// Shutdown 关闭AOF管理器
func (am *AOFManager) Shutdown() error {
	// 如果不启用AOF，不需要执行任何操作
	if !am.config.EnableAOF || am.aofFile == nil {
		return nil
	}

	// 发送关闭信号
	close(am.shutdownCh)

	// 同步AOF文件
	am.aofLock.Lock()
	defer am.aofLock.Unlock()

	// 刷新缓冲区
	if err := am.aofBuf.Flush(); err != nil {
		return fmt.Errorf("error flushing AOF buffer on shutdown: %v", err)
	}

	// 关闭文件
	if err := am.aofFile.Close(); err != nil {
		return fmt.Errorf("error closing AOF file on shutdown: %v", err)
	}

	return nil
}

// TriggerRewrite 触发AOF重写
func (am *AOFManager) TriggerRewrite() {
	select {
	case am.aofRewriteCh <- struct{}{}:
		// 通知已发送
	default:
		// 已有重写任务在进行中，跳过
	}
}

// isReadOnlyCommand 判断命令是否为只读命令
func isReadOnlyCommand(cmdName string) bool {
	readOnlyCommands := map[string]bool{
		"GET":           true,
		"EXISTS":        true,
		"TYPE":          true,
		"TTL":           true,
		"KEYS":          true,
		"LLEN":          true,
		"LRANGE":        true,
		"LINDEX":        true,
		"HGET":          true,
		"HEXISTS":       true,
		"HLEN":          true,
		"HGETALL":       true,
		"HKEYS":         true,
		"HVALS":         true,
		"SISMEMBER":     true,
		"SMEMBERS":      true,
		"SCARD":         true,
		"SINTER":        true,
		"SUNION":        true,
		"SDIFF":         true,
		"ZSCORE":        true,
		"ZCARD":         true,
		"ZRANGE":        true,
		"ZREVRANGE":     true,
		"ZRANGEBYSCORE": true,
		"ZRANK":         true,
		"ZREVRANK":      true,
		"PING":          true,
	}

	return readOnlyCommands[cmdName]
}

// parseRESPCommand 解析RESP格式的命令
func parseRESPCommand(reader *bufio.Reader) (Command, error) {
	// 读取第一行，确认是数组
	line, err := reader.ReadString('\n')
	if err != nil {
		return Command{}, err
	}

	// 检查是否是数组格式
	if len(line) < 2 || line[0] != '*' {
		return Command{}, errors.New("invalid RESP format")
	}

	// 解析参数数量
	count, err := strconv.Atoi(strings.TrimSpace(line[1:]))
	if err != nil {
		return Command{}, fmt.Errorf("invalid array length: %v", err)
	}

	if count < 1 {
		return Command{}, errors.New("empty command")
	}

	args := make([]string, count)
	for i := 0; i < count; i++ {
		// 读取批量字符串标记
		line, err = reader.ReadString('\n')
		if err != nil {
			return Command{}, err
		}

		if len(line) < 2 || line[0] != '$' {
			return Command{}, errors.New("invalid bulk string format")
		}

		// 解析字符串长度
		length, err := strconv.Atoi(strings.TrimSpace(line[1:]))
		if err != nil {
			return Command{}, fmt.Errorf("invalid string length: %v", err)
		}

		// 读取字符串内容
		buf := make([]byte, length+2) // +2 for CRLF
		_, err = io.ReadFull(reader, buf)
		if err != nil {
			return Command{}, err
		}

		args[i] = string(buf[:length])
	}

	// 创建命令对象
	return Command{
		Name: args[0],
		Args: args[1:],
	}, nil
}

// valueToCommands 将Value对象转换为Command对象
func valueToCommands(key string, value cache2.Value, expireTime *time.Time) []Command {
	if value == nil {
		return []Command{}
	}

	// 首先根据值类型生成对应的命令
	cmds := generateCommandsForValue(key, value)

	// 如果没有生成任何命令（可能是因为类型无法识别），尝试使用基本方法获取字符串表示
	if len(cmds) == 0 {
		// 尝试获取字符串表示
		var strValue string

		// 尝试使用String()方法
		if strGetter, ok := value.(interface{ String() string }); ok {
			strValue = strGetter.String()
			cmds = append(cmds, Command{
				Name: "SET",
				Args: []string{key, strValue},
			})
		} else {
			// 尝试编码
			encodedValue, err := value.Encode()
			if err == nil {
				cmds = append(cmds, Command{
					Name: "SET",
					Args: []string{key, string(encodedValue)},
				})
			}
		}
	}

	// 如果有过期时间，添加EXPIRE命令
	if expireTime != nil {
		now := time.Now()
		ttl := int64(expireTime.Sub(now).Seconds())
		if ttl > 0 {
			cmds = append(cmds, Command{
				Name: "EXPIRE",
				Args: []string{key, strconv.FormatInt(ttl, 10)},
			})
		}
	}

	return cmds
}

// commandToRESP 将Command对象转换为RESP格式
func commandToRESP(cmd Command) []byte {
	return encodeCommandToRESP(cmd.Name, cmd.Args)
}

// encodeCommandToRESP 将命令编码为RESP格式
func encodeCommandToRESP(name string, args []string) []byte {
	var buffer strings.Builder

	// 数组标记
	buffer.WriteString(fmt.Sprintf("*%d\r\n", len(args)+1))

	// 命令名
	buffer.WriteString(fmt.Sprintf("$%d\r\n%s\r\n", len(name), name))

	// 参数
	for _, arg := range args {
		buffer.WriteString(fmt.Sprintf("$%d\r\n%s\r\n", len(arg), arg))
	}

	return []byte(buffer.String())
}

// startRewriteBuffering 开始重写缓冲
func (am *AOFManager) startRewriteBuffering(buffer *[][]byte) {
	am.rewriteActive = true
	am.rewriteBuffer = buffer
}

// stopRewriteBuffering 停止重写缓冲
func (am *AOFManager) stopRewriteBuffering() {
	am.rewriteActive = false
	am.rewriteBuffer = nil
}

// generateCommandsForValue 根据值的类型生成对应的命令
func generateCommandsForValue(key string, value cache2.Value) []Command {
	var commands []Command

	switch value.Type() {
	case cache2.TypeString:
		// 对于字符串类型，使用SET命令
		// 尝试多种方式获取字符串值
		var strValue string
		var success bool

		// 方法1: 尝试使用StringValue接口
		if strVal, ok := value.(cache2.StringValue); ok {
			strValue = strVal.String()
			success = true
		}

		// 方法2: 尝试String()方法
		if !success {
			if strGetter, ok := value.(interface{ String() string }); ok {
				strValue = strGetter.String()
				success = true
			}
		}

		// 方法3: 尝试编码
		if !success {
			if encoded, err := value.Encode(); err == nil {
				strValue = string(encoded)
				success = true
			}
		}

		if success {
			commands = append(commands, Command{
				Name: "SET",
				Args: []string{key, strValue},
			})
		}

	case cache2.TypeList:
		// 对于列表类型，使用RPUSH命令
		if listVal, ok := value.(cache2.ListValue); ok {
			items := listVal.Range(0, listVal.Len()-1)
			if len(items) > 0 {
				args := make([]string, len(items)+1)
				args[0] = key
				copy(args[1:], items)
				commands = append(commands, Command{
					Name: "RPUSH",
					Args: args,
				})
			}
		}

	case cache2.TypeHash:
		// 对于哈希表类型，使用HSET命令（将HMSET改为多个HSET命令）
		if hashVal, ok := value.(cache2.HashValue); ok {
			fields := hashVal.GetAll()
			if len(fields) > 0 {
				for field, val := range fields {
					commands = append(commands, Command{
						Name: "HSET",
						Args: []string{key, field, val},
					})
				}
			}
		}

	case cache2.TypeSet:
		// 对于集合类型，使用SADD命令
		if setVal, ok := value.(cache2.SetValue); ok {
			members := setVal.Members()
			if len(members) > 0 {
				args := make([]string, len(members)+1)
				args[0] = key
				copy(args[1:], members)
				commands = append(commands, Command{
					Name: "SADD",
					Args: args,
				})
			}
		}

	case cache2.TypeZSet:
		// 对于有序集合类型，使用ZADD命令
		if zsetVal, ok := value.(cache2.ZSetValue); ok {
			entries := zsetVal.RangeWithScores(0, -1)
			if len(entries) > 0 {
				for member, score := range entries {
					// 使用%g格式确保浮点数格式正确
					commands = append(commands, Command{
						Name: "ZADD",
						Args: []string{key, fmt.Sprintf("%.17g", score), member},
					})
				}
			}
		}
	}

	return commands
}

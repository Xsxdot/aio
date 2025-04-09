package engine

import (
	"fmt"
	cache2 "github.com/xsxdot/aio/internal/cache"
	persistence2 "github.com/xsxdot/aio/internal/cache/persistence"
	"github.com/xsxdot/aio/pkg/common"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// PersistenceManager 管理持久化
type PersistenceManager struct {
	db         *MemoryDatabase
	rdbManager *persistence2.RDBManager
	aofManager *persistence2.AOFManager
	logger     *common.Logger // 日志记录器
}

// NewPersistenceManager 创建一个新的持久化管理器
func NewPersistenceManager(db *MemoryDatabase, wg *sync.WaitGroup) (*PersistenceManager, error) {
	pm := &PersistenceManager{
		db:     db,
		logger: common.GetLogger(), // 初始化logger
	}

	// 创建RDB适配器
	rdbAdapter := &rdbAccessorAdapter{db: db}
	pm.rdbManager = persistence2.NewRDBManager(db.config, rdbAdapter, wg)

	// 创建AOF适配器和管理器
	aofAdapter := &aofExecutorAdapter{db: db}
	aofManager, err := persistence2.NewAOFManager(db.config, aofAdapter, wg)
	if err != nil {
		return nil, err
	}
	pm.aofManager = aofManager

	return pm, nil
}

// Start 启动持久化管理器
func (pm *PersistenceManager) Start() error {
	// 根据端口号设置不同的RDB文件路径，避免主从节点共用同一个RDB文件
	if pm.db.config.EnableRDB {
		// 生成基于端口的RDB文件路径
		originalPath := pm.db.config.RDBFilePath
		dir := strings.TrimSuffix(originalPath, "dump.rdb")
		newPath := fmt.Sprintf("%sdump_%d.rdb", dir, pm.db.config.Port)
		pm.db.config.RDBFilePath = newPath
		pm.logger.Infof("使用端口特定的RDB文件路径: %s", newPath)
	}

	// 如果启用了RDB，则从RDB文件加载数据并启动定期保存
	if pm.db.config.EnableRDB {
		if err := pm.rdbManager.LoadFromRDB(); err != nil {
			return err
		}
		pm.rdbManager.StartPeriodicSave()
	}

	// 如果启用了AOF，则启动AOF管理器
	if pm.db.config.EnableAOF {
		// 只有在没有RDB或RDB文件不存在的情况下才加载AOF
		// 因为在RDB包含了截至某个时间点的所有数据
		if !pm.db.config.EnableRDB || !fileExists(pm.db.config.RDBFilePath) {
			if err := pm.aofManager.LoadAOF(); err != nil {
				return err
			}
		}
		if err := pm.aofManager.Start(); err != nil {
			return err
		}
	}

	return nil
}

// Close 关闭持久化管理器
func (pm *PersistenceManager) Close() error {
	// 关闭AOF管理器
	if pm.db.config.EnableAOF {
		if err := pm.aofManager.Shutdown(); err != nil {
			return err
		}
	}

	// 关闭RDB管理器
	if pm.db.config.EnableRDB {
		pm.rdbManager.Shutdown()
	}

	return nil
}

// WriteAOF 将命令写入AOF
func (pm *PersistenceManager) WriteAOF(name string, args []string) error {
	if !pm.db.config.EnableAOF {
		return nil
	}

	// 创建命令对象
	cmd := persistence2.Command{
		Name: name,
		Args: args,
	}

	// 尝试写入命令，重试几次
	var err error
	for i := 0; i < 3; i++ {
		err = pm.aofManager.WriteCommand(cmd)
		if err == nil {
			return nil
		}
		// 如果失败，短暂等待后重试
		time.Sleep(10 * time.Millisecond)
	}

	return err
}

// SaveRDB 保存RDB文件
func (pm *PersistenceManager) SaveRDB() error {
	if !pm.db.config.EnableRDB {
		return nil
	}

	return pm.rdbManager.SaveToRDB()
}

// TriggerAOFRewrite 触发AOF重写
func (pm *PersistenceManager) TriggerAOFRewrite() {
	if pm.db.config.EnableAOF {
		pm.aofManager.TriggerRewrite()
	}
}

// LoadAOF 从AOF文件加载命令
func (pm *PersistenceManager) LoadAOF() error {
	if !pm.db.config.EnableAOF {
		return nil
	}

	// 尝试加载AOF文件，重试几次
	var err error
	for i := 0; i < 3; i++ {
		err = pm.aofManager.LoadAOF()
		if err == nil {
			return nil
		}
		// 如果失败，短暂等待后重试
		time.Sleep(100 * time.Millisecond)
	}

	return fmt.Errorf("加载AOF文件失败: %v", err)
}

// RDB访问适配器
type rdbAccessorAdapter struct {
	db *MemoryDatabase
}

// GetAllData 获取数据库中的所有数据
func (a *rdbAccessorAdapter) GetAllData() (map[string]cache2.Value, map[string]time.Time) {
	a.db.mutex.RLock()
	defer a.db.mutex.RUnlock()

	// 创建数据副本
	data := make(map[string]cache2.Value, len(a.db.data))
	expires := a.db.expiryPolicy.GetExpiryMap()

	for k, v := range a.db.data {
		data[k] = v.DeepCopy()
	}

	return data, expires
}

// LoadData 加载数据到数据库
func (a *rdbAccessorAdapter) LoadData(data map[string]cache2.Value, expires map[string]time.Time) error {
	a.db.mutex.Lock()
	defer a.db.mutex.Unlock()

	// 清空当前数据
	a.db.data = make(map[string]cache2.Value, len(data))

	// 加载新数据
	for k, v := range data {
		a.db.data[k] = v
	}

	// 加载过期时间
	a.db.expiryPolicy.LoadExpiryMap(expires)

	return nil
}

// AOF执行适配器
type aofExecutorAdapter struct {
	db *MemoryDatabase
}

// ExecuteCommand 执行命令
func (a *aofExecutorAdapter) ExecuteCommand(cmd persistence2.Command) error {
	// 检查命令参数
	if cmd.Name == "" {
		return &CommandError{Message: "命令名称不能为空"}
	}

	// 参数验证
	if err := validateCommand(cmd.Name, cmd.Args); err != nil {
		return err
	}

	// 构造内部命令对象
	internalCmd := &simpleCommand{
		name:     cmd.Name,
		args:     cmd.Args,
		clientID: "aof-executor",
		replyCh:  make(chan cache2.Reply, 1),
	}

	// 直接执行命令
	reply := a.db.ProcessCommand(internalCmd)

	// 检查响应是否为错误
	if reply == nil {
		return &CommandError{Message: "命令执行返回nil响应"}
	}

	if reply.Type() == cache2.ReplyError {
		return &CommandError{Message: fmt.Sprintf("命令执行错误: %s", reply.String())}
	}

	// 命令执行成功
	return nil
}

// validateCommand 验证命令参数
func validateCommand(name string, args []string) error {
	name = strings.ToUpper(name)

	// 验证参数数量和格式
	switch name {
	case "SET":
		if len(args) < 2 {
			return &CommandError{Message: "ERR wrong number of arguments for 'set' command"}
		}
	case "HSET":
		if len(args) < 3 {
			return &CommandError{Message: "ERR wrong number of arguments for 'hset' command"}
		}
	case "ZADD":
		if len(args) < 3 || len(args)%2 != 1 {
			return &CommandError{Message: "ERR wrong number of arguments for 'zadd' command"}
		}
		// 验证分数是有效的浮点数
		for i := 1; i < len(args); i += 2 {
			if _, err := strconv.ParseFloat(args[i], 64); err != nil {
				return &CommandError{Message: "ERR value is not a valid float"}
			}
		}
	}

	return nil
}

// GetAllData 获取数据库中的所有数据
func (a *aofExecutorAdapter) GetAllData() (map[string]cache2.Value, map[string]time.Time) {
	return a.db.getAllData()
}

// CommandError 命令执行错误
type CommandError struct {
	Message string
}

// Error 实现error接口
func (e *CommandError) Error() string {
	return e.Message
}

// getAllData 获取数据库中的所有数据（辅助方法）
func (db *MemoryDatabase) getAllData() (map[string]cache2.Value, map[string]time.Time) {
	db.mutex.RLock()
	defer db.mutex.RUnlock()

	// 创建数据副本
	data := make(map[string]cache2.Value, len(db.data))
	expires := db.expiryPolicy.GetExpiryMap()

	for k, v := range db.data {
		data[k] = v.DeepCopy()
	}

	return data, expires
}

// 辅助函数，检查文件是否存在
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

// SyncAOF 立即同步AOF文件到磁盘
func (pm *PersistenceManager) SyncAOF() error {
	if !pm.db.config.EnableAOF || pm.aofManager == nil {
		return nil
	}

	return pm.aofManager.Sync()
}

// WriteCommandAndSync 将命令写入AOF文件并立即同步
func (pm *PersistenceManager) WriteCommandAndSync(name string, args []string) error {
	if !pm.db.config.EnableAOF {
		return nil
	}

	// 创建命令对象
	cmd := persistence2.Command{
		Name: name,
		Args: args,
	}

	// 写入命令
	if err := pm.aofManager.WriteCommand(cmd); err != nil {
		return err
	}

	// 立即同步到磁盘
	return pm.aofManager.Sync()
}

package protocol

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"github.com/xsxdot/aio/internal/cache"
	"io"
	"strconv"
	"strings"
)

// 错误定义
var (
	ErrEmptyCommand = errors.New("空命令")
	ErrParseCommand = errors.New("解析命令失败")
)

// RESPCommandParser 负责将RESP协议字节数据解析为命令结构体
type RESPCommandParser struct{}

// NewRESPCommandParser 创建新的RESP命令解析器
func NewRESPCommandParser() *RESPCommandParser {
	return &RESPCommandParser{}
}

// Parse 解析RESP字节数据为命令
func (p *RESPCommandParser) Parse(data []byte, clientID string) (cache.Command, error) {
	reader := bufio.NewReader(bytes.NewReader(data))

	// 读取类型标记
	typeChar, err := reader.ReadByte()
	if err != nil {
		return nil, err
	}

	var cmd []string
	switch typeChar {
	case Array:
		// 读取数组长度行
		lenLine, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}

		// 移除CRLF并解析长度
		cleanLine := lenLine
		if len(cleanLine) >= 2 && cleanLine[len(cleanLine)-2] == '\r' && cleanLine[len(cleanLine)-1] == '\n' {
			cleanLine = cleanLine[:len(cleanLine)-2]
		}

		length, err := strconv.Atoi(cleanLine)
		if err != nil {
			return nil, fmt.Errorf("解析数组长度错误: %w, 数据: %q", err, cleanLine)
		}

		// 空数组
		if length == -1 || length == 0 {
			return nil, ErrEmptyCommand
		}

		// 读取数组元素
		args := make([]string, length)
		for i := 0; i < length; i++ {
			// 读取元素类型
			elemType, err := reader.ReadByte()
			if err != nil {
				return nil, err
			}

			// 目前只支持批量字符串类型的命令参数
			if elemType != BulkString {
				return nil, ErrInvalidRESPType
			}

			// 读取字符串长度
			lenLine, err := reader.ReadString('\n')
			if err != nil {
				return nil, err
			}

			// 移除CRLF并解析长度
			cleanLenLine := lenLine
			if len(cleanLenLine) >= 2 && cleanLenLine[len(cleanLenLine)-2] == '\r' && cleanLenLine[len(cleanLenLine)-1] == '\n' {
				cleanLenLine = cleanLenLine[:len(cleanLenLine)-2]
			}

			strLen, err := strconv.Atoi(cleanLenLine)
			if err != nil {
				return nil, fmt.Errorf("解析字符串长度错误: %w, 数据: %q", err, cleanLenLine)
			}

			// 处理nil值
			if strLen == -1 {
				args[i] = ""
				continue
			}

			// 读取字符串内容和CRLF
			data := make([]byte, strLen+2) // +2 为CRLF
			if _, err := io.ReadFull(reader, data); err != nil {
				return nil, err
			}

			// 移除CRLF
			if len(data) >= 2 && data[len(data)-2] == '\r' && data[len(data)-1] == '\n' {
				args[i] = string(data[:len(data)-2])
			} else {
				return nil, ErrInvalidRESP
			}
		}

		cmd = args
	default:
		// 回退一个字节以便重新读取
		reader = bufio.NewReader(bytes.NewReader(data))
		// 尝试作为内联命令解析
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		// 移除CRLF
		if len(line) >= 2 && line[len(line)-2] == '\r' && line[len(line)-1] == '\n' {
			line = line[:len(line)-2]
		}
		// 分割命令
		cmd = strings.Fields(line)
		if len(cmd) == 0 {
			return nil, ErrEmptyCommand
		}
	}

	// 创建命令结构体
	return NewRESPCommand(cmd[0], cmd[1:], clientID), nil
}

// RESPCommand 表示一个RESP协议命令
type RESPCommand struct {
	name     string
	args     []string
	clientID string
	replyCh  chan cache.Reply
	dbIndex  int // 新增的数据库索引字段
}

// NewRESPCommand 创建新的RESP命令
func NewRESPCommand(name string, args []string, clientID string) *RESPCommand {
	return &RESPCommand{
		name:     name,
		args:     args,
		clientID: clientID,
		replyCh:  make(chan cache.Reply, 1),
		dbIndex:  0, // 默认数据库索引为0
	}
}

// Name 返回命令名称
func (c *RESPCommand) Name() string {
	return c.name
}

// Args 返回命令参数
func (c *RESPCommand) Args() []string {
	return c.args
}

// ClientID 返回客户端ID
func (c *RESPCommand) ClientID() string {
	return c.clientID
}

// SetReply 设置命令的回复
func (c *RESPCommand) SetReply(reply cache.Reply) {
	c.replyCh <- reply
}

// GetReply 获取命令的回复通道
func (c *RESPCommand) GetReply() chan cache.Reply {
	return c.replyCh
}

// IsWriteCommand 判断命令是否会修改数据
func (c *RESPCommand) IsWriteCommand() bool {
	// 获取命令名称的大写形式
	cmdName := strings.ToUpper(c.name)

	// 定义写入命令列表
	writeCommands := map[string]bool{
		"SET":              true,
		"SETNX":            true,
		"SETEX":            true,
		"PSETEX":           true,
		"MSET":             true,
		"MSETNX":           true,
		"APPEND":           true,
		"DEL":              true,
		"UNLINK":           true,
		"INCR":             true,
		"INCRBY":           true,
		"INCRBYFLOAT":      true,
		"DECR":             true,
		"DECRBY":           true,
		"RPUSH":            true,
		"LPUSH":            true,
		"RPUSHX":           true,
		"LPUSHX":           true,
		"LINSERT":          true,
		"LSET":             true,
		"LREM":             true,
		"LTRIM":            true,
		"RPOP":             true,
		"LPOP":             true,
		"RPOPLPUSH":        true,
		"LMOVE":            true,
		"BLMOVE":           true,
		"SADD":             true,
		"SREM":             true,
		"SMOVE":            true,
		"SPOP":             true,
		"SINTERSTORE":      true,
		"SUNIONSTORE":      true,
		"SDIFFSTORE":       true,
		"ZADD":             true,
		"ZINCRBY":          true,
		"ZREM":             true,
		"ZREMRANGEBYRANK":  true,
		"ZREMRANGEBYSCORE": true,
		"ZREMRANGEBYLEX":   true,
		"ZINTERSTORE":      true,
		"ZUNIONSTORE":      true,
		"HSET":             true,
		"HSETNX":           true,
		"HMSET":            true,
		"HINCRBY":          true,
		"HINCRBYFLOAT":     true,
		"HDEL":             true,
		"EXPIRE":           true,
		"EXPIREAT":         true,
		"PEXPIRE":          true,
		"PEXPIREAT":        true,
		"FLUSHDB":          true,
		"FLUSHALL":         true,
		"RENAME":           true,
		"RENAMENX":         true,
	}

	return writeCommands[cmdName]
}

// CommandToRESP 将命令转换为RESP协议字节数组
func CommandToRESP(cmd cache.Command) []byte {
	var buffer bytes.Buffer

	// 写入数组类型和长度
	// 命令名称 + 参数数量
	arrayLen := 1 + len(cmd.Args())
	buffer.WriteString(fmt.Sprintf("*%d\r\n", arrayLen))

	// 写入命令名称
	buffer.WriteString(fmt.Sprintf("$%d\r\n%s\r\n", len(cmd.Name()), cmd.Name()))

	// 写入参数
	for _, arg := range cmd.Args() {
		buffer.WriteString(fmt.Sprintf("$%d\r\n%s\r\n", len(arg), arg))
	}

	return buffer.Bytes()
}

// GetDbIndex 获取命令关联的数据库索引
func (c *RESPCommand) GetDbIndex() int {
	return c.dbIndex
}

// SetDbIndex 设置命令关联的数据库索引
func (c *RESPCommand) SetDbIndex(index int) {
	c.dbIndex = index
}

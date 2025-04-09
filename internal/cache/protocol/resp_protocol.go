package protocol

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"
)

const (
	// RESP 类型标记
	SimpleString byte = '+'
	Error        byte = '-'
	Integer      byte = ':'
	BulkString   byte = '$'
	Array        byte = '*'

	// 行分隔符
	CRLF = "\r\n"
)

// 错误定义
var (
	ErrInvalidRESP     = errors.New("无效的RESP数据")
	ErrIncompleteRESP  = errors.New("不完整的RESP数据")
	ErrInvalidRESPType = errors.New("无效的RESP类型")
)

// RESPProtocol 实现网络协议接口
type RESPProtocol struct{}

// NewRESPProtocol 创建RESP协议
func NewRESPProtocol() *RESPProtocol {
	return &RESPProtocol{}
}

// Read 从读取器读取一个完整的RESP命令
func (p *RESPProtocol) Read(reader io.Reader) ([]byte, error) {
	br, ok := reader.(*bufio.Reader)
	if !ok {
		br = bufio.NewReader(reader)
	}

	var buffer bytes.Buffer

	// 尝试读取第一个字节确定类型
	typeChar, err := br.ReadByte()
	if err != nil {
		return nil, err
	}
	buffer.WriteByte(typeChar)

	switch typeChar {
	case SimpleString, Error, Integer:
		// 简单字符串、错误、整数类型，读取一行直到CRLF
		line, err := br.ReadString('\n')
		if err != nil {
			return nil, err
		}
		buffer.WriteString(line)
		return buffer.Bytes(), nil

	case BulkString:
		// 读取长度行
		lenLine, err := br.ReadString('\n')
		if err != nil {
			return nil, err
		}
		buffer.WriteString(lenLine)

		// 从长度行中提取数字部分
		lenStr := lenLine
		if len(lenStr) >= 2 && lenStr[len(lenStr)-2] == '\r' && lenStr[len(lenStr)-1] == '\n' {
			lenStr = lenStr[:len(lenStr)-2]
		}

		length, err := strconv.Atoi(lenStr)
		if err != nil {
			return nil, fmt.Errorf("解析字符串长度错误: %w, 数据: %q", err, lenStr)
		}

		// 对于nil值
		if length == -1 {
			return buffer.Bytes(), nil
		}

		// 读取指定长度的数据
		dataWithCRLF := make([]byte, length+2) // +2 为CRLF
		if _, err := io.ReadFull(br, dataWithCRLF); err != nil {
			return nil, err
		}
		buffer.Write(dataWithCRLF)
		return buffer.Bytes(), nil

	case Array:
		// 读取数组长度行
		lenLine, err := br.ReadString('\n')
		if err != nil {
			return nil, err
		}
		buffer.WriteString(lenLine)

		// 从长度行中提取数字部分
		lenStr := lenLine
		if len(lenStr) >= 2 && lenStr[len(lenStr)-2] == '\r' && lenStr[len(lenStr)-1] == '\n' {
			lenStr = lenStr[:len(lenStr)-2]
		}

		length, err := strconv.Atoi(lenStr)
		if err != nil {
			return nil, fmt.Errorf("解析数组长度错误: %w, 数据: %q", err, lenStr)
		}

		// 对于nil数组
		if length == -1 {
			return buffer.Bytes(), nil
		}

		// 读取每个数组元素
		for i := 0; i < length; i++ {
			// 读取元素类型
			elemType, err := br.ReadByte()
			if err != nil {
				return nil, err
			}
			buffer.WriteByte(elemType)

			switch elemType {
			case SimpleString, Error, Integer:
				// 读取一行直到CRLF
				line, err := br.ReadString('\n')
				if err != nil {
					return nil, err
				}
				buffer.WriteString(line)

			case BulkString:
				// 读取长度行
				lenLine, err := br.ReadString('\n')
				if err != nil {
					return nil, err
				}
				buffer.WriteString(lenLine)

				// 提取数字部分
				bulkLenStr := lenLine
				if len(bulkLenStr) >= 2 && bulkLenStr[len(bulkLenStr)-2] == '\r' && bulkLenStr[len(bulkLenStr)-1] == '\n' {
					bulkLenStr = bulkLenStr[:len(bulkLenStr)-2]
				}

				bulkLen, err := strconv.Atoi(bulkLenStr)
				if err != nil {
					return nil, fmt.Errorf("解析批量字符串长度错误: %w, 数据: %q", err, bulkLenStr)
				}

				// 对于nil值
				if bulkLen == -1 {
					continue
				}

				// 读取指定长度的数据和CRLF
				dataWithCRLF := make([]byte, bulkLen+2) // +2 为CRLF
				if _, err := io.ReadFull(br, dataWithCRLF); err != nil {
					return nil, err
				}
				buffer.Write(dataWithCRLF)

			case Array:
				// 对于嵌套数组，递归读取
				elemData, err := p.Read(br)
				if err != nil {
					return nil, err
				}
				// 写入除了第一个类型字节外的数据（因为已经写入过了）
				buffer.Write(elemData[1:])

			default:
				return nil, ErrInvalidRESPType
			}
		}

		return buffer.Bytes(), nil

	default:
		// 不是标准RESP起始字符，尝试作为内联命令读取
		_ = br.UnreadByte() // 回退以便重新读取
		buffer.Reset()
		line, err := br.ReadString('\n')
		if err != nil {
			return nil, err
		}
		buffer.WriteString(line)
		return buffer.Bytes(), nil
	}
}

// Write 将数据写入写入器
func (p *RESPProtocol) Write(writer io.Writer, data []byte) error {
	_, err := writer.Write(data)
	return err
}

// Name 返回协议名称
func (p *RESPProtocol) Name() string {
	return "RESP"
}

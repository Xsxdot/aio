package protocol

import (
	"bufio"
	"fmt"
	"io"
)

// DelimiterProtocol 分隔符协议
type DelimiterProtocol struct {
	delimiter []byte
	reader    *bufio.Reader
}

// NewDelimiterProtocol 创建分隔符协议
func NewDelimiterProtocol(delimiter []byte) *DelimiterProtocol {
	if len(delimiter) == 0 {
		panic("delimiter cannot be empty")
	}

	return &DelimiterProtocol{
		delimiter: delimiter,
	}
}

// Read 读取消息
func (p *DelimiterProtocol) Read(reader io.Reader) ([]byte, error) {
	// 如果reader是bufio.Reader，直接使用
	if p.reader == nil {
		if br, ok := reader.(*bufio.Reader); ok {
			p.reader = br
		} else {
			p.reader = bufio.NewReader(reader)
		}
	}

	// 读取直到分隔符
	data, err := p.reader.ReadBytes(p.delimiter[0])
	if err != nil {
		return nil, fmt.Errorf("read error: %w", err)
	}

	// 移除分隔符
	return data[:len(data)-1], nil
}

// Write 写入消息
func (p *DelimiterProtocol) Write(writer io.Writer, data []byte) error {
	// 写入消息体
	if _, err := writer.Write(data); err != nil {
		return fmt.Errorf("write payload error: %w", err)
	}

	// 写入分隔符
	if _, err := writer.Write(p.delimiter); err != nil {
		return fmt.Errorf("write delimiter error: %w", err)
	}

	return nil
}

// Name 获取协议名称
func (p *DelimiterProtocol) Name() string {
	return "delimiter"
}

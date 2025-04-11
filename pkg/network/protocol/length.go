package protocol

import (
	"encoding/binary"
	"fmt"
	"io"
)

// LengthFieldProtocol 长度字段协议
type LengthFieldProtocol struct {
	headerLength int
	maxLength    int
}

// NewLengthFieldProtocol 创建长度字段协议
func NewLengthFieldProtocol(headerLength, maxLength int) *LengthFieldProtocol {
	if headerLength <= 0 || headerLength > 8 {
		panic("header length must be between 1 and 8")
	}
	if maxLength <= 0 {
		panic("max length must be positive")
	}

	return &LengthFieldProtocol{
		headerLength: headerLength,
		maxLength:    maxLength,
	}
}

// Read 读取消息
func (p *LengthFieldProtocol) Read(reader io.Reader) ([]byte, error) {
	// 读取消息头
	header := make([]byte, p.headerLength)
	if _, err := io.ReadFull(reader, header); err != nil {
		return nil, fmt.Errorf("read header error: %w", err)
	}

	// 解析消息长度
	var length uint64
	switch p.headerLength {
	case 1:
		length = uint64(header[0])
	case 2:
		length = uint64(binary.BigEndian.Uint16(header))
	case 4:
		length = uint64(binary.BigEndian.Uint32(header))
	case 8:
		length = binary.BigEndian.Uint64(header)
	default:
		return nil, fmt.Errorf("unsupported header length: %d", p.headerLength)
	}

	if length > uint64(p.maxLength) {
		return nil, fmt.Errorf("message too large: %d > %d", length, p.maxLength)
	}

	// 读取消息体
	payload := make([]byte, length)
	if _, err := io.ReadFull(reader, payload); err != nil {
		return nil, fmt.Errorf("read payload error: %w", err)
	}

	return payload, nil
}

// Write 写入消息
func (p *LengthFieldProtocol) Write(writer io.Writer, data []byte) error {
	length := uint64(len(data))
	if length > uint64(p.maxLength) {
		return fmt.Errorf("message too large: %d > %d", length, p.maxLength)
	}

	// 写入消息头
	header := make([]byte, p.headerLength)
	switch p.headerLength {
	case 1:
		header[0] = byte(length)
	case 2:
		binary.BigEndian.PutUint16(header, uint16(length))
	case 4:
		binary.BigEndian.PutUint32(header, uint32(length))
	case 8:
		binary.BigEndian.PutUint64(header, length)
	default:
		return fmt.Errorf("unsupported header length: %d", p.headerLength)
	}

	// 一次性写入头部和数据，避免多次系统调用
	combined := make([]byte, p.headerLength+int(length))
	copy(combined, header)
	copy(combined[p.headerLength:], data)

	if _, err := writer.Write(combined); err != nil {
		return fmt.Errorf("write message error: %w", err)
	}

	return nil
}

// Name 获取协议名称
func (p *LengthFieldProtocol) Name() string {
	return "length-field"
}

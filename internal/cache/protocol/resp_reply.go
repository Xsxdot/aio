// Package protocol 实现Redis序列化协议(RESP)
package protocol

import (
	"bytes"
	"github.com/xsxdot/aio/internal/cache"
	"github.com/xsxdot/aio/pkg/network"
	"strconv"
	"strings"
)

// RESPReply 实现RESP协议回复
type RESPReply struct {
	// 回复类型
	replyType cache.ReplyType
	// 字符串形式的回复内容
	content string
	// 字节形式的回复内容
	data []byte
}

// NewStatusReply 创建状态回复
func NewStatusReply(status string) *RESPReply {
	return &RESPReply{
		replyType: cache.ReplyStatus,
		content:   status,
	}
}

// NewErrorReply 创建错误回复
func NewErrorReply(err string) *RESPReply {
	return &RESPReply{
		replyType: cache.ReplyError,
		content:   err,
	}
}

// NewIntegerReply 创建整数回复
func NewIntegerReply(value int64) *RESPReply {
	return &RESPReply{
		replyType: cache.ReplyInteger,
		content:   strconv.FormatInt(value, 10),
	}
}

// NewBulkReply 创建批量回复
func NewBulkReply(value string) *RESPReply {
	return &RESPReply{
		replyType: cache.ReplyBulk,
		content:   value,
	}
}

// NewMultiBulkReply 创建多批量回复
func NewMultiBulkReply(values []string) *RESPReply {
	// 特殊处理空切片，返回空数组而不是包含一个空字符串的数组
	if len(values) == 0 {
		return &RESPReply{
			replyType: cache.ReplyMultiBulk,
			content:   "",
		}
	}

	// 使用特殊分隔符和长度前缀，避免内容中包含分隔符导致解析错误
	// 格式: ${length}:${content}|${length}:${content}|...
	var builder strings.Builder
	for i, v := range values {
		if i > 0 {
			builder.WriteString("|")
		}
		builder.WriteString(strconv.Itoa(len(v)))
		builder.WriteString(":")
		builder.WriteString(v)
	}

	return &RESPReply{
		replyType: cache.ReplyMultiBulk,
		content:   builder.String(),
	}
}

// NewNilReply 创建空回复
func NewNilReply() *RESPReply {
	return &RESPReply{
		replyType: cache.ReplyNil,
	}
}

// NewSimpleStringReply 创建简单字符串回复
func NewSimpleStringReply(str string) *RESPReply {
	return &RESPReply{
		replyType: cache.ReplyStatus,
		content:   str,
	}
}

// NewArrayReply 创建数组回复，接受任意类型的值数组
func NewArrayReply(values []interface{}) *RESPReply {
	// 特殊处理空切片，返回空数组
	if len(values) == 0 {
		return &RESPReply{
			replyType: cache.ReplyMultiBulk,
			content:   "",
		}
	}

	var buffer bytes.Buffer
	buffer.WriteByte(Array)
	buffer.WriteString(strconv.Itoa(len(values)))
	buffer.WriteString(CRLF)

	for _, value := range values {
		// 根据值的类型进行不同的处理
		switch v := value.(type) {
		case *RESPReply:
			// 如果是已经构建好的回复，直接添加序列化结果
			buffer.Write(v.serialize())
		case string:
			// 如果是字符串，作为批量字符串处理
			serializeBulkString(&buffer, v)
		case int:
			// 如果是整数，转换为字符串
			buffer.WriteByte(Integer)
			buffer.WriteString(strconv.Itoa(v))
			buffer.WriteString(CRLF)
		case int64:
			// 如果是64位整数，转换为字符串
			buffer.WriteByte(Integer)
			buffer.WriteString(strconv.FormatInt(v, 10))
			buffer.WriteString(CRLF)
		case nil:
			// 如果是nil，作为null批量字符串处理
			buffer.WriteByte(BulkString)
			buffer.WriteString("-1")
			buffer.WriteString(CRLF)
		default:
			// 默认处理为空字符串
			serializeBulkString(&buffer, "")
		}
	}

	return &RESPReply{
		replyType: cache.ReplyMultiBulk,
		data:      buffer.Bytes(),
	}
}

// Type 返回回复类型
func (r *RESPReply) Type() cache.ReplyType {
	return r.replyType
}

// String 返回回复的字符串形式
func (r *RESPReply) String() string {
	return r.content
}

// Bytes 返回回复的字节数组形式
func (r *RESPReply) Bytes() []byte {
	if r.data == nil {
		r.data = []byte(r.content)
	}
	return r.data
}

// ToMessage 转换为网络消息
func (r *RESPReply) ToMessage() network.Message {
	// 序列化回复
	data := r.serialize()
	// 创建消息
	return NewRESPMessage(MessageReply, data)
}

// serialize 序列化回复为字节数据
func (r *RESPReply) serialize() []byte {
	var buffer bytes.Buffer

	switch r.Type() {
	case cache.ReplyStatus:
		buffer.WriteByte(SimpleString)
		buffer.WriteString(r.String())
		buffer.WriteString(CRLF)
	case cache.ReplyError:
		buffer.WriteByte(Error)
		buffer.WriteString(r.String())
		buffer.WriteString(CRLF)
	case cache.ReplyInteger:
		buffer.WriteByte(Integer)
		buffer.WriteString(r.String())
		buffer.WriteString(CRLF)
	case cache.ReplyBulk:
		serializeBulkString(&buffer, r.String())
	case cache.ReplyMultiBulk:
		// 特殊处理空内容的多批量回复，表示空数组
		if r.String() == "" {
			buffer.WriteByte(Array)
			buffer.WriteString("0")
			buffer.WriteString(CRLF)
			return buffer.Bytes()
		}

		// 解析自定义格式: ${length}:${content}|${length}:${content}|...
		content := r.String()
		values := []string{}
		parts := strings.Split(content, "|")

		for _, part := range parts {
			if len(part) == 0 {
				continue
			}

			colonPos := strings.Index(part, ":")
			if colonPos == -1 {
				continue
			}

			lengthStr := part[:colonPos]
			length, err := strconv.Atoi(lengthStr)
			if err != nil || length < 0 {
				continue
			}

			// 提取值内容
			if colonPos+1+length <= len(part) {
				value := part[colonPos+1 : colonPos+1+length]
				values = append(values, value)
			}
		}

		buffer.WriteByte(Array)
		buffer.WriteString(strconv.Itoa(len(values)))
		buffer.WriteString(CRLF)

		for _, value := range values {
			serializeBulkString(&buffer, value)
		}
	case cache.ReplyNil:
		buffer.WriteByte(BulkString)
		buffer.WriteString("-1")
		buffer.WriteString(CRLF)
	default:
		// 未知类型作为空字符串处理
		serializeBulkString(&buffer, "")
	}

	return buffer.Bytes()
}

// serializeBulkString 将字符串序列化为批量字符串格式，作为辅助函数
func serializeBulkString(buffer *bytes.Buffer, str string) {
	buffer.WriteByte(BulkString)
	buffer.WriteString(strconv.Itoa(len(str)))
	buffer.WriteString(CRLF)
	buffer.WriteString(str)
	buffer.WriteString(CRLF)
}

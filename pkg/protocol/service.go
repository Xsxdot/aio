package protocol

// Request 表示服务请求
type Request struct {
	// ServiceType 服务类型
	ServiceType ServiceType
	// Command 命令
	Command string
	// Payload 请求负载
	Payload []byte
	// ConnID 连接ID
	ConnID string
	// MessageID 消息ID
	MessageID string
}

// Response 表示服务响应
type Response struct {
	// Status 响应状态码
	Status int
	// Message 响应消息
	Message string
	// Payload 响应负载
	Payload []byte
	// Error 错误信息
	Error string
}

// NewRequest 创建新的请求
func NewRequest(serviceType ServiceType, command string, payload []byte, connID string, messageID string) *Request {
	return &Request{
		ServiceType: serviceType,
		Command:     command,
		Payload:     payload,
		ConnID:      connID,
		MessageID:   messageID,
	}
}

// NewResponse 创建新的响应
func NewResponse(status int, message string, payload []byte) *Response {
	return &Response{
		Status:  status,
		Message: message,
		Payload: payload,
	}
}

// NewErrorResponse 创建错误响应
func NewErrorResponse(status int, errorMsg string) *Response {
	return &Response{
		Status: status,
		Error:  errorMsg,
	}
}

// Size 获取请求大小
func (r *Request) Size() int {
	return len(r.Payload)
}

// Size 获取响应大小
func (r *Response) Size() int {
	return len(r.Payload)
}

// IsSuccess 判断响应是否成功
func (r *Response) IsSuccess() bool {
	return r.Status >= 200 && r.Status < 300
}

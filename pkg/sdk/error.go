package sdk

import (
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Error SDK 错误包装
type Error struct {
	Code    codes.Code
	Message string
	Cause   error
}

// Error 实现 error 接口
func (e *Error) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("sdk error: %s (code: %s, cause: %v)", e.Message, e.Code, e.Cause)
	}
	return fmt.Sprintf("sdk error: %s (code: %s)", e.Message, e.Code)
}

// Unwrap 支持 errors.Unwrap
func (e *Error) Unwrap() error {
	return e.Cause
}

// WrapError 包装 gRPC 错误
func WrapError(err error, message string) error {
	if err == nil {
		return nil
	}

	// 提取 gRPC 状态码
	code := codes.Unknown
	if st, ok := status.FromError(err); ok {
		code = st.Code()
	}

	return &Error{
		Code:    code,
		Message: message,
		Cause:   err,
	}
}

// IsNotFound 判断是否为 NotFound 错误
func IsNotFound(err error) bool {
	if e, ok := err.(*Error); ok {
		return e.Code == codes.NotFound
	}
	if st, ok := status.FromError(err); ok {
		return st.Code() == codes.NotFound
	}
	return false
}

// IsUnauthenticated 判断是否为未认证错误
func IsUnauthenticated(err error) bool {
	if e, ok := err.(*Error); ok {
		return e.Code == codes.Unauthenticated
	}
	if st, ok := status.FromError(err); ok {
		return st.Code() == codes.Unauthenticated
	}
	return false
}

// IsUnavailable 判断是否为服务不可用错误
func IsUnavailable(err error) bool {
	if e, ok := err.(*Error); ok {
		return e.Code == codes.Unavailable
	}
	if st, ok := status.FromError(err); ok {
		return st.Code() == codes.Unavailable
	}
	return false
}

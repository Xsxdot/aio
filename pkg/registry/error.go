package registry

import (
	"fmt"
)

// 错误码定义
const (
	ErrCodeInvalidConfig    = "INVALID_CONFIG"    // 无效配置
	ErrCodeConnectionFailed = "CONNECTION_FAILED" // 连接失败
	ErrCodeRegistryFailed   = "REGISTRY_FAILED"   // 注册失败
	ErrCodeDiscoveryFailed  = "DISCOVERY_FAILED"  // 发现失败
	ErrCodeServiceNotFound  = "SERVICE_NOT_FOUND" // 服务未找到
	ErrCodeWatchFailed      = "WATCH_FAILED"      // 监听失败
	ErrCodeUnregisterFailed = "UNREGISTER_FAILED" // 注销失败
	ErrCodeLeaseError       = "LEASE_ERROR"       // 租约错误
	ErrCodeOperationTimeout = "OPERATION_TIMEOUT" // 操作超时
	ErrCodeInvalidInstance  = "INVALID_INSTANCE"  // 无效实例
)

// RegistryError 注册中心错误
type RegistryError struct {
	Code    string `json:"code"`    // 错误码
	Message string `json:"message"` // 错误消息
	Cause   error  `json:"cause"`   // 原因错误
}

// Error 实现error接口
func (e *RegistryError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap 支持错误链
func (e *RegistryError) Unwrap() error {
	return e.Cause
}

// NewRegistryError 创建注册中心错误
func NewRegistryError(code, message string) *RegistryError {
	return &RegistryError{
		Code:    code,
		Message: message,
	}
}

// NewRegistryErrorWithCause 创建带原因的注册中心错误
func NewRegistryErrorWithCause(code, message string, cause error) *RegistryError {
	return &RegistryError{
		Code:    code,
		Message: message,
		Cause:   cause,
	}
}

// IsRegistryError 检查是否为注册中心错误
func IsRegistryError(err error) bool {
	_, ok := err.(*RegistryError)
	return ok
}

// GetErrorCode 获取错误码
func GetErrorCode(err error) string {
	if regErr, ok := err.(*RegistryError); ok {
		return regErr.Code
	}
	return ""
}

// 预定义的常见错误
var (
	ErrInvalidServiceInstance = NewRegistryError(ErrCodeInvalidInstance, "invalid service instance")
	ErrServiceNotFound        = NewRegistryError(ErrCodeServiceNotFound, "service not found")
	ErrRegistryFailed         = NewRegistryError(ErrCodeRegistryFailed, "service registration failed")
	ErrDiscoveryFailed        = NewRegistryError(ErrCodeDiscoveryFailed, "service discovery failed")
	ErrWatchFailed            = NewRegistryError(ErrCodeWatchFailed, "service watch failed")
	ErrUnregisterFailed       = NewRegistryError(ErrCodeUnregisterFailed, "service unregister failed")
	ErrConnectionFailed       = NewRegistryError(ErrCodeConnectionFailed, "etcd connection failed")
	ErrLeaseError             = NewRegistryError(ErrCodeLeaseError, "lease operation failed")
	ErrOperationTimeout       = NewRegistryError(ErrCodeOperationTimeout, "operation timeout")
)

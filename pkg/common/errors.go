package common

import (
	"errors"
	"fmt"
	"net/http"
	"runtime"
	"strings"
)

// ErrorType 错误类型
type ErrorType uint

const (
	// ErrorTypeNormal 普通错误
	ErrorTypeNormal ErrorType = iota
	// ErrorTypeValidation 验证错误
	ErrorTypeValidation
	// ErrorTypeUnauthorized 未授权错误
	ErrorTypeUnauthorized
	// ErrorTypeForbidden 禁止访问错误
	ErrorTypeForbidden
	// ErrorTypeNotFound 未找到错误
	ErrorTypeNotFound
	// ErrorTypeInternal 内部错误
	ErrorTypeInternal
	// ErrorTypeExternal 外部服务错误
	ErrorTypeExternal
	// ErrorTypeTimeout 超时错误
	ErrorTypeTimeout
	// ErrorTypeConflict 冲突错误
	ErrorTypeConflict
	// ErrorTypeUnavailable 服务不可用错误
	ErrorTypeUnavailable
)

// AppError 应用错误
type AppError struct {
	// Type 错误类型
	Type ErrorType
	// Code 错误代码
	Code string
	// Message 错误消息
	Message string
	// Err 原始错误
	Err error
	// Fields 相关字段
	Fields map[string]interface{}
	// Stack 调用栈
	Stack string
}

// Error 实现error接口
func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap 实现errors.Unwrap接口
func (e *AppError) Unwrap() error {
	return e.Err
}

// statusCode 返回对应的HTTP状态码
func (e *AppError) statusCode() int {
	switch e.Type {
	case ErrorTypeValidation:
		return http.StatusBadRequest
	case ErrorTypeUnauthorized:
		return http.StatusUnauthorized
	case ErrorTypeForbidden:
		return http.StatusForbidden
	case ErrorTypeNotFound:
		return http.StatusNotFound
	case ErrorTypeConflict:
		return http.StatusConflict
	case ErrorTypeTimeout:
		return http.StatusRequestTimeout
	case ErrorTypeUnavailable:
		return http.StatusServiceUnavailable
	case ErrorTypeExternal:
		return http.StatusBadGateway
	case ErrorTypeInternal:
		return http.StatusInternalServerError
	default:
		return http.StatusInternalServerError
	}
}

// WithField 添加字段信息
func (e *AppError) WithField(key string, value interface{}) *AppError {
	if e.Fields == nil {
		e.Fields = make(map[string]interface{})
	}
	e.Fields[key] = value
	return e
}

// WithFields 添加多个字段信息
func (e *AppError) WithFields(fields map[string]interface{}) *AppError {
	if e.Fields == nil {
		e.Fields = make(map[string]interface{})
	}
	for k, v := range fields {
		e.Fields[k] = v
	}
	return e
}

// Response 生成错误响应
func (e *AppError) Response() map[string]interface{} {
	resp := map[string]interface{}{
		"code":    e.Code,
		"message": e.Message,
		"status":  e.statusCode(),
	}
	if len(e.Fields) > 0 {
		resp["details"] = e.Fields
	}
	if e.Err != nil && ErrorDebugMode {
		resp["error"] = e.Err.Error()
		resp["stack"] = e.Stack
	}
	return resp
}

// NewAppError 创建应用错误
func NewAppError(errType ErrorType, code string, message string, err error) *AppError {
	var stack string
	if ErrorDebugMode {
		stack = captureStack(2)
	}
	return &AppError{
		Type:    errType,
		Code:    code,
		Message: message,
		Err:     err,
		Stack:   stack,
	}
}

// captureStack 捕获调用栈
func captureStack(skip int) string {
	stackBuf := make([]uintptr, 50)
	length := runtime.Callers(skip+1, stackBuf)
	stack := stackBuf[:length]

	frames := runtime.CallersFrames(stack)
	var result strings.Builder

	for {
		frame, more := frames.Next()
		if !more {
			break
		}
		if !strings.Contains(frame.Function, "runtime.") {
			result.WriteString(fmt.Sprintf("%s\n\t%s:%d\n", frame.Function, frame.File, frame.Line))
		}
		if !more {
			break
		}
	}
	return result.String()
}

// IsAppError 检查错误是否为AppError类型
func IsAppError(err error) bool {
	var appErr *AppError
	return errors.As(err, &appErr)
}

// ToAppError 将普通错误转换为AppError
func ToAppError(err error) *AppError {
	if err == nil {
		return nil
	}

	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr
	}

	return NewAppError(ErrorTypeNormal, "UNKNOWN", err.Error(), err)
}

// ErrorDebugMode 是否启用调试模式
var ErrorDebugMode bool = false

// EnableErrorDebug 启用错误调试模式
func EnableErrorDebug() {
	ErrorDebugMode = true
}

// DisableErrorDebug 禁用错误调试模式
func DisableErrorDebug() {
	ErrorDebugMode = false
}

// NewValidationError 创建验证错误
func NewValidationError(message string, err error) *AppError {
	return NewAppError(ErrorTypeValidation, "VALIDATION_ERROR", message, err)
}

// NewUnauthorizedError 创建未授权错误
func NewUnauthorizedError(message string, err error) *AppError {
	return NewAppError(ErrorTypeUnauthorized, "UNAUTHORIZED", message, err)
}

// NewForbiddenError 创建禁止访问错误
func NewForbiddenError(message string, err error) *AppError {
	return NewAppError(ErrorTypeForbidden, "FORBIDDEN", message, err)
}

// NewNotFoundError 创建未找到错误
func NewNotFoundError(message string, err error) *AppError {
	return NewAppError(ErrorTypeNotFound, "NOT_FOUND", message, err)
}

// NewInternalError 创建内部错误
func NewInternalError(message string, err error) *AppError {
	return NewAppError(ErrorTypeInternal, "INTERNAL_ERROR", message, err)
}

// NewExternalError 创建外部服务错误
func NewExternalError(message string, err error) *AppError {
	return NewAppError(ErrorTypeExternal, "EXTERNAL_ERROR", message, err)
}

// NewTimeoutError 创建超时错误
func NewTimeoutError(message string, err error) *AppError {
	return NewAppError(ErrorTypeTimeout, "TIMEOUT", message, err)
}

// NewConflictError 创建冲突错误
func NewConflictError(message string, err error) *AppError {
	return NewAppError(ErrorTypeConflict, "CONFLICT", message, err)
}

// NewUnavailableError 创建服务不可用错误
func NewUnavailableError(message string, err error) *AppError {
	return NewAppError(ErrorTypeUnavailable, "SERVICE_UNAVAILABLE", message, err)
}

package errorc

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"xiaozhizhang/pkg/core/consts"

	"github.com/sirupsen/logrus"

	"github.com/redis/go-redis/v9"

	//"github.com/sirupsen/logrus"
	"runtime"

	"gopkg.in/mgo.v2"
	"gorm.io/gorm"
)

// 配置选项
var (
	enableFullStack = true // 可以通过环境变量或配置文件控制
	stackBufferPool = sync.Pool{
		New: func() interface{} {
			return make([]byte, 4096)
		},
	}
)

type ErrorBuilder struct {
	entryName string
}

func NewErrorBuilder(entryName string) *ErrorBuilder {
	return &ErrorBuilder{entryName: entryName}
}

func (e *ErrorBuilder) New(msg string, err error) *Error {
	stack := getStackOptimized(2)
	stack.Msg = msg
	stack.Cause = err
	stack.Entry = e.entryName
	stack.ErrorCode = getErrCode(err)
	return stack
}

// New err or msg can nil
func New(msg string, err error) *Error {
	stack := getStackOptimized(2)
	stack.Msg = msg
	stack.Cause = err
	stack.ErrorCode = getErrCode(err)
	return stack
}

func (e *Error) WithTraceID(ctx context.Context) *Error {
	var traceID string

	if ctx != nil {
		// span := zipkin.SpanFromContext(ctx)
		// if span == nil {
		// 	if uuid, ok := ctx.Value(consts.TraceKey).(string); ok {
		// 		traceID = uuid
		// 	} else {
		// 		traceID = ""
		// 	}
		// } else {
		// 	traceID = span.Context().TraceID.String()
		// 	span.Tag("error", "true")
		// }
		if uuid, ok := ctx.Value(consts.TraceKey).(string); ok {
			traceID = uuid
		} else {
			traceID = ""
		}
	} else {
		traceID = ""
	}
	e.TraceID = traceID
	return e
}

func (e *Error) WithEntry(entry string) *Error {
	e.Entry = entry
	return e
}

func (e *Error) WithCode(code *ErrorCode) *Error {
	e.ErrorCode = code
	return e
}

func (e *Error) DB() *Error {
	if e.Code == 404 {
		return e
	}
	e.ErrorCode = ErrorCodeDB
	return e
}

func (e *Error) Third() *Error {
	e.ErrorCode = ErrorCodeThird
	return e
}

func (e *Error) ValidWithCtx() *Error {
	e.ErrorCode = ErrorCodeValid
	return e
}

func (e *Error) NoAuth() *Error {
	e.ErrorCode = ErrorCodeNoAuth
	return e
}

func (e *Error) Forbidden() *Error {
	e.ErrorCode = ErrorCodeForbidden
	return e
}

func (e *Error) NotFound() *Error {
	e.ErrorCode = ErrorCodeNotFound
	return e
}

func (e *Error) Unavailable() *Error {
	e.ErrorCode = ErrorCodeUnavailable
	return e
}

func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}

	// 1. 收集错误链
	var errChain []*Error
	currErr := e
	for {
		errChain = append(errChain, currErr)
		if cause, ok := currErr.Cause.(*Error); ok {
			currErr = cause
		} else {
			break
		}
	}

	// 2. 查找根因错误 (第一个包装了非 *Error 错误的 Error)
	var rootCause *Error
	var originalError error // 底层的原始错误
	for i := len(errChain) - 1; i >= 0; i-- {
		err := errChain[i]
		if err.Cause != nil {
			if _, ok := err.Cause.(*Error); !ok {
				rootCause = err
				originalError = err.Cause
				break
			}
		}
	}
	// 如果没找到明确的包装了第三方错误的error，就认为最内层的就是根因
	if rootCause == nil && len(errChain) > 0 {
		rootCause = errChain[len(errChain)-1]
		originalError = rootCause.Cause
	}

	// 3. 构建格式化的错误信息
	var sb strings.Builder

	// 3.1 打印根因
	sb.WriteString("========================= Root Cause =========================\n")
	if rootCause != nil {
		if originalError != nil {
			sb.WriteString(fmt.Sprintf("Error: %s\n", originalError.Error()))
		}
		if rootCause.FileName != "" {
			sb.WriteString(fmt.Sprintf("Location: %s:%d\n", rootCause.FileName, rootCause.Line))
		}
		if rootCause.FuncName != "" {
			sb.WriteString(fmt.Sprintf("Function: %s\n", rootCause.FuncName))
		}
		if rootCause.Msg != "" {
			sb.WriteString(fmt.Sprintf("Message: %s\n", rootCause.Msg))
		}
		if rootCause.TraceID != "" {
			sb.WriteString(fmt.Sprintf("Trace ID: %s\n", rootCause.TraceID))
		}
	} else {
		sb.WriteString("No specific root cause identified.\n")
	}

	// 3.2 打印完整错误跟踪链
	sb.WriteString("\n======================= Full Error Trace =======================\n")
	for i, err := range errChain {
		sb.WriteString(fmt.Sprintf("%d: ", i+1))
		if err.ErrorCode != nil {
			sb.WriteString(fmt.Sprintf("[%s] ", err.ErrorCode.String()))
		}
		sb.WriteString(err.Msg)

		if err.FileName != "" {
			sb.WriteString(fmt.Sprintf("\n   at %s:%d", err.FileName, err.Line))
		}
		sb.WriteString("\n")
	}
	sb.WriteString("==============================================================\n")

	return sb.String()
}

// RootCause returns a simple string representing the root cause of the error.
func (e *Error) RootCause() string {
	if e == nil {
		return ""
	}

	// 1. 收集错误链
	var errChain []*Error
	currErr := e
	for {
		errChain = append(errChain, currErr)
		if cause, ok := currErr.Cause.(*Error); ok {
			currErr = cause
		} else {
			break
		}
	}

	// 2. 查找根因错误
	var rootCause *Error
	var originalError error // 底层的原始错误
	for i := len(errChain) - 1; i >= 0; i-- {
		err := errChain[i]
		if err.Cause != nil {
			if _, ok := err.Cause.(*Error); !ok {
				rootCause = err
				originalError = err.Cause
				break
			}
		}
	}
	if rootCause == nil && len(errChain) > 0 {
		rootCause = errChain[len(errChain)-1]
		originalError = rootCause.Cause
	}

	if rootCause == nil {
		return e.Msg // Fallback for safety
	}

	// 3. Format the concise string.
	var sb strings.Builder
	sb.WriteString(rootCause.Msg)

	if originalError != nil {
		sb.WriteString(fmt.Sprintf(": %v", originalError))
	}

	if rootCause.FileName != "" {
		sb.WriteString(fmt.Sprintf(" at %s:%d", rootCause.FileName, rootCause.Line))
	}

	return sb.String()
}

func (e *Error) ToLog(log *logrus.Entry, msgs ...string) *Error {
	if e == nil {
		return nil
	}

	// 1. 收集错误链
	var errChain []*Error
	currErr := e
	for {
		errChain = append(errChain, currErr)
		if cause, ok := currErr.Cause.(*Error); ok {
			currErr = cause
		} else {
			break
		}
	}

	// 2. 查找根因错误
	var rootCause *Error
	var originalError error
	for i := len(errChain) - 1; i >= 0; i-- {
		err := errChain[i]
		if err.Cause != nil {
			if _, ok := err.Cause.(*Error); !ok {
				rootCause = err
				originalError = err.Cause
				break
			}
		}
	}
	if rootCause == nil && len(errChain) > 0 {
		rootCause = errChain[len(errChain)-1]
		originalError = rootCause.Cause
	}

	// 3. 构建日志字段
	fields := make(map[string]interface{})

	// 3.1 添加根因信息
	if rootCause != nil {
		fields["root_cause_file"] = rootCause.FileName
		fields["root_cause_line"] = rootCause.Line
		fields["root_cause_func"] = rootCause.FuncName
		fields["root_cause_msg"] = rootCause.Msg
		if originalError != nil {
			fields["root_cause_original_error"] = originalError.Error()
		}
		if rootCause.ErrorCode != nil {
			fields["root_cause_error_code"] = rootCause.ErrorCode.String()
		}
	}

	// 3.2 添加完整的错误链信息
	chain := make([]map[string]interface{}, 0, len(errChain))
	for _, err := range errChain {
		level := make(map[string]interface{})
		level["file"] = err.FileName
		level["line"] = err.Line
		level["func"] = err.FuncName
		level["msg"] = err.Msg
		if err.ErrorCode != nil {
			level["code"] = err.ErrorCode.String()
		}
		if err.TraceID != "" {
			level["trace_id"] = err.TraceID
		}
		if err == e && enableFullStack { // 只为最外层错误添加完整堆栈
			stack := err.getFullStack()
			if stack != "" {
				level["stack_trace"] = stack
			}
		}
		chain = append(chain, level)
	}
	fields["error_chain"] = chain
	if e.TraceID != "" {
		fields["trace_id"] = e.TraceID
	}

	// 4. 构建最终日志消息
	var finalMsg string
	if len(msgs) > 0 {
		finalMsg = strings.Join(msgs, ", ")
	} else if len(errChain) > 0 {
		finalMsg = errChain[0].Msg // 使用最外层错误的Msg作为主要信息
	} else {
		finalMsg = "An error occurred"
	}

	log.WithFields(fields).Error(finalMsg)
	println(e.Error())
	return e
}

// getStackOptimized 优化的堆栈获取函数
func getStackOptimized(num int) *Error {
	// 获取调用栈信息（轻量级操作）
	pc, file, line, ok := runtime.Caller(num)
	if !ok {
		return &Error{
			FileName: "<unknown>",
			Line:     0,
			FuncName: "<unknown>",
		}
	}

	var funcName string
	if details := runtime.FuncForPC(pc); details != nil {
		funcName = details.Name()
	} else {
		funcName = "<unknown>"
	}

	return &Error{
		FileName: file,
		Line:     line,
		FuncName: funcName,
		// Stack字段延迟计算，不在这里获取
	}
}

// getFullStack 延迟获取完整堆栈信息
func (e *Error) getFullStack() string {
	if e.Stack != "" {
		return e.Stack
	}

	if !enableFullStack {
		return ""
	}

	// 从池中获取buffer
	buf := stackBufferPool.Get().([]byte)
	defer stackBufferPool.Put(buf)

	// 获取堆栈信息
	n := runtime.Stack(buf, false)
	e.Stack = string(buf[:n])

	return e.Stack
}

// getStack 保留原函数以兼容，但标记为已废弃
// Deprecated: 使用getStackOptimized替代
func getStack(num int) *Error {
	return getStackOptimized(num)
}

// SetStackTraceEnabled 控制是否启用完整堆栈跟踪
func SetStackTraceEnabled(enabled bool) {
	enableFullStack = enabled
}

// IsStackTraceEnabled 检查是否启用完整堆栈跟踪
func IsStackTraceEnabled() bool {
	return enableFullStack
}

func getErrCode(err error) *ErrorCode {
	if err == nil {
		return ErrorCodeUnknown
	}

	for _, e := range notfounds {
		if errors.Is(err, e) {
			return ErrorCodeNotFound
		}
	}

	return ErrorCodeUnknown
}

var notfounds = []error{gorm.ErrRecordNotFound, redis.Nil, mgo.ErrNotFound}

// 快速构造函数 - 不获取堆栈信息，适用于性能敏感场景
func (e *ErrorBuilder) Quick(msg string, err error) *Error {
	return &Error{
		Msg:       msg,
		Cause:     err,
		Entry:     e.entryName,
		ErrorCode: getErrCode(err),
	}
}

// 快速构造函数 - 全局版本
func Quick(msg string, err error) *Error {
	return &Error{
		Msg:       msg,
		Cause:     err,
		ErrorCode: getErrCode(err),
	}
}

// 快速构造特定错误类型的方法
func (e *ErrorBuilder) NotFound(msg string) *Error {
	return &Error{
		Msg:       msg,
		Entry:     e.entryName,
		ErrorCode: ErrorCodeNotFound,
	}
}

func (e *ErrorBuilder) Internal(msg string) *Error {
	return &Error{
		Msg:       msg,
		Entry:     e.entryName,
		ErrorCode: ErrorCodeInternal,
	}
}

func (e *ErrorBuilder) BadRequest(msg string) *Error {
	return &Error{
		Msg:       msg,
		Entry:     e.entryName,
		ErrorCode: ErrorCodeValid,
	}
}

func (e *ErrorBuilder) Unauthorized(msg string) *Error {
	return &Error{
		Msg:       msg,
		Entry:     e.entryName,
		ErrorCode: ErrorCodeNoAuth,
	}
}

func (e *ErrorBuilder) Forbidden(msg string) *Error {
	return &Error{
		Msg:       msg,
		Entry:     e.entryName,
		ErrorCode: ErrorCodeForbidden,
	}
}

// WithCause 链式添加原因错误
func (e *Error) WithCause(err error) *Error {
	if e != nil {
		e.Cause = err
	}
	return e
}

// WithStackTrace 按需添加堆栈跟踪
func (e *Error) WithStackTrace() *Error {
	if e == nil {
		return nil
	}
	// 获取调用栈信息（从调用WithStackTrace的位置开始）
	pc, file, line, ok := runtime.Caller(1)
	if ok {
		if details := runtime.FuncForPC(pc); details != nil {
			e.FuncName = details.Name()
		}
		e.FileName = file
		e.Line = line
	}
	return e
}

func ParseError(err error) *Error {
	if err == nil {
		return nil
	}

	var e *Error
	// Use errors.As to check if an *Error already exists in the chain.
	if errors.As(err, &e) {
		return e
	}

	// If not, wrap the original error.
	return Quick("", err)
}

func IsNotFound(err error) bool {
	if err == nil {
		return false
	}

	// 1. Check if it's our custom NotFound error, anywhere in the chain.
	var e *Error
	if errors.As(err, &e) {
		if e.ErrorCode == ErrorCodeNotFound {
			return true
		}
	}

	// 2. Check for other known 'not found' error values in the chain.
	for _, target := range notfounds {
		if errors.Is(err, target) {
			return true
		}
	}

	return false
}

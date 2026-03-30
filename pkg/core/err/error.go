package errorc

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"
	"github.com/xsxdot/aio/pkg/core/consts"

	"runtime"
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

	// 1. 收集错误链（带防死循环保护）
	var errChain []*Error
	currErr := e
	depth := 0
	for currErr != nil && depth < 20 {
		errChain = append(errChain, currErr)
		depth++
		if cause, ok := currErr.Cause.(*Error); ok && cause != nil && cause != currErr {
			currErr = cause
		} else {
			break
		}
	}

	lastErr := errChain[len(errChain)-1]
	originalErr := lastErr.Cause

	var sb strings.Builder

	// 2. 简洁的单行报错汇总 (Outer -> Inner -> Root)
	sb.WriteString("Err: ")
	for i, err := range errChain {
		if i > 0 {
			sb.WriteString(" -> ")
		}
		sb.WriteString(err.Msg)
	}
	if originalErr != nil && originalErr != lastErr {
		sb.WriteString(fmt.Sprintf(" -> %v", originalErr))
	}
	sb.WriteString("\n")

	// 3. 核心追踪信息
	if e.TraceID != "" {
		sb.WriteString(fmt.Sprintf("[TraceID: %s]\n", e.TraceID))
	}

	// 4. 干净的逻辑调用栈
	sb.WriteString("--- Logical Stack Trace ---\n")
	for i, err := range errChain {
		codeStr := "Unknown"
		if err.ErrorCode != nil {
			codeStr = err.ErrorCode.Name
		}
		// 打印：1. [NotFound] 找不到用户数据
		sb.WriteString(fmt.Sprintf("  %d. [%s] %s\n", i+1, codeStr, err.Msg))
		// 打印：    @ core/mvc/controller.go:45 (FindById)
		if err.FileName != "" {
			funcName := trimFilename(err.FuncName) // 函数名也可能带长包名，稍微精简
			sb.WriteString(fmt.Sprintf("     @ %s:%d (%s)\n", trimFilename(err.FileName), err.Line, funcName))
		}
	}

	// 5. 打印最底层的原生错误（如 sql.ErrNoRows）
	if originalErr != nil && originalErr != lastErr {
		sb.WriteString(fmt.Sprintf("  %d. [Root Cause] %v\n", len(errChain)+1, originalErr))
	}

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
	depth := 0
	for depth < 20 {
		errChain = append(errChain, currErr)
		if cause, ok := currErr.Cause.(*Error); ok && cause != nil {
			// 防止自引用导致的死循环
			if cause == currErr {
				break
			}
			currErr = cause
			depth++
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

	var errChain []*Error
	currErr := e
	depth := 0
	for currErr != nil && depth < 20 {
		errChain = append(errChain, currErr)
		depth++
		if cause, ok := currErr.Cause.(*Error); ok && cause != nil && cause != currErr {
			currErr = cause
		} else {
			break
		}
	}

	lastErr := errChain[len(errChain)-1]
	originalErr := lastErr.Cause

	// 构建用于 JSON 检索的结构化字段
	fields := make(map[string]interface{})

	if e.TraceID != "" {
		fields["trace_id"] = e.TraceID
	}

	// 记录原始的底层错误
	if originalErr != nil {
		fields["root_cause"] = originalErr.Error()
	}

	// 构建逻辑错误链数组
	chain := make([]map[string]interface{}, 0, len(errChain))
	for _, err := range errChain {
		level := make(map[string]interface{})
		level["msg"] = err.Msg
		level["location"] = fmt.Sprintf("%s:%d", trimFilename(err.FileName), err.Line)
		if err.ErrorCode != nil {
			level["code"] = err.ErrorCode.Name
		}
		chain = append(chain, level)
	}
	fields["error_chain"] = chain

	// 最终日志消息：如果有自定义传参优先用传参，否则用错误链的单行汇总
	var finalMsg string
	if len(msgs) > 0 {
		finalMsg = strings.Join(msgs, ", ")
	} else {
		// 取最外层的 Msg 作为日志标题
		finalMsg = e.Msg
	}

	// 输出结构化日志
	log.WithFields(fields).Error(finalMsg)

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

	// 避免触发自定义 Error 的堆栈序列化
	var msg string
	if e, ok := err.(*Error); ok {
		msg = e.Msg
		if e.Cause != nil {
			if ce, ok := e.Cause.(*Error); ok {
				msg += " " + ce.Msg
			} else {
				msg += " " + e.Cause.Error()
			}
		}
	} else {
		msg = err.Error()
	}

	if isNotFoundMessage(msg) {
		return ErrorCodeNotFound
	}
	return ErrorCodeUnknown
}

// 内置的 NotFound 关键词列表（小写）
var notFoundKeywords = []string{
	"record not found",
	"redis: nil",
}

// isNotFoundMessage 判断错误消息是否包含 NotFound 关键词
func isNotFoundMessage(msg string) bool {
	lowerMsg := strings.ToLower(msg)
	for _, kw := range notFoundKeywords {
		if strings.Contains(lowerMsg, kw) {
			return true
		}
	}
	return false
}

// RegisterNotFoundKeyword 允许用户注册自定义的 NotFound 关键词
func RegisterNotFoundKeyword(keyword string) {
	notFoundKeywords = append(notFoundKeywords, strings.ToLower(keyword))
}

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
	stack := getStackOptimized(2) // 关键修复：抓取调用者位置
	stack.Msg = msg
	stack.Cause = err
	stack.ErrorCode = getErrCode(err)
	return stack
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
	if errors.As(err, &e) {
		return e
	}
	// 将 depth 改为 2，以准确抓取调用 ParseError 的那行业务代码
	stack := getStackOptimized(2)
	stack.Cause = err
	stack.ErrorCode = getErrCode(err)
	return stack
}

func IsNotFound(err error) bool {
	if err == nil {
		return false
	}

	// 1. 检查是否是我们自定义的 NotFound
	var e *Error
	if errors.As(err, &e) {
		if e.ErrorCode == ErrorCodeNotFound {
			return true
		}
		// 只判断 Msg 和 Cause，不调用 e.Error()
		if isNotFoundMessage(e.Msg) {
			return true
		}
		if e.Cause != nil {
			// 对 Cause 进行判断，但避免触发我们的 Error 的序列化
			if ce, ok := e.Cause.(*Error); ok {
				return IsNotFound(ce) // 递归处理嵌套的 Error
			}
			return isNotFoundMessage(e.Cause.Error())
		}
		return false
	}

	// 2. 第三方原生 error，直接判断 Message
	return isNotFoundMessage(err.Error())
}

// trimFilename 截断绝对路径，只保留项目相对路径，减少日志噪音
func trimFilename(fullPath string) string {
	if fullPath == "" {
		return ""
	}
	parts := strings.Split(fullPath, "/")
	if len(parts) > 3 {
		// 只保留最后 3 级路径，例如 "core/mvc/controller.go"
		return strings.Join(parts[len(parts)-3:], "/")
	}
	return fullPath
}

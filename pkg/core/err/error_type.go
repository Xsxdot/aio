package errorc

import (
	"fmt"
	"strings"
)

type Error struct {
	*ErrorCode
	Msg      string
	Cause    error
	Stack    string `json:"-"`
	TraceID  string
	Entry    string `json:"-"`
	FileName string `json:"-"`
	Line     int    `json:"-"`
	FuncName string `json:"-"`
}

func (e *Error) formatStack() string {
	if e.Stack == "" {
		return ""
	}

	// 按行分割堆栈信息
	lines := strings.Split(e.Stack, "\n")
	var filteredLines []string

	// 过滤掉不需要的堆栈信息
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		// 跳过测试相关的堆栈
		if strings.Contains(line, "/go/pkg/mod") || strings.Contains(line, "github.com/") {
			continue
		}
		// 跳过错误包内部的调用
		if strings.Contains(line, "github.com/xsxdot/aio/pkg/core/err.") || strings.Contains(line, "github.com/xsxdot/aio/pkg/core/err/error.go") {
			continue
		}
		filteredLines = append(filteredLines, line)
	}

	// 如果没有有效的堆栈信息，返回空字符串
	if len(filteredLines) == 0 {
		return ""
	}

	// 将过滤后的堆栈信息组合成字符串
	return strings.Join(filteredLines, "\n")
}

type ErrorCode struct {
	Code int
	Name string
}

func (c *ErrorCode) String() string {
	return fmt.Sprintf("%d: %s", c.Code, c.Name)
}

var (
	ErrorCodeUnknown     *ErrorCode = &ErrorCode{500, "Unknown"}
	ErrorCodeDB          *ErrorCode = &ErrorCode{501, "DB"}
	ErrorCodeThird       *ErrorCode = &ErrorCode{502, "Third"}
	ErrorCodeValid       *ErrorCode = &ErrorCode{400, "ValidWithCtx"}
	ErrorCodeNoAuth      *ErrorCode = &ErrorCode{401, "Unauthenticated"}
	ErrorCodeForbidden   *ErrorCode = &ErrorCode{403, "Forbidden"}
	ErrorCodeNotFound    *ErrorCode = &ErrorCode{404, "NotFound"}
	ErrorCodeUnavailable *ErrorCode = &ErrorCode{503, "Unavailable"}
	ErrorCodeInternal    *ErrorCode = &ErrorCode{503, "InternalError"}
)

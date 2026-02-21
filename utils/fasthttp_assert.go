package utils

import (
	"fmt"
	"strings"

	errorc "github.com/xsxdot/aio/pkg/core/err"
)

var errBuilder = errorc.NewErrorBuilder("FasthttpClient")

// buildAssertError 统一构造断言失败错误
// keys 非空时：从响应 JSON 中提取指定 keys 的值作为错误上下文
//   - 如果至少有一个 key 存在，则只显示提取的字段
//   - 如果所有 keys 都不存在或 body 为空，则 fallback 到完整响应
//
// keys 为空时：将完整响应（status + headers + body）作为错误上下文
func (r *FasthttpResponse) buildAssertError(baseMsg string, keys []string) error {
	var contextMsg strings.Builder
	contextMsg.WriteString(baseMsg)

	// 用于判断是否需要 fallback 到完整响应
	shouldShowFullResponse := len(keys) == 0

	if len(keys) > 0 && len(r.body) > 0 {
		// 传了 keys 且 body 不为空：尝试从 JSON 中提取字段
		result := r.Gson()
		foundAnyKey := false
		var keyInfo strings.Builder

		for _, key := range keys {
			value := result.Get(key)
			if value.Exists() {
				foundAnyKey = true
				keyInfo.WriteString(fmt.Sprintf(", %s=%v", key, value.Value()))
			} else {
				keyInfo.WriteString(fmt.Sprintf(", %s=<not_found>", key))
			}
		}

		// 如果至少找到一个 key，使用提取的信息
		if foundAnyKey {
			contextMsg.WriteString(fmt.Sprintf(" [StatusCode=%d%s]", r.statusCode, keyInfo.String()))
		} else {
			// 所有 keys 都不存在，fallback 到完整响应
			shouldShowFullResponse = true
		}
	} else if len(keys) > 0 && len(r.body) == 0 {
		// 传了 keys 但 body 为空，fallback 到完整响应
		shouldShowFullResponse = true
	}

	// 显示完整响应
	if shouldShowFullResponse {
		contextMsg.WriteString(fmt.Sprintf("\n--- Response Context ---\nStatusCode: %d\n", r.statusCode))

		// Headers
		if len(r.headers) > 0 {
			contextMsg.WriteString("Headers:\n")
			for k, v := range r.headers {
				contextMsg.WriteString(fmt.Sprintf("  %s: %s\n", k, v))
			}
		}

		// Body
		contextMsg.WriteString(fmt.Sprintf("Body (length=%d):\n%s\n", len(r.body), string(r.body)))
		contextMsg.WriteString("--- End Response ---")
	}

	return errBuilder.New(contextMsg.String(), nil).WithTraceID(r.ctx)
}

// EnsureNoError 确保请求无错误
func (r *FasthttpResponse) EnsureNoError() *FasthttpResponse {
	// 如果已经有错误，不再检查
	return r
}

// EnsureStatusCode 确保HTTP状态码等于指定值
// keys: 可选的JSON字段，用于从响应中提取错误上下文
func (r *FasthttpResponse) EnsureStatusCode(code int, keys ...string) *FasthttpResponse {
	if r.err != nil {
		return r
	}

	if r.statusCode != code {
		r.err = r.buildAssertError(
			fmt.Sprintf("期望状态码 %d，实际得到 %d", code, r.statusCode),
			keys,
		)
	}

	return r
}

// EnsureStatus2xx 确保HTTP状态码为2xx
// keys: 可选的JSON字段，用于从响应中提取错误上下文
func (r *FasthttpResponse) EnsureStatus2xx(keys ...string) *FasthttpResponse {
	if r.err != nil {
		return r
	}

	if r.statusCode < 200 || r.statusCode >= 300 {
		r.err = r.buildAssertError(
			fmt.Sprintf("期望2xx状态码，实际得到 %d", r.statusCode),
			keys,
		)
	}

	return r
}

// EnsureContains 确保响应体包含指定字符串
// keys: 可选的JSON字段，用于从响应中提取错误上下文
func (r *FasthttpResponse) EnsureContains(substr string, keys ...string) *FasthttpResponse {
	if r.err != nil {
		return r
	}

	if !strings.Contains(string(r.body), substr) {
		r.err = r.buildAssertError(
			fmt.Sprintf("响应体不包含字符串: %s", substr),
			keys,
		)
	}

	return r
}

// EnsureNotContains 确保响应体不包含指定字符串
// keys: 可选的JSON字段，用于从响应中提取错误上下文
func (r *FasthttpResponse) EnsureNotContains(substr string, keys ...string) *FasthttpResponse {
	if r.err != nil {
		return r
	}

	if strings.Contains(string(r.body), substr) {
		r.err = r.buildAssertError(
			fmt.Sprintf("响应体不应包含字符串: %s", substr),
			keys,
		)
	}

	return r
}

// EnsureJsonStringEq 确保JSON中某个key的string值等于期望值
// keys: 可选的JSON字段，用于从响应中提取错误上下文
func (r *FasthttpResponse) EnsureJsonStringEq(key, expected string, keys ...string) *FasthttpResponse {
	if r.err != nil {
		return r
	}

	result := r.Gson()
	actual := result.Get(key).String()

	if actual != expected {
		r.err = r.buildAssertError(
			fmt.Sprintf("JSON路径 %s 期望值为 %s，实际得到 %s", key, expected, actual),
			keys,
		)
	}

	return r
}

// EnsureJsonStringNe 确保JSON中某个key的string值不等于指定值
// keys: 可选的JSON字段，用于从响应中提取错误上下文
func (r *FasthttpResponse) EnsureJsonStringNe(key, notExpected string, keys ...string) *FasthttpResponse {
	if r.err != nil {
		return r
	}

	result := r.Gson()
	actual := result.Get(key).String()

	if actual == notExpected {
		r.err = r.buildAssertError(
			fmt.Sprintf("JSON路径 %s 的值不应为 %s", key, notExpected),
			keys,
		)
	}

	return r
}

// EnsureJsonExists 确保JSON中某个key存在
// keys: 可选的JSON字段，用于从响应中提取错误上下文
func (r *FasthttpResponse) EnsureJsonExists(key string, keys ...string) *FasthttpResponse {
	if r.err != nil {
		return r
	}

	result := r.Gson()
	if !result.Get(key).Exists() {
		r.err = r.buildAssertError(
			fmt.Sprintf("JSON路径 %s 不存在", key),
			keys,
		)
	}

	return r
}

// EnsureJsonIntEq 确保JSON中某个key的int值等于期望值
// keys: 可选的JSON字段，用于从响应中提取错误上下文
func (r *FasthttpResponse) EnsureJsonIntEq(key string, expected int64, keys ...string) *FasthttpResponse {
	if r.err != nil {
		return r
	}

	result := r.Gson()
	actual := result.Get(key).Int()

	if actual != expected {
		r.err = r.buildAssertError(
			fmt.Sprintf("JSON路径 %s 期望值为 %d，实际得到 %d", key, expected, actual),
			keys,
		)
	}

	return r
}

// EnsureJsonBoolEq 确保JSON中某个key的bool值等于期望值
// keys: 可选的JSON字段，用于从响应中提取错误上下文
func (r *FasthttpResponse) EnsureJsonBoolEq(key string, expected bool, keys ...string) *FasthttpResponse {
	if r.err != nil {
		return r
	}

	result := r.Gson()
	actual := result.Get(key).Bool()

	if actual != expected {
		r.err = r.buildAssertError(
			fmt.Sprintf("JSON路径 %s 期望值为 %v，实际得到 %v", key, expected, actual),
			keys,
		)
	}

	return r
}

package utils

import (
	"context"

	json "github.com/json-iterator/go"
	"github.com/tidwall/gjson"
)

// FasthttpResponse 标准HTTP响应对象
type FasthttpResponse struct {
	err        error             // 请求错误或断言错误
	statusCode int               // HTTP状态码
	body       []byte            // 响应体
	headers    map[string]string // 响应头
	ctx        context.Context   // 上下文（用于traceId等）
}

// newFasthttpResponse 创建响应对象
func newFasthttpResponse(ctx context.Context) *FasthttpResponse {
	return &FasthttpResponse{
		headers: make(map[string]string),
		ctx:     ctx,
	}
}

// Err 返回最终错误（请求错误或断言错误）
func (r *FasthttpResponse) Err() error {
	return r.err
}

// StatusCode 获取HTTP状态码
func (r *FasthttpResponse) StatusCode() int {
	return r.statusCode
}

// Header 获取响应头
func (r *FasthttpResponse) Header(key string) string {
	return r.headers[key]
}

// Headers 获取所有响应头
func (r *FasthttpResponse) Headers() map[string]string {
	return r.headers
}

// Bytes 返回响应体字节数组
func (r *FasthttpResponse) Bytes() ([]byte, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.body, nil
}

// String 返回响应体字符串
func (r *FasthttpResponse) String() (string, error) {
	if r.err != nil {
		return "", r.err
	}
	return string(r.body), nil
}

// Gson 返回gjson.Result用于快速JSON查询
// 注意：此方法不会设置错误到r.err，即使body为空也只返回空Result
// 如需校验JSON格式，请使用断言方法（如EnsureJsonExists）
func (r *FasthttpResponse) Gson() gjson.Result {
	if r.err != nil {
		return gjson.Result{}
	}
	
	if len(r.body) == 0 {
		return gjson.Result{}
	}
	
	return gjson.ParseBytes(r.body)
}

// Bind 将响应体反序列化到结构体
func (r *FasthttpResponse) Bind(v interface{}) error {
	if r.err != nil {
		return r.err
	}
	
	if len(r.body) == 0 {
		return nil
	}
	
	err := json.Unmarshal(r.body, v)
	if err != nil {
		r.err = err
	}
	return r.err
}

// IsOK 检查是否请求成功（无错误）
func (r *FasthttpResponse) IsOK() bool {
	return r.err == nil
}


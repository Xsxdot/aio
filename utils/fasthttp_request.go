package utils

import (
	"context"
	"crypto/tls"

	json "github.com/json-iterator/go"
	"github.com/valyala/fasthttp"
)

// FasthttpRequest HTTP请求构建器
type FasthttpRequest struct {
	url        string
	method     string
	body       []byte
	headers    map[string]string
	cookies    map[string]string
	options    *FasthttpOptions
	ctx        context.Context
	client     *fasthttp.Client
	queryObj   interface{} // GET/HEAD 时用于转 query 参数的对象
	rawQuery   string      // GET/HEAD 时用于追加的原始 query 字符串
	marshalErr error       // JSON 序列化错误
}

// newFasthttpRequest 创建请求构建器
func newFasthttpRequest(method, url string, opts *FasthttpOptions) *FasthttpRequest {
	if opts == nil {
		opts = NewFasthttpOptions()
	}

	return &FasthttpRequest{
		url:     url,
		method:  method,
		headers: make(map[string]string),
		cookies: make(map[string]string),
		options: opts,
		ctx:     context.Background(),
	}
}

// WithContext 设置上下文
func (r *FasthttpRequest) WithContext(ctx context.Context) *FasthttpRequest {
	r.ctx = ctx
	return r
}

// Header 设置单个请求头
func (r *FasthttpRequest) Header(key, value string) *FasthttpRequest {
	r.headers[key] = value
	return r
}

// Headers 批量设置请求头
func (r *FasthttpRequest) Headers(headers map[string]string) *FasthttpRequest {
	for k, v := range headers {
		r.headers[k] = v
	}
	return r
}

// Cookie 设置单个Cookie
func (r *FasthttpRequest) Cookie(key, value string) *FasthttpRequest {
	r.cookies[key] = value
	return r
}

// Cookies 批量设置Cookie
func (r *FasthttpRequest) Cookies(cookies map[string]string) *FasthttpRequest {
	for k, v := range cookies {
		r.cookies[k] = v
	}
	return r
}

// Body 设置请求体（字节数组）
// 对于 GET/HEAD 请求，会将内容作为原始 query 字符串追加到 URL
func (r *FasthttpRequest) Body(body []byte) *FasthttpRequest {
	if isGetOrHead(r.method) {
		r.rawQuery = string(body)
	} else {
		r.body = body
	}
	return r
}

// BodyString 设置请求体（字符串）
// 对于 GET/HEAD 请求，会将内容作为原始 query 字符串追加到 URL
func (r *FasthttpRequest) BodyString(body string) *FasthttpRequest {
	if isGetOrHead(r.method) {
		r.rawQuery = body
	} else {
		r.body = []byte(body)
	}
	return r
}

// JSON 设置请求体为JSON（自动序列化对象）
// 对于 GET/HEAD 请求，会将对象转换为 query 参数追加到 URL
// 对于其他请求，会序列化为 JSON 并设置为请求体
func (r *FasthttpRequest) JSON(obj interface{}) *FasthttpRequest {
	if isGetOrHead(r.method) {
		// GET/HEAD 请求：保存对象，稍后转为 query 参数
		r.queryObj = obj
	} else {
		// 其他请求：序列化为 JSON
		data, err := json.Marshal(obj)
		if err != nil {
			r.marshalErr = err
			return r
		}
		r.body = data
		// 自动设置Content-Type
		r.headers["Content-Type"] = "application/json"
	}
	return r
}

// Do 执行HTTP请求
func (r *FasthttpRequest) Do() *FasthttpResponse {
	resp := newFasthttpResponse(r.ctx)

	// 检查 JSON 序列化错误
	if r.marshalErr != nil {
		resp.err = errBuilder.New("JSON序列化失败", r.marshalErr).WithTraceID(r.ctx)
		return resp
	}

	// 处理 GET/HEAD 请求的 query 参数
	if isGetOrHead(r.method) && (r.queryObj != nil || r.rawQuery != "") {
		newURL, err := mergeQueryIntoURL(r.url, r.rawQuery, r.queryObj)
		if err != nil {
			resp.err = errBuilder.New("合并query参数失败", err).WithTraceID(r.ctx)
			return resp
		}
		r.url = newURL
	}

	// 获取或创建client
	client := r.client
	if client == nil {
		var err error
		client, err = r.buildClient()
		if err != nil {
			resp.err = errBuilder.New("创建HTTP客户端失败", err).WithTraceID(r.ctx)
			return resp
		}
	}

	// 创建fasthttp请求和响应对象
	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)

	fasthttpResp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(fasthttpResp)

	// 设置请求方法和URL
	req.Header.SetMethod(r.method)
	req.SetRequestURI(r.url)

	// 设置请求头（先从options，再从request）
	if r.options != nil && r.options.Headers != nil {
		for k, v := range r.options.Headers {
			req.Header.Set(k, v)
		}
	}
	for k, v := range r.headers {
		req.Header.Set(k, v)
	}

	// 设置Cookie
	if r.options != nil && r.options.Cookies != nil {
		for k, v := range r.options.Cookies {
			req.Header.SetCookie(k, v)
		}
	}
	for k, v := range r.cookies {
		req.Header.SetCookie(k, v)
	}

	// 设置请求体（GET/HEAD 不设置 body）
	if !isGetOrHead(r.method) && len(r.body) > 0 {
		req.SetBody(r.body)
	}

	// 执行请求
	var err error
	if r.options != nil && r.options.Timeout > 0 {
		err = client.DoTimeout(req, fasthttpResp, r.options.Timeout)
	} else {
		err = client.Do(req, fasthttpResp)
	}

	// 按照plan：只要err == nil就认为请求成功，更深层校验由断言链处理
	if err != nil {
		resp.err = errBuilder.New("HTTP请求失败", err).WithTraceID(r.ctx)
		return resp
	}

	// 保存响应信息
	resp.statusCode = fasthttpResp.StatusCode()
	resp.body = make([]byte, len(fasthttpResp.Body()))
	copy(resp.body, fasthttpResp.Body())

	// 保存响应头
	fasthttpResp.Header.VisitAll(func(key, value []byte) {
		resp.headers[string(key)] = string(value)
	})

	// 自动执行 options 中配置的断言（fail-fast）
	if r.options != nil && len(r.options.Asserts) > 0 {
		for _, assertFn := range r.options.Asserts {
			if assertFn == nil {
				continue
			}
			assertFn(resp)
			// fail-fast: 一旦有错误就停止后续断言
			if resp.err != nil {
				break
			}
		}
	}

	return resp
}

// buildClient 根据配置构建fasthttp.Client
func (r *FasthttpRequest) buildClient() (*fasthttp.Client, error) {
	client := &fasthttp.Client{}

	// 配置TLS
	if r.options != nil && r.options.TLS != nil {
		client.TLSConfig = &tls.Config{
			InsecureSkipVerify: r.options.TLS.InsecureSkipVerify,
		}
	}

	// 配置代理
	if r.options != nil && r.options.Proxy != nil {
		dialer, err := buildProxyDialer(r.options.Proxy)
		if err != nil {
			return nil, err
		}
		client.Dial = dialer
	}

	return client, nil
}

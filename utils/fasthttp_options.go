package utils

import (
	"time"
)

// FasthttpAssertFunc 响应断言函数类型
type FasthttpAssertFunc func(*FasthttpResponse)

// FasthttpOptions 可复用的HTTP请求配置对象
type FasthttpOptions struct {
	Timeout time.Duration        // 请求超时时间
	Headers map[string]string    // 默认请求头
	Cookies map[string]string    // 默认Cookie
	Proxy   *ProxyConfig         // 代理配置
	TLS     *TLSConfig           // TLS配置
	Asserts []FasthttpAssertFunc // 响应断言列表（在Do()返回前自动执行）
}

// ProxyConfig 代理配置
// 支持格式：
// - http://host:port
// - http://user:pass@host:port
// - https://user:pass@host:port
// - socks5://user:pass@host:port
type ProxyConfig struct {
	URL      string // 代理URL
	Username string // 用户名（可选，也可包含在URL中）
	Password string // 密码（可选，也可包含在URL中）
}

// TLSConfig TLS配置
type TLSConfig struct {
	InsecureSkipVerify bool // 是否跳过证书验证
}

// NewFasthttpOptions 创建默认配置
func NewFasthttpOptions() *FasthttpOptions {
	return &FasthttpOptions{
		Timeout: 10 * time.Second,
		Headers: make(map[string]string),
		Cookies: make(map[string]string),
	}
}

// WithTimeout 设置超时时间
func (o *FasthttpOptions) WithTimeout(timeout time.Duration) *FasthttpOptions {
	o.Timeout = timeout
	return o
}

// WithHeader 添加请求头
func (o *FasthttpOptions) WithHeader(key, value string) *FasthttpOptions {
	if o.Headers == nil {
		o.Headers = make(map[string]string)
	}
	o.Headers[key] = value
	return o
}

// WithHeaders 批量设置请求头
func (o *FasthttpOptions) WithHeaders(headers map[string]string) *FasthttpOptions {
	if o.Headers == nil {
		o.Headers = make(map[string]string)
	}
	for k, v := range headers {
		o.Headers[k] = v
	}
	return o
}

// WithCookie 添加Cookie
func (o *FasthttpOptions) WithCookie(key, value string) *FasthttpOptions {
	if o.Cookies == nil {
		o.Cookies = make(map[string]string)
	}
	o.Cookies[key] = value
	return o
}

// WithProxy 设置代理
func (o *FasthttpOptions) WithProxy(proxyURL string) *FasthttpOptions {
	o.Proxy = &ProxyConfig{URL: proxyURL}
	return o
}

// WithProxyAuth 设置带认证的代理
func (o *FasthttpOptions) WithProxyAuth(proxyURL, username, password string) *FasthttpOptions {
	o.Proxy = &ProxyConfig{
		URL:      proxyURL,
		Username: username,
		Password: password,
	}
	return o
}

// WithInsecureSkipVerify 设置是否跳过TLS证书验证
func (o *FasthttpOptions) WithInsecureSkipVerify(skip bool) *FasthttpOptions {
	if o.TLS == nil {
		o.TLS = &TLSConfig{}
	}
	o.TLS.InsecureSkipVerify = skip
	return o
}

// WithAssert 添加单个断言函数
func (o *FasthttpOptions) WithAssert(fn FasthttpAssertFunc) *FasthttpOptions {
	if o.Asserts == nil {
		o.Asserts = make([]FasthttpAssertFunc, 0)
	}
	o.Asserts = append(o.Asserts, fn)
	return o
}

// WithAsserts 批量添加断言函数
func (o *FasthttpOptions) WithAsserts(fns ...FasthttpAssertFunc) *FasthttpOptions {
	if o.Asserts == nil {
		o.Asserts = make([]FasthttpAssertFunc, 0, len(fns))
	}
	o.Asserts = append(o.Asserts, fns...)
	return o
}

// WithEnsureStatusCode 添加状态码断言
// keys: 可选的JSON字段，用于从响应中提取错误上下文
func (o *FasthttpOptions) WithEnsureStatusCode(code int, keys ...string) *FasthttpOptions {
	return o.WithAssert(func(r *FasthttpResponse) {
		r.EnsureStatusCode(code, keys...)
	})
}

// WithEnsureStatus2xx 添加2xx状态码断言
// keys: 可选的JSON字段，用于从响应中提取错误上下文
func (o *FasthttpOptions) WithEnsureStatus2xx(keys ...string) *FasthttpOptions {
	return o.WithAssert(func(r *FasthttpResponse) {
		r.EnsureStatus2xx(keys...)
	})
}

// WithEnsureContains 添加响应体包含字符串断言
// keys: 可选的JSON字段，用于从响应中提取错误上下文
func (o *FasthttpOptions) WithEnsureContains(substr string, keys ...string) *FasthttpOptions {
	return o.WithAssert(func(r *FasthttpResponse) {
		r.EnsureContains(substr, keys...)
	})
}

// WithEnsureNotContains 添加响应体不包含字符串断言
// keys: 可选的JSON字段，用于从响应中提取错误上下文
func (o *FasthttpOptions) WithEnsureNotContains(substr string, keys ...string) *FasthttpOptions {
	return o.WithAssert(func(r *FasthttpResponse) {
		r.EnsureNotContains(substr, keys...)
	})
}

// WithEnsureJsonStringEq 添加JSON字符串字段等于断言
// keys: 可选的JSON字段，用于从响应中提取错误上下文
func (o *FasthttpOptions) WithEnsureJsonStringEq(key, expected string, keys ...string) *FasthttpOptions {
	return o.WithAssert(func(r *FasthttpResponse) {
		r.EnsureJsonStringEq(key, expected, keys...)
	})
}

// WithEnsureJsonStringNe 添加JSON字符串字段不等于断言
// keys: 可选的JSON字段，用于从响应中提取错误上下文
func (o *FasthttpOptions) WithEnsureJsonStringNe(key, notExpected string, keys ...string) *FasthttpOptions {
	return o.WithAssert(func(r *FasthttpResponse) {
		r.EnsureJsonStringNe(key, notExpected, keys...)
	})
}

// WithEnsureJsonExists 添加JSON路径存在断言
// keys: 可选的JSON字段，用于从响应中提取错误上下文
func (o *FasthttpOptions) WithEnsureJsonExists(key string, keys ...string) *FasthttpOptions {
	return o.WithAssert(func(r *FasthttpResponse) {
		r.EnsureJsonExists(key, keys...)
	})
}

// WithEnsureJsonIntEq 添加JSON整数字段等于断言
// keys: 可选的JSON字段，用于从响应中提取错误上下文
func (o *FasthttpOptions) WithEnsureJsonIntEq(key string, expected int64, keys ...string) *FasthttpOptions {
	return o.WithAssert(func(r *FasthttpResponse) {
		r.EnsureJsonIntEq(key, expected, keys...)
	})
}

// WithEnsureJsonBoolEq 添加JSON布尔字段等于断言
// keys: 可选的JSON字段，用于从响应中提取错误上下文
func (o *FasthttpOptions) WithEnsureJsonBoolEq(key string, expected bool, keys ...string) *FasthttpOptions {
	return o.WithAssert(func(r *FasthttpResponse) {
		r.EnsureJsonBoolEq(key, expected, keys...)
	})
}

// Clone 克隆配置对象
func (o *FasthttpOptions) Clone() *FasthttpOptions {
	if o == nil {
		return NewFasthttpOptions()
	}

	clone := &FasthttpOptions{
		Timeout: o.Timeout,
		Headers: make(map[string]string),
		Cookies: make(map[string]string),
	}

	for k, v := range o.Headers {
		clone.Headers[k] = v
	}

	for k, v := range o.Cookies {
		clone.Cookies[k] = v
	}

	if o.Proxy != nil {
		clone.Proxy = &ProxyConfig{
			URL:      o.Proxy.URL,
			Username: o.Proxy.Username,
			Password: o.Proxy.Password,
		}
	}

	if o.TLS != nil {
		clone.TLS = &TLSConfig{
			InsecureSkipVerify: o.TLS.InsecureSkipVerify,
		}
	}

	// 深拷贝 Asserts slice
	if o.Asserts != nil {
		clone.Asserts = make([]FasthttpAssertFunc, len(o.Asserts))
		copy(clone.Asserts, o.Asserts)
	}

	return clone
}

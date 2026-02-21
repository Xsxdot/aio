package utils

// FasthttpClient HTTP客户端
type FasthttpClient struct {
	options *FasthttpOptions
}

// NewFasthttpClient 创建HTTP客户端
func NewFasthttpClient(opts *FasthttpOptions) *FasthttpClient {
	if opts == nil {
		opts = NewFasthttpOptions()
	}
	return &FasthttpClient{options: opts}
}

// Get 创建GET请求
func (c *FasthttpClient) Get(url string) *FasthttpRequest {
	return newFasthttpRequest("GET", url, c.options)
}

// Post 创建POST请求
func (c *FasthttpClient) Post(url string) *FasthttpRequest {
	return newFasthttpRequest("POST", url, c.options)
}

// Put 创建PUT请求
func (c *FasthttpClient) Put(url string) *FasthttpRequest {
	return newFasthttpRequest("PUT", url, c.options)
}

// Delete 创建DELETE请求
func (c *FasthttpClient) Delete(url string) *FasthttpRequest {
	return newFasthttpRequest("DELETE", url, c.options)
}

// Patch 创建PATCH请求
func (c *FasthttpClient) Patch(url string) *FasthttpRequest {
	return newFasthttpRequest("PATCH", url, c.options)
}

// Head 创建HEAD请求
func (c *FasthttpClient) Head(url string) *FasthttpRequest {
	return newFasthttpRequest("HEAD", url, c.options)
}

// Options 创建OPTIONS请求
func (c *FasthttpClient) Options(url string) *FasthttpRequest {
	return newFasthttpRequest("OPTIONS", url, c.options)
}

// 全局默认客户端
var defaultClient = NewFasthttpClient(nil)

// HttpGet 快速GET请求（使用默认配置）
func HttpGet(url string) *FasthttpRequest {
	return defaultClient.Get(url)
}

// HttpPost 快速POST请求（使用默认配置）
func HttpPost(url string) *FasthttpRequest {
	return defaultClient.Post(url)
}

// HttpPostJSON 快速POST JSON请求（使用默认配置）
func HttpPostJSON(url string, obj interface{}) *FasthttpRequest {
	return defaultClient.Post(url).JSON(obj)
}

// HttpPut 快速PUT请求（使用默认配置）
func HttpPut(url string) *FasthttpRequest {
	return defaultClient.Put(url)
}

// HttpDelete 快速DELETE请求（使用默认配置）
func HttpDelete(url string) *FasthttpRequest {
	return defaultClient.Delete(url)
}

// SetDefaultOptions 设置全局默认配置
func SetDefaultOptions(opts *FasthttpOptions) {
	if opts != nil {
		defaultClient = NewFasthttpClient(opts)
	}
}

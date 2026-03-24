package utils

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/proxy"
)

// BuildHTTPClientWithProxy 构建带代理的 HTTP 客户端
// 支持 http(s):// 和 socks5:// 代理
//
// 参数:
//   - proxyURL: 代理地址，支持 http://、https://、socks5:// 协议，空字符串表示不使用代理
//   - timeout: 客户端超时时间，0 表示使用默认值 60 秒
//
// 返回:
//   - *http.Client: 配置好的 HTTP 客户端
//   - error: 代理 URL 解析错误或不支持的协议
func BuildHTTPClientWithProxy(proxyURL string, timeout time.Duration) (*http.Client, error) {
	if timeout == 0 {
		timeout = 60 * time.Second
	}

	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
	}

	// 如果没有代理配置，直接返回
	if proxyURL == "" {
		return &http.Client{
			Transport: transport,
			Timeout:   timeout,
		}, nil
	}

	// 解析代理 URL
	parsedProxy, err := url.Parse(proxyURL)
	if err != nil {
		return nil, fmt.Errorf("invalid proxy URL: %w", err)
	}

	// 根据协议类型设置代理
	switch strings.ToLower(parsedProxy.Scheme) {
	case "http", "https":
		// HTTP/HTTPS 代理
		transport.Proxy = http.ProxyURL(parsedProxy)

	case "socks5":
		// SOCKS5 代理
		var auth *proxy.Auth
		if parsedProxy.User != nil {
			password, _ := parsedProxy.User.Password()
			auth = &proxy.Auth{
				User:     parsedProxy.User.Username(),
				Password: password,
			}
		}

		dialer, err := proxy.SOCKS5("tcp", parsedProxy.Host, auth, proxy.Direct)
		if err != nil {
			return nil, fmt.Errorf("failed to create SOCKS5 dialer: %w", err)
		}

		// 使用 DialContext 替代已废弃的 Dial（Go 1.24+）
		transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialer.Dial(network, addr)
		}

	default:
		return nil, fmt.Errorf("unsupported proxy scheme: %s (supported: http, https, socks5)", parsedProxy.Scheme)
	}

	return &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}, nil
}

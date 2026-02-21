package utils

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttpproxy"
	"golang.org/x/net/proxy"
)

// buildProxyDialer 根据代理配置构建dialer
func buildProxyDialer(proxyConfig *ProxyConfig) (fasthttp.DialFunc, error) {
	if proxyConfig == nil || proxyConfig.URL == "" {
		return fasthttp.Dial, nil
	}

	// 解析代理URL
	proxyURL, err := url.Parse(proxyConfig.URL)
	if err != nil {
		return nil, fmt.Errorf("解析代理URL失败: %w", err)
	}

	// 从URL中提取用户名密码（如果有）
	username := proxyConfig.Username
	password := proxyConfig.Password
	if proxyURL.User != nil {
		if username == "" {
			username = proxyURL.User.Username()
		}
		if password == "" {
			password, _ = proxyURL.User.Password()
		}
	}

	scheme := strings.ToLower(proxyURL.Scheme)

	switch scheme {
	case "http", "https":
		return buildHTTPProxyDialer(proxyURL, username, password)
	case "socks5":
		return buildSOCKS5ProxyDialer(proxyURL, username, password)
	default:
		return nil, fmt.Errorf("不支持的代理协议: %s", scheme)
	}
}

// buildHTTPProxyDialer 构建HTTP/HTTPS代理dialer
func buildHTTPProxyDialer(proxyURL *url.URL, username, password string) (fasthttp.DialFunc, error) {
	// 构建代理地址（不包含scheme）
	proxyAddr := proxyURL.Host

	// 如果有认证信息，使用fasthttpproxy的HTTPDialer
	if username != "" || password != "" {
		// 重新构建包含认证信息的URL
		authProxyURL := &url.URL{
			Scheme: proxyURL.Scheme,
			Host:   proxyURL.Host,
		}
		if username != "" || password != "" {
			authProxyURL.User = url.UserPassword(username, password)
		}

		return fasthttpproxy.FasthttpHTTPDialer(authProxyURL.String()), nil
	}

	// 无认证的HTTP代理
	return fasthttpproxy.FasthttpHTTPDialer(proxyAddr), nil
}

// buildSOCKS5ProxyDialer 构建SOCKS5代理dialer
func buildSOCKS5ProxyDialer(proxyURL *url.URL, username, password string) (fasthttp.DialFunc, error) {
	proxyAddr := proxyURL.Host

	// 如果有认证信息，使用golang.org/x/net/proxy包（支持auth）
	if username != "" || password != "" {
		auth := &proxy.Auth{
			User:     username,
			Password: password,
		}

		// 创建SOCKS5 dialer
		dialer, err := proxy.SOCKS5("tcp", proxyAddr, auth, &net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		})
		if err != nil {
			return nil, fmt.Errorf("创建SOCKS5代理失败: %w", err)
		}

		// 包装为fasthttp.DialFunc
		return func(addr string) (net.Conn, error) {
			return dialer.Dial("tcp", addr)
		}, nil
	}

	// 无认证的SOCKS5代理，尝试使用fasthttpproxy
	// fasthttpproxy.FasthttpSocksDialer可能不支持auth，所以有auth时用上面的方式
	return fasthttpproxy.FasthttpSocksDialer(proxyAddr), nil
}

// buildTLSProxyDialer 构建支持TLS的代理dialer（用于HTTPS代理）
func buildTLSProxyDialer(proxyURL *url.URL, username, password string, tlsConfig *tls.Config) (fasthttp.DialFunc, error) {
	baseDialer, err := buildHTTPProxyDialer(proxyURL, username, password)
	if err != nil {
		return nil, err
	}

	// 包装TLS
	return func(addr string) (net.Conn, error) {
		conn, err := baseDialer(addr)
		if err != nil {
			return nil, err
		}

		// 如果目标地址需要TLS，进行TLS握手
		if tlsConfig != nil {
			host, _, _ := net.SplitHostPort(addr)
			tlsConn := tls.Client(conn, &tls.Config{
				ServerName:         host,
				InsecureSkipVerify: tlsConfig.InsecureSkipVerify,
			})

			if err := tlsConn.Handshake(); err != nil {
				conn.Close()
				return nil, err
			}

			return tlsConn, nil
		}

		return conn, nil
	}, nil
}

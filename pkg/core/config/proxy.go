package config

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"golang.org/x/net/proxy"
)

// ProxyConfig SOCKS代理配置结构体
type ProxyConfig struct {
	Enabled  bool   `yaml:"enabled" json:"enabled"`   // 是否启用代理
	Host     string `yaml:"host" json:"host"`         // 代理服务器地址
	Port     int    `yaml:"port" json:"port"`         // 代理服务器端口
	Username string `yaml:"username" json:"username"` // 代理认证用户名（可选）
	Password string `yaml:"password" json:"password"` // 代理认证密码（可选）
}

// GetDialer 获取配置好的SOCKS5 dialer
// 如果代理未启用，返回默认的net.Dialer
func (p ProxyConfig) GetDialer() proxy.Dialer {
	if !p.Enabled {
		return &net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}
	}

	address := fmt.Sprintf("%s:%d", p.Host, p.Port)

	var auth *proxy.Auth
	if p.Username != "" && p.Password != "" {
		auth = &proxy.Auth{
			User:     p.Username,
			Password: p.Password,
		}
	}

	dialer, err := proxy.SOCKS5("tcp", address, auth, proxy.Direct)
	if err != nil {
		// 如果创建代理失败，返回默认dialer
		return &net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}
	}

	return dialer
}

// GetContextDialer 获取支持context的dialer函数
func (p ProxyConfig) GetContextDialer() func(ctx context.Context, network, address string) (net.Conn, error) {
	dialer := p.GetDialer()

	return func(ctx context.Context, network, address string) (net.Conn, error) {
		return dialer.Dial(network, address)
	}
}

// GetHTTPTransport 获取配置了代理的HTTP Transport
func (p ProxyConfig) GetHTTPTransport() *http.Transport {
	if !p.Enabled {
		return &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		}
	}

	dialer := p.GetDialer()

	return &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialer.Dial(network, addr)
		},
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
}

// GetHTTPClient 获取配置了代理的HTTP客户端
func (p ProxyConfig) GetHTTPClient() *http.Client {
	return &http.Client{
		Transport: p.GetHTTPTransport(),
		Timeout:   30 * time.Second,
	}
}

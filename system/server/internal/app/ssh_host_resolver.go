package app

import (
	"context"
	"fmt"
	"net"
	"time"
	"github.com/xsxdot/aio/system/server/internal/model"
)

// DialContextFunc 用于 TCP 连接探测的函数类型（便于单元测试注入）
type DialContextFunc func(ctx context.Context, network, address string) (net.Conn, error)

// defaultDialContext 默认的 TCP 连接探测函数
var defaultDialContext DialContextFunc = func(ctx context.Context, network, address string) (net.Conn, error) {
	dialer := &net.Dialer{
		Timeout: 800 * time.Millisecond, // 固定超时 800ms
	}
	return dialer.DialContext(ctx, network, address)
}

// ResolveSSHHost 解析服务器的 SSH 连接地址
// 优先选择内网地址（如果可达），否则回退到外网地址
// 返回：已验证可达的 host 地址
func (a *App) ResolveSSHHost(ctx context.Context, server *model.ServerModel, port int) (string, error) {
	// 1. 如果内网地址非空，尝试探测内网可达性
	if server.IntranetHost != "" {
		address := fmt.Sprintf("%s:%d", server.IntranetHost, port)
		if a.isHostReachable(ctx, address) {
			a.log.WithFields(map[string]interface{}{
				"server_id":     server.ID,
				"server_name":   server.Name,
				"intranet_host": server.IntranetHost,
				"port":          port,
			}).Info("内网地址可达，使用内网连接")
			return server.IntranetHost, nil
		}
		a.log.WithFields(map[string]interface{}{
			"server_id":     server.ID,
			"server_name":   server.Name,
			"intranet_host": server.IntranetHost,
			"port":          port,
		}).Warn("内网地址不可达，回退到外网地址")
	}

	// 2. 回退到外网地址（优先使用 ExtranetHost，否则使用兼容字段 Host）
	extranetHost := server.ExtranetHost
	if extranetHost == "" {
		extranetHost = server.Host
	}

	if extranetHost == "" {
		return "", a.errBuilder.New("服务器未配置有效的连接地址", nil).ValidWithCtx()
	}

	a.log.WithFields(map[string]interface{}{
		"server_id":     server.ID,
		"server_name":   server.Name,
		"extranet_host": extranetHost,
	}).Info("使用外网地址连接")

	return extranetHost, nil
}

// isHostReachable 检查指定地址是否可达（TCP 探测）
func (a *App) isHostReachable(ctx context.Context, address string) bool {
	// 使用可注入的 dial 函数（便于单元测试）
	dialFunc := a.getDialContext()

	conn, err := dialFunc(ctx, "tcp", address)
	if err != nil {
		return false
	}

	// 连接成功，立即关闭
	_ = conn.Close()
	return true
}

// getDialContext 获取 dial 函数（优先使用注入的，否则使用默认）
func (a *App) getDialContext() DialContextFunc {
	if a.dialContext != nil {
		return a.dialContext
	}
	return defaultDialContext
}

// SetDialContext 设置自定义 dial 函数（用于单元测试）
func (a *App) SetDialContext(dialFunc DialContextFunc) {
	a.dialContext = dialFunc
}

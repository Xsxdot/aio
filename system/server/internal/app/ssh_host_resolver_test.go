package app

import (
	"context"
	"errors"
	"net"
	"testing"
	errorc "github.com/xsxdot/aio/pkg/core/err"
	"github.com/xsxdot/aio/pkg/core/logger"
	"github.com/xsxdot/aio/system/server/internal/model"

	"github.com/stretchr/testify/assert"
)

// mockConn 模拟的网络连接
type mockConn struct {
	net.Conn
}

func (m *mockConn) Close() error {
	return nil
}

// TestResolveSSHHost_IntranetReachable 测试内网可达时返回内网地址
func TestResolveSSHHost_IntranetReachable(t *testing.T) {
	app := &App{
		log:        logger.GetLogger().WithEntryName("TestApp"),
		errBuilder: errorc.NewErrorBuilder("TestApp"),
	}

	// 注入模拟 dial 函数：内网可达
	app.SetDialContext(func(ctx context.Context, network, address string) (net.Conn, error) {
		if address == "192.168.1.100:22" {
			return &mockConn{}, nil
		}
		return nil, errors.New("connection refused")
	})

	server := &model.ServerModel{
		Name:         "test-server",
		IntranetHost: "192.168.1.100",
		ExtranetHost: "1.2.3.4",
		Host:         "1.2.3.4",
	}
	server.ID = 1

	ctx := context.Background()
	host, err := app.ResolveSSHHost(ctx, server, 22)

	assert.NoError(t, err)
	assert.Equal(t, "192.168.1.100", host, "应返回内网地址")
}

// TestResolveSSHHost_IntranetUnreachable 测试内网不可达时回退到外网地址
func TestResolveSSHHost_IntranetUnreachable(t *testing.T) {
	app := &App{
		log:        logger.GetLogger().WithEntryName("TestApp"),
		errBuilder: errorc.NewErrorBuilder("TestApp"),
	}

	// 注入模拟 dial 函数：内网不可达
	app.SetDialContext(func(ctx context.Context, network, address string) (net.Conn, error) {
		return nil, errors.New("connection timeout")
	})

	server := &model.ServerModel{
		Name:         "test-server",
		IntranetHost: "192.168.1.100",
		ExtranetHost: "1.2.3.4",
		Host:         "1.2.3.4",
	}
	server.ID = 1

	ctx := context.Background()
	host, err := app.ResolveSSHHost(ctx, server, 22)

	assert.NoError(t, err)
	assert.Equal(t, "1.2.3.4", host, "应回退到外网地址")
}

// TestResolveSSHHost_NoIntranetHost 测试没有内网地址时直接返回外网地址
func TestResolveSSHHost_NoIntranetHost(t *testing.T) {
	app := &App{
		log:        logger.GetLogger().WithEntryName("TestApp"),
		errBuilder: errorc.NewErrorBuilder("TestApp"),
	}

	// 注入模拟 dial 函数（不应被调用）
	app.SetDialContext(func(ctx context.Context, network, address string) (net.Conn, error) {
		t.Fatal("不应调用 dial 函数")
		return nil, errors.New("should not be called")
	})

	server := &model.ServerModel{
		Name:         "test-server",
		IntranetHost: "", // 没有内网地址
		ExtranetHost: "1.2.3.4",
		Host:         "1.2.3.4",
	}
	server.ID = 1

	ctx := context.Background()
	host, err := app.ResolveSSHHost(ctx, server, 22)

	assert.NoError(t, err)
	assert.Equal(t, "1.2.3.4", host, "应直接返回外网地址")
}

// TestResolveSSHHost_FallbackToHost 测试 ExtranetHost 为空时回退到 Host 字段
func TestResolveSSHHost_FallbackToHost(t *testing.T) {
	app := &App{
		log:        logger.GetLogger().WithEntryName("TestApp"),
		errBuilder: errorc.NewErrorBuilder("TestApp"),
	}

	// 注入模拟 dial 函数：内网不可达
	app.SetDialContext(func(ctx context.Context, network, address string) (net.Conn, error) {
		return nil, errors.New("connection timeout")
	})

	server := &model.ServerModel{
		Name:         "test-server",
		IntranetHost: "192.168.1.100",
		ExtranetHost: "",        // 外网地址为空
		Host:         "5.6.7.8", // 兼容字段
	}
	server.ID = 1

	ctx := context.Background()
	host, err := app.ResolveSSHHost(ctx, server, 22)

	assert.NoError(t, err)
	assert.Equal(t, "5.6.7.8", host, "应回退到 Host 兼容字段")
}

// TestResolveSSHHost_NoValidHost 测试没有任何有效地址时返回错误
func TestResolveSSHHost_NoValidHost(t *testing.T) {
	app := &App{
		log:        logger.GetLogger().WithEntryName("TestApp"),
		errBuilder: errorc.NewErrorBuilder("TestApp"),
	}

	// 注入模拟 dial 函数：内网不可达
	app.SetDialContext(func(ctx context.Context, network, address string) (net.Conn, error) {
		return nil, errors.New("connection timeout")
	})

	server := &model.ServerModel{
		Name:         "test-server",
		IntranetHost: "192.168.1.100",
		ExtranetHost: "",
		Host:         "", // 所有地址都为空
	}
	server.ID = 1

	ctx := context.Background()
	_, err := app.ResolveSSHHost(ctx, server, 22)

	assert.Error(t, err, "应返回错误")
}

// TestResolveSSHHost_ContextCancellation 测试上下文取消时的行为
func TestResolveSSHHost_ContextCancellation(t *testing.T) {
	app := &App{
		log:        logger.GetLogger().WithEntryName("TestApp"),
		errBuilder: errorc.NewErrorBuilder("TestApp"),
	}

	// 注入模拟 dial 函数：返回上下文取消错误
	app.SetDialContext(func(ctx context.Context, network, address string) (net.Conn, error) {
		return nil, context.Canceled
	})

	server := &model.ServerModel{
		Name:         "test-server",
		IntranetHost: "192.168.1.100",
		ExtranetHost: "1.2.3.4",
		Host:         "1.2.3.4",
	}
	server.ID = 1

	ctx := context.Background()
	host, err := app.ResolveSSHHost(ctx, server, 22)

	// 内网不可达，应回退到外网
	assert.NoError(t, err)
	assert.Equal(t, "1.2.3.4", host, "上下文取消时应回退到外网地址")
}

// TestResolveSSHHost_DifferentPorts 测试不同端口的探测
func TestResolveSSHHost_DifferentPorts(t *testing.T) {
	app := &App{
		log:        logger.GetLogger().WithEntryName("TestApp"),
		errBuilder: errorc.NewErrorBuilder("TestApp"),
	}

	// 注入模拟 dial 函数：只有端口 2222 可达
	app.SetDialContext(func(ctx context.Context, network, address string) (net.Conn, error) {
		if address == "192.168.1.100:2222" {
			return &mockConn{}, nil
		}
		return nil, errors.New("connection refused")
	})

	server := &model.ServerModel{
		Name:         "test-server",
		IntranetHost: "192.168.1.100",
		ExtranetHost: "1.2.3.4",
		Host:         "1.2.3.4",
	}
	server.ID = 1

	ctx := context.Background()

	// 测试端口 22（不可达）
	host, err := app.ResolveSSHHost(ctx, server, 22)
	assert.NoError(t, err)
	assert.Equal(t, "1.2.3.4", host, "端口 22 不可达，应回退到外网")

	// 测试端口 2222（可达）
	host, err = app.ResolveSSHHost(ctx, server, 2222)
	assert.NoError(t, err)
	assert.Equal(t, "192.168.1.100", host, "端口 2222 可达，应返回内网")
}

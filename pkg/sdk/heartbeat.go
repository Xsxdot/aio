package sdk

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	registrypb "xiaozhizhang/system/registry/api/proto"
)

// RegistrationHandle 注册句柄
type RegistrationHandle struct {
	client      *Client
	serviceID   int64
	InstanceKey string // 导出以便外部访问
	ttlSeconds  int64

	// 心跳控制
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	stopped bool
	stopMu  sync.Mutex
}

// RegisterSelf 注册自身到注册中心
func (rc *RegistryClient) RegisterSelf(ctx context.Context, req *RegisterInstanceRequest) (*RegistrationHandle, error) {
	// 注册实例
	resp, err := rc.RegisterInstance(ctx, req)
	if err != nil {
		return nil, err
	}

	// 创建注册句柄
	handle := &RegistrationHandle{
		client:      rc.client,
		serviceID:   req.ServiceID,
		InstanceKey: resp.InstanceKey,
		ttlSeconds:  req.TTLSeconds,
	}
	handle.ctx, handle.cancel = context.WithCancel(context.Background())

	// 启动心跳 goroutine
	handle.wg.Add(1)
	go handle.heartbeatLoop()

	return handle, nil
}

// Stop 停止心跳并注销实例
func (h *RegistrationHandle) Stop() error {
	h.stopMu.Lock()
	if h.stopped {
		h.stopMu.Unlock()
		return nil
	}
	h.stopped = true
	h.stopMu.Unlock()

	// 取消心跳
	h.cancel()

	// 等待心跳 goroutine 退出
	h.wg.Wait()

	// 注销实例
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := h.client.Registry.DeregisterInstance(ctx, h.serviceID, h.InstanceKey)
	if err != nil {
		return fmt.Errorf("failed to deregister: %w", err)
	}

	return nil
}

// heartbeatLoop 心跳循环
func (h *RegistrationHandle) heartbeatLoop() {
	defer h.wg.Done()

	// 计算心跳间隔（TTL 的 1/3）
	interval := time.Duration(h.ttlSeconds) * time.Second / 3
	if interval < 10*time.Second {
		interval = 10 * time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// 重连 backoff
	retryDelay := time.Second
	maxRetryDelay := 30 * time.Second

	for {
		select {
		case <-h.ctx.Done():
			return
		case <-ticker.C:
			err := h.sendHeartbeatStream()
			if err != nil {
				// 心跳失败，等待后重试
				fmt.Printf("Heartbeat failed: %v, will retry in %v\n", err, retryDelay)
				time.Sleep(retryDelay)

				// 指数退避
				retryDelay *= 2
				if retryDelay > maxRetryDelay {
					retryDelay = maxRetryDelay
				}
			} else {
				// 成功，重置退避
				retryDelay = time.Second
			}
		}
	}
}

// sendHeartbeatStream 发送心跳流
func (h *RegistrationHandle) sendHeartbeatStream() error {
	ctx, cancel := context.WithTimeout(h.ctx, 10*time.Second)
	defer cancel()

	// 创建心跳流
	stream, err := h.client.Registry.service.HeartbeatStream(ctx)
	if err != nil {
		return fmt.Errorf("failed to create heartbeat stream: %w", err)
	}

	// 发送心跳请求
	req := &registrypb.HeartbeatRequest{
		ServiceId:   h.serviceID,
		InstanceKey: h.InstanceKey,
	}

	err = stream.Send(req)
	if err != nil {
		stream.CloseSend()
		return fmt.Errorf("failed to send heartbeat: %w", err)
	}

	// 接收响应
	resp, err := stream.Recv()
	if err != nil {
		if err == io.EOF {
			return nil
		}
		return fmt.Errorf("failed to receive heartbeat response: %w", err)
	}

	// 关闭发送端
	stream.CloseSend()

	// 可选：记录新的过期时间
	_ = resp.ExpiresAt

	return nil
}

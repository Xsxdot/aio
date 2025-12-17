package sdk

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// unaryAuthInterceptor 一元 RPC 鉴权拦截器
func (c *Client) unaryAuthInterceptor() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		// 获取 token
		token, err := c.Auth.Token(ctx)
		if err != nil {
			return fmt.Errorf("failed to get auth token: %w", err)
		}

		// 注入 token 到 metadata
		ctx = c.injectToken(ctx, token)

		// 调用原始方法
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// streamAuthInterceptor 流式 RPC 鉴权拦截器
func (c *Client) streamAuthInterceptor() grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		// 获取 token
		token, err := c.Auth.Token(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get auth token: %w", err)
		}

		// 注入 token 到 metadata
		ctx = c.injectToken(ctx, token)

		// 调用原始 streamer
		return streamer(ctx, desc, cc, method, opts...)
	}
}

// injectToken 将 token 注入到 context 的 metadata 中
func (c *Client) injectToken(ctx context.Context, token string) context.Context {
	// 获取或创建 metadata
	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		md = metadata.New(nil)
	} else {
		md = md.Copy()
	}

	// 注入 authorization header
	md.Set("authorization", "Bearer "+token)

	return metadata.NewOutgoingContext(ctx, md)
}

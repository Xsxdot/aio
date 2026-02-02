package tracer

import (
	"context"
	uuid "github.com/satori/go.uuid"
	"github.com/xsxdot/aio/pkg/core/consts"
)

// Tracer 定义通用的追踪接口
type Tracer interface {
	// StartTrace 开始一个新的追踪
	StartTrace(ctx context.Context, name string) (context.Context, string, func())

	// StartTraceWithParent 使用父追踪上下文开始一个新的追踪
	StartTraceWithParent(ctx context.Context, name string, parentTraceID string) (context.Context, string, func(), error)
}

// SimpleTracer 简单追踪实现，只生成和传递TraceID
type SimpleTracer struct{}

func NewSimpleTracer() *SimpleTracer {
	return &SimpleTracer{}
}

func (t *SimpleTracer) StartTrace(ctx context.Context, name string) (context.Context, string, func()) {
	traceID := uuid.NewV4().String()
	return context.WithValue(ctx, consts.TraceKey, traceID), traceID, func() {}
}

func (t *SimpleTracer) StartTraceWithParent(ctx context.Context, name string, parentTraceID string) (context.Context, string, func(), error) {
	return ctx, parentTraceID, func() {}, nil
}

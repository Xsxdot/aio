package tracer

import (
	"context"
	"encoding/json"
	"github.com/openzipkin/zipkin-go"
	"github.com/openzipkin/zipkin-go/model"
	"xiaozhizhang/pkg/core/consts"
)

// ZipkinTracer Zipkin追踪实现
type ZipkinTracer struct {
	tracer  *zipkin.Tracer
	appName string
}

func NewZipkinTracer(tracer *zipkin.Tracer, appName string) *ZipkinTracer {
	return &ZipkinTracer{
		tracer:  tracer,
		appName: appName,
	}
}

func (t *ZipkinTracer) StartTrace(ctx context.Context, name string) (context.Context, string, func()) {
	span, newCtx := t.tracer.StartSpanFromContext(ctx, t.appName+"."+name)
	traceId := span.Context().TraceID.String()
	return context.WithValue(newCtx, consts.TraceKey, traceId), traceId, span.Finish
}

func (t *ZipkinTracer) StartTraceWithParent(ctx context.Context, name string, parentTraceStr string) (context.Context, string, func(), error) {
	spanContext := new(model.SpanContext)
	err := json.Unmarshal([]byte(parentTraceStr), spanContext)
	if err != nil {
		return ctx, "", func() {}, err
	}

	span := t.tracer.StartSpan(t.appName+"."+name, zipkin.Parent(*spanContext))
	newCtx := zipkin.NewContext(ctx, span)
	return newCtx, span.Context().TraceID.String(), span.Finish, nil
}

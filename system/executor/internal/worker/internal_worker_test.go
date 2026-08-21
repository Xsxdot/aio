package worker

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/xsxdot/aio/system/executor/api/callback"
	"github.com/xsxdot/aio/system/executor/internal/model"
	"github.com/xsxdot/aio/system/executor/internal/service"
	"github.com/xsxdot/gokit/logger"
)

type fakeRunner struct {
	acquired    []*service.AcquiredJobResult
	dispatched  []callback.CallbackPayload
	acked       []model.JobStatus
	dispatchErr error
}

func (f *fakeRunner) AcquireJobs(ctx context.Context, req service.AcquireJobsRequest) ([]*service.AcquiredJobResult, error) {
	out := f.acquired
	f.acquired = nil // 只投递一轮，避免测试无限循环
	return out, nil
}

func (f *fakeRunner) AckJob(ctx context.Context, jobID uint64, attemptNo int32, consumerID string,
	status model.JobStatus, errorMsg, resultJSON string, retryAfter int32,
	stopRetry bool, addMaxAttempts int32, errorType string) error {
	f.acked = append(f.acked, status)
	return nil
}

func (f *fakeRunner) DispatchCompletionCallback(ctx context.Context, source string,
	jobID uint64, callbackData, resultJSON string) error {
	f.dispatched = append(f.dispatched, callback.CallbackPayload{
		JobID: jobID, Source: source, CallbackData: callbackData, ResultJSON: resultJSON,
	})
	return f.dispatchErr
}

// newOutboxJob 构造一条 outbox 回调任务。
// outboxID 是 outbox 任务自身的 ID（worker 用它 Ack）；
// originJobID 是载荷里的原始业务任务 ID（派发给 handler 的那个）。两者必须区分。
func newOutboxJob(t *testing.T, outboxID int64, originJobID uint64, source string) *service.AcquiredJobResult {
	t.Helper()
	args, err := json.Marshal(callback.CallbackPayload{
		JobID: originJobID, Source: source,
		CallbackData: `{"instance_id":7,"node_id":"A","env":"dev"}`,
		ResultJSON:   `{"ok":true}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	job := &model.ExecutorJobModel{
		Env: "dev", TargetService: callback.InternalTargetService,
		Method: callback.MethodJobCompletedCallback, ArgsJSON: string(args),
	}
	job.ID = outboxID
	return &service.AcquiredJobResult{Job: job, AttemptNo: 1, ConsumerID: "aio-1-slot-0"}
}

// 领到 outbox 任务后按 source 路由派发，并以 succeeded 确认。
func TestWorkerDispatchesAndAcksSucceeded(t *testing.T) {
	job := newOutboxJob(t, 555, 9001, "workflow")
	f := &fakeRunner{acquired: []*service.AcquiredJobResult{job}}
	w := NewInternalCallbackWorker(f, "dev", "aio-1", logger.GetLogger())

	w.pollOnce(context.Background())

	if len(f.dispatched) != 1 {
		t.Fatalf("dispatched = %d, want 1", len(f.dispatched))
	}
	if f.dispatched[0].JobID != 9001 || f.dispatched[0].Source != "workflow" {
		t.Fatalf("dispatched payload = %+v", f.dispatched[0])
	}
	if len(f.acked) != 1 || f.acked[0] != model.JobStatusSucceeded {
		t.Fatalf("acked = %v, want [succeeded]", f.acked)
	}
}

// 派发失败必须以 failed 确认，让 executor 重试而不是静默丢弃。
func TestWorkerAcksFailedOnDispatchError(t *testing.T) {
	job := newOutboxJob(t, 556, 9002, "workflow")
	f := &fakeRunner{
		acquired:    []*service.AcquiredJobResult{job},
		dispatchErr: errors.New("handler boom"),
	}
	w := NewInternalCallbackWorker(f, "dev", "aio-1", logger.GetLogger())

	w.pollOnce(context.Background())

	if len(f.acked) != 1 || f.acked[0] != model.JobStatusFailed {
		t.Fatalf("acked = %v, want [failed]", f.acked)
	}
}

// ArgsJSON 无法解析时以 failed 确认（重试到死信后人工可见），不得 panic。
func TestWorkerAcksFailedOnBadPayload(t *testing.T) {
	// ID 来自内嵌的 common.Model，Go 不允许在复合字面量里设置提升字段，必须单独赋值
	badJob := &model.ExecutorJobModel{
		Env:           "dev",
		TargetService: callback.InternalTargetService,
		Method:        callback.MethodJobCompletedCallback,
		ArgsJSON:      `{not json`,
	}
	badJob.ID = 557
	f := &fakeRunner{acquired: []*service.AcquiredJobResult{{
		Job: badJob, AttemptNo: 1, ConsumerID: "aio-1-slot-0",
	}}}
	w := NewInternalCallbackWorker(f, "dev", "aio-1", logger.GetLogger())

	w.pollOnce(context.Background())

	if len(f.dispatched) != 0 {
		t.Fatalf("dispatched = %d, want 0", len(f.dispatched))
	}
	if len(f.acked) != 1 || f.acked[0] != model.JobStatusFailed {
		t.Fatalf("acked = %v, want [failed]", f.acked)
	}
}

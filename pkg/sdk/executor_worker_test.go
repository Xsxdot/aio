package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"testing"
	"time"

	executorpb "github.com/xsxdot/aio/system/executor/api/proto"
	"github.com/xsxdot/gokit/scheduler"
	"google.golang.org/grpc"
)

// mockExecutorServiceClient 模拟 ExecutorServiceClient
type mockExecutorServiceClient struct {
	executorpb.ExecutorServiceClient

	mu              sync.Mutex
	acquireFunc     func(ctx context.Context, in *executorpb.AcquireJobRequest, opts ...grpc.CallOption) (*executorpb.AcquireJobResponse, error)
	acquireJobsFunc func(ctx context.Context, in *executorpb.AcquireJobsRequest, opts ...grpc.CallOption) (*executorpb.AcquireJobsResponse, error)
	renewLeaseFunc  func(ctx context.Context, in *executorpb.RenewLeaseRequest, opts ...grpc.CallOption) (*executorpb.RenewLeaseResponse, error)
	ackJobFunc      func(ctx context.Context, in *executorpb.AckJobRequest, opts ...grpc.CallOption) (*executorpb.AckJobResponse, error)

	// 记录调用
	acquireCalls     []acquireCall
	acquireJobsCalls []acquireJobsCall
	renewCalls       []renewCall
	ackCalls         []ackCall
}

type acquireCall struct {
	req  *executorpb.AcquireJobRequest
	time time.Time
}

type acquireJobsCall struct {
	req  *executorpb.AcquireJobsRequest
	time time.Time
}

type renewCall struct {
	req  *executorpb.RenewLeaseRequest
	time time.Time
}

type ackCall struct {
	req  *executorpb.AckJobRequest
	time time.Time
}

func (m *mockExecutorServiceClient) AcquireJob(ctx context.Context, in *executorpb.AcquireJobRequest, opts ...grpc.CallOption) (*executorpb.AcquireJobResponse, error) {
	m.mu.Lock()
	m.acquireCalls = append(m.acquireCalls, acquireCall{req: in, time: time.Now()})
	m.mu.Unlock()

	if m.acquireFunc != nil {
		return m.acquireFunc(ctx, in, opts...)
	}
	return &executorpb.AcquireJobResponse{JobId: 0}, nil
}

func (m *mockExecutorServiceClient) AcquireJobs(ctx context.Context, in *executorpb.AcquireJobsRequest, opts ...grpc.CallOption) (*executorpb.AcquireJobsResponse, error) {
	m.mu.Lock()
	m.acquireJobsCalls = append(m.acquireJobsCalls, acquireJobsCall{req: in, time: time.Now()})
	m.mu.Unlock()

	if m.acquireJobsFunc != nil {
		return m.acquireJobsFunc(ctx, in, opts...)
	}
	if m.acquireFunc != nil {
		resp := &executorpb.AcquireJobsResponse{}
		for _, method := range in.Methods {
			consumerID := in.BaseConsumerId
			if in.Mode == executorpb.AcquireJobsMode_ACQUIRE_JOBS_MODE_ONE_PER_METHOD {
				consumerID = fmt.Sprintf("%s-m-%s", in.BaseConsumerId, method)
			}
			job, err := m.AcquireJob(ctx, &executorpb.AcquireJobRequest{
				Env:           in.Env,
				TargetService: in.TargetService,
				Method:        method,
				ConsumerId:    consumerID,
				LeaseDuration: in.LeaseDuration,
			}, opts...)
			if err != nil {
				return nil, err
			}
			if job.GetJobId() == 0 {
				continue
			}
			resp.Jobs = append(resp.Jobs, &executorpb.AcquiredJobItem{
				JobId:         job.JobId,
				AttemptNo:     job.AttemptNo,
				Env:           job.Env,
				TargetService: job.TargetService,
				Method:        job.Method,
				ArgsJson:      job.ArgsJson,
				LeaseUntil:    job.LeaseUntil,
				ConsumerId:    consumerID,
			})
		}
		return resp, nil
	}
	return &executorpb.AcquireJobsResponse{}, nil
}

func (m *mockExecutorServiceClient) RenewLease(ctx context.Context, in *executorpb.RenewLeaseRequest, opts ...grpc.CallOption) (*executorpb.RenewLeaseResponse, error) {
	m.mu.Lock()
	m.renewCalls = append(m.renewCalls, renewCall{req: in, time: time.Now()})
	m.mu.Unlock()

	if m.renewLeaseFunc != nil {
		return m.renewLeaseFunc(ctx, in, opts...)
	}
	return &executorpb.RenewLeaseResponse{Success: true, LeaseUntil: time.Now().Unix() + 30}, nil
}

func (m *mockExecutorServiceClient) AckJob(ctx context.Context, in *executorpb.AckJobRequest, opts ...grpc.CallOption) (*executorpb.AckJobResponse, error) {
	m.mu.Lock()
	m.ackCalls = append(m.ackCalls, ackCall{req: in, time: time.Now()})
	m.mu.Unlock()

	if m.ackJobFunc != nil {
		return m.ackJobFunc(ctx, in, opts...)
	}
	return &executorpb.AckJobResponse{Success: true}, nil
}

func (m *mockExecutorServiceClient) getAcquireCalls() []acquireCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	calls := make([]acquireCall, len(m.acquireCalls))
	copy(calls, m.acquireCalls)
	return calls
}

func (m *mockExecutorServiceClient) getAcquireJobsCalls() []acquireJobsCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	calls := make([]acquireJobsCall, len(m.acquireJobsCalls))
	copy(calls, m.acquireJobsCalls)
	return calls
}

func (m *mockExecutorServiceClient) getRenewCalls() []renewCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	calls := make([]renewCall, len(m.renewCalls))
	copy(calls, m.renewCalls)
	return calls
}

func (m *mockExecutorServiceClient) getAckCalls() []ackCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	calls := make([]ackCall, len(m.ackCalls))
	copy(calls, m.ackCalls)
	return calls
}

func (m *mockExecutorServiceClient) reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.acquireCalls = nil
	m.acquireJobsCalls = nil
	m.renewCalls = nil
	m.ackCalls = nil
}

// createTestWorker 创建测试用 Worker
func createTestWorker(t *testing.T, mock *mockExecutorServiceClient) (*ExecutorWorker, *scheduler.Scheduler) {
	client := &ExecutorClient{
		service: mock,
	}

	// 创建 scheduler
	schedulerConfig := scheduler.DefaultSchedulerConfig()
	schedulerConfig.MaxWorkers = 5
	s := scheduler.NewScheduler(schedulerConfig)

	// 创建 worker
	config := &WorkerConfig{
		TargetService:   "test-service",
		ConsumerID:      "test-worker",
		LeaseDuration:   30,
		EnableAutoRenew: false, // 默认禁用自动续租，按需启用
		TaskTimeout:     5 * time.Second,
		PollInterval:    100 * time.Millisecond, // 测试时使用更短的间隔
	}

	worker, err := client.NewWorkerWithScheduler(s, config)
	if err != nil {
		t.Fatalf("failed to create worker: %v", err)
	}

	return worker, s
}

func sameStringSet(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	gotCopy := append([]string(nil), got...)
	wantCopy := append([]string(nil), want...)
	sort.Strings(gotCopy)
	sort.Strings(wantCopy)
	for i := range gotCopy {
		if gotCopy[i] != wantCopy[i] {
			return false
		}
	}
	return true
}

func TestExecutorClientAcquireJobsMapsRequestAndResponse(t *testing.T) {
	mock := &mockExecutorServiceClient{}
	client := &ExecutorClient{env: "dev", service: mock}
	mock.acquireJobsFunc = func(ctx context.Context, in *executorpb.AcquireJobsRequest, opts ...grpc.CallOption) (*executorpb.AcquireJobsResponse, error) {
		if in.Env != "dev" {
			t.Fatalf("env = %q", in.Env)
		}
		if in.TargetService != "tk-server" {
			t.Fatalf("target_service = %q", in.TargetService)
		}
		if in.BaseConsumerId != "worker-base" {
			t.Fatalf("base_consumer_id = %q", in.BaseConsumerId)
		}
		if len(in.ConsumerIds) != 0 {
			t.Fatalf("consumer_ids = %v, want empty for ONE_PER_METHOD", in.ConsumerIds)
		}
		if in.Mode != executorpb.AcquireJobsMode_ACQUIRE_JOBS_MODE_ONE_PER_METHOD {
			t.Fatalf("mode = %v", in.Mode)
		}
		return &executorpb.AcquireJobsResponse{Jobs: []*executorpb.AcquiredJobItem{{
			JobId:         101,
			AttemptNo:     2,
			Env:           "dev",
			TargetService: "tk-server",
			Method:        "method.a",
			ArgsJson:      `{}`,
			LeaseUntil:    999,
			ConsumerId:    "worker-base-m-method.a",
		}}}, nil
	}

	jobs, err := client.AcquireJobs(context.Background(), &AcquireJobsRequest{
		TargetService:  "tk-server",
		Methods:        []string{"method.a"},
		BaseConsumerID: "worker-base",
		LeaseDuration:  60,
		Mode:           AcquireJobsModeOnePerMethod,
	})
	if err != nil {
		t.Fatalf("AcquireJobs: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("jobs len = %d, want 1", len(jobs))
	}
	if jobs[0].JobID != 101 || jobs[0].AttemptNo != 2 || jobs[0].ConsumerID != "worker-base-m-method.a" {
		t.Fatalf("job = %#v", jobs[0])
	}
}

func TestExecutorWorkerUsesSingleBatchPollForRegisteredMethods(t *testing.T) {
	mock := &mockExecutorServiceClient{}
	worker, s := createTestWorker(t, mock)

	if err := worker.Register("method.a", func(ctx context.Context, job *AcquiredJob) (interface{}, error) {
		return nil, nil
	}); err != nil {
		t.Fatalf("register method.a: %v", err)
	}
	if err := worker.Register("method.b", func(ctx context.Context, job *AcquiredJob) (interface{}, error) {
		return nil, nil
	}); err != nil {
		t.Fatalf("register method.b: %v", err)
	}

	acquireJobsCalled := make(chan struct{}, 1)
	mock.acquireJobsFunc = func(ctx context.Context, in *executorpb.AcquireJobsRequest, opts ...grpc.CallOption) (*executorpb.AcquireJobsResponse, error) {
		if in.Mode != executorpb.AcquireJobsMode_ACQUIRE_JOBS_MODE_ONE_PER_METHOD {
			t.Fatalf("mode = %v", in.Mode)
		}
		if !sameStringSet(in.Methods, []string{"method.a", "method.b"}) {
			t.Fatalf("methods = %v", in.Methods)
		}
		if in.BaseConsumerId != "test-worker" {
			t.Fatalf("base_consumer_id = %q", in.BaseConsumerId)
		}
		if len(in.ConsumerIds) != 0 {
			t.Fatalf("consumer_ids = %v, want empty", in.ConsumerIds)
		}
		select {
		case acquireJobsCalled <- struct{}{}:
		default:
		}
		return &executorpb.AcquireJobsResponse{}, nil
	}

	if err := s.Start(); err != nil {
		t.Fatalf("failed to start scheduler: %v", err)
	}
	defer s.Stop()

	if err := worker.Start(); err != nil {
		t.Fatalf("failed to start worker: %v", err)
	}
	defer worker.Stop()

	select {
	case <-acquireJobsCalled:
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("AcquireJobs was not called; legacy AcquireJob calls = %d", len(mock.getAcquireCalls()))
	}

	if legacyCalls := mock.getAcquireCalls(); len(legacyCalls) != 0 {
		t.Fatalf("legacy AcquireJob calls = %d, want 0", len(legacyCalls))
	}
}

func TestConcurrentExecutorWorkerUsesOneBatchAcquireForFreeSlots(t *testing.T) {
	mock := &mockExecutorServiceClient{}
	client := &ExecutorClient{service: mock}
	worker, err := client.NewConcurrentWorker(&ConcurrentWorkerConfig{
		WorkerConfig: WorkerConfig{
			TargetService:   "test-service",
			ConsumerID:      "test-worker",
			LeaseDuration:   30,
			EnableAutoRenew: false,
			TaskTimeout:     5 * time.Second,
			PollInterval:    100 * time.Millisecond,
		},
		MaxConcurrent: 2,
	})
	if err != nil {
		t.Fatalf("new concurrent worker: %v", err)
	}

	if err := worker.Register("method.a", func(ctx context.Context, job *AcquiredJob) (interface{}, error) {
		return nil, nil
	}); err != nil {
		t.Fatalf("register method.a: %v", err)
	}
	if err := worker.Register("method.b", func(ctx context.Context, job *AcquiredJob) (interface{}, error) {
		return nil, nil
	}); err != nil {
		t.Fatalf("register method.b: %v", err)
	}

	acquireJobsCalled := make(chan struct{}, 1)
	mock.acquireJobsFunc = func(ctx context.Context, in *executorpb.AcquireJobsRequest, opts ...grpc.CallOption) (*executorpb.AcquireJobsResponse, error) {
		if in.Mode != executorpb.AcquireJobsMode_ACQUIRE_JOBS_MODE_FILL_SLOTS {
			t.Fatalf("mode = %v", in.Mode)
		}
		if !sameStringSet(in.Methods, []string{"method.a", "method.b"}) {
			t.Fatalf("methods = %v", in.Methods)
		}
		if len(in.ConsumerIds) != 2 {
			t.Fatalf("consumer_ids len = %d, want 2: %v", len(in.ConsumerIds), in.ConsumerIds)
		}
		if in.BaseConsumerId != "" {
			t.Fatalf("base_consumer_id = %q, want empty", in.BaseConsumerId)
		}
		select {
		case acquireJobsCalled <- struct{}{}:
		default:
		}
		return &executorpb.AcquireJobsResponse{}, nil
	}

	if err := worker.Start(); err != nil {
		t.Fatalf("start concurrent worker: %v", err)
	}
	defer worker.Stop()

	select {
	case <-acquireJobsCalled:
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("AcquireJobs was not called; legacy AcquireJob calls = %d", len(mock.getAcquireCalls()))
	}

	if legacyCalls := mock.getAcquireCalls(); len(legacyCalls) != 0 {
		t.Fatalf("legacy AcquireJob calls = %d, want 0", len(legacyCalls))
	}
}

// TestWorker_SuccessfulJob 测试成功执行任务并 Ack 成功
func TestWorker_SuccessfulJob(t *testing.T) {
	mock := &mockExecutorServiceClient{}
	worker, s := createTestWorker(t, mock)

	// 设置 mock 行为：返回一个任务
	jobReturned := false
	mock.acquireFunc = func(ctx context.Context, in *executorpb.AcquireJobRequest, opts ...grpc.CallOption) (*executorpb.AcquireJobResponse, error) {
		if !jobReturned {
			jobReturned = true
			return &executorpb.AcquireJobResponse{
				JobId:         123,
				AttemptNo:     1,
				TargetService: "test-service",
				Method:        "TestMethod",
				ArgsJson:      `{"value": 42}`,
				LeaseUntil:    time.Now().Unix() + 30,
			}, nil
		}
		// 后续返回无任务
		return &executorpb.AcquireJobResponse{JobId: 0}, nil
	}

	// 注册处理器
	var handlerCalled bool
	var handlerJob *AcquiredJob
	err := worker.Register("TestMethod", func(ctx context.Context, job *AcquiredJob) (interface{}, error) {
		handlerCalled = true
		handlerJob = job
		return map[string]interface{}{"status": "success"}, nil
	})
	if err != nil {
		t.Fatalf("failed to register handler: %v", err)
	}

	// 启动 scheduler 和 worker
	if err := s.Start(); err != nil {
		t.Fatalf("failed to start scheduler: %v", err)
	}
	defer s.Stop()

	if err := worker.Start(); err != nil {
		t.Fatalf("failed to start worker: %v", err)
	}
	defer worker.Stop()

	// 等待任务执行
	time.Sleep(500 * time.Millisecond)

	// 验证 handler 被调用
	if !handlerCalled {
		t.Error("handler was not called")
	}
	if handlerJob == nil {
		t.Fatal("handler job is nil")
	}
	if handlerJob.JobID != 123 {
		t.Errorf("expected job ID 123, got %d", handlerJob.JobID)
	}
	if handlerJob.Method != "TestMethod" {
		t.Errorf("expected method TestMethod, got %s", handlerJob.Method)
	}

	// 验证 Ack 调用
	ackCalls := mock.getAckCalls()
	if len(ackCalls) != 1 {
		t.Fatalf("expected 1 ack call, got %d", len(ackCalls))
	}

	ackReq := ackCalls[0].req
	if ackReq.JobId != 123 {
		t.Errorf("expected job ID 123, got %d", ackReq.JobId)
	}
	if ackReq.AttemptNo != 1 {
		t.Errorf("expected attempt no 1, got %d", ackReq.AttemptNo)
	}
	if ackReq.Status != executorpb.JobStatus_JOB_STATUS_SUCCEEDED {
		t.Errorf("expected status SUCCEEDED, got %v", ackReq.Status)
	}
	if ackReq.Error != "" {
		t.Errorf("expected no error, got %s", ackReq.Error)
	}

	// 验证结果 JSON
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(ackReq.ResultJson), &result); err != nil {
		t.Fatalf("failed to unmarshal result JSON: %v", err)
	}
	if result["status"] != "success" {
		t.Errorf("expected status success, got %v", result["status"])
	}
}

// TestWorker_FailedJob 测试任务失败并 Ack 失败
func TestWorker_FailedJob(t *testing.T) {
	mock := &mockExecutorServiceClient{}
	worker, s := createTestWorker(t, mock)

	// 设置 mock 行为
	jobReturned := false
	mock.acquireFunc = func(ctx context.Context, in *executorpb.AcquireJobRequest, opts ...grpc.CallOption) (*executorpb.AcquireJobResponse, error) {
		if !jobReturned {
			jobReturned = true
			return &executorpb.AcquireJobResponse{
				JobId:         456,
				AttemptNo:     2,
				TargetService: "test-service",
				Method:        "FailingMethod",
				ArgsJson:      `{}`,
				LeaseUntil:    time.Now().Unix() + 30,
			}, nil
		}
		return &executorpb.AcquireJobResponse{JobId: 0}, nil
	}

	// 注册处理器（返回普通错误）
	err := worker.Register("FailingMethod", func(ctx context.Context, job *AcquiredJob) (interface{}, error) {
		return nil, fmt.Errorf("something went wrong")
	})
	if err != nil {
		t.Fatalf("failed to register handler: %v", err)
	}

	// 启动
	if err := s.Start(); err != nil {
		t.Fatalf("failed to start scheduler: %v", err)
	}
	defer s.Stop()

	if err := worker.Start(); err != nil {
		t.Fatalf("failed to start worker: %v", err)
	}
	defer worker.Stop()

	// 等待执行
	time.Sleep(500 * time.Millisecond)

	// 验证 Ack 调用
	ackCalls := mock.getAckCalls()
	if len(ackCalls) != 1 {
		t.Fatalf("expected 1 ack call, got %d", len(ackCalls))
	}

	ackReq := ackCalls[0].req
	if ackReq.JobId != 456 {
		t.Errorf("expected job ID 456, got %d", ackReq.JobId)
	}
	if ackReq.Status != executorpb.JobStatus_JOB_STATUS_FAILED {
		t.Errorf("expected status FAILED, got %v", ackReq.Status)
	}
	if ackReq.Error != "something went wrong" {
		t.Errorf("expected error 'something went wrong', got %s", ackReq.Error)
	}
	if ackReq.RetryAfter != 0 {
		t.Errorf("expected retry after 0, got %d", ackReq.RetryAfter)
	}
}

// TestWorker_FailedJobWithRetryAfter 测试任务失败并指定 RetryAfter
func TestWorker_FailedJobWithRetryAfter(t *testing.T) {
	mock := &mockExecutorServiceClient{}
	worker, s := createTestWorker(t, mock)

	// 设置 mock 行为
	jobReturned := false
	mock.acquireFunc = func(ctx context.Context, in *executorpb.AcquireJobRequest, opts ...grpc.CallOption) (*executorpb.AcquireJobResponse, error) {
		if !jobReturned {
			jobReturned = true
			return &executorpb.AcquireJobResponse{
				JobId:         789,
				AttemptNo:     1,
				TargetService: "test-service",
				Method:        "RetryableMethod",
				ArgsJson:      `{}`,
				LeaseUntil:    time.Now().Unix() + 30,
			}, nil
		}
		return &executorpb.AcquireJobResponse{JobId: 0}, nil
	}

	// 注册处理器（返回 JobFailedError）
	err := worker.Register("RetryableMethod", func(ctx context.Context, job *AcquiredJob) (interface{}, error) {
		return nil, &JobFailedError{
			Message:    "temporary failure",
			RetryAfter: 60,
		}
	})
	if err != nil {
		t.Fatalf("failed to register handler: %v", err)
	}

	// 启动
	if err := s.Start(); err != nil {
		t.Fatalf("failed to start scheduler: %v", err)
	}
	defer s.Stop()

	if err := worker.Start(); err != nil {
		t.Fatalf("failed to start worker: %v", err)
	}
	defer worker.Stop()

	// 等待执行
	time.Sleep(500 * time.Millisecond)

	// 验证 Ack 调用
	ackCalls := mock.getAckCalls()
	if len(ackCalls) != 1 {
		t.Fatalf("expected 1 ack call, got %d", len(ackCalls))
	}

	ackReq := ackCalls[0].req
	if ackReq.JobId != 789 {
		t.Errorf("expected job ID 789, got %d", ackReq.JobId)
	}
	if ackReq.Status != executorpb.JobStatus_JOB_STATUS_FAILED {
		t.Errorf("expected status FAILED, got %v", ackReq.Status)
	}
	if ackReq.Error != "temporary failure (retry after 60s)" {
		t.Errorf("expected error 'temporary failure (retry after 60s)', got %s", ackReq.Error)
	}
	if ackReq.RetryAfter != 60 {
		t.Errorf("expected retry after 60, got %d", ackReq.RetryAfter)
	}
}

// TestWorker_PanicInHandler 测试 handler panic 时的处理
func TestWorker_PanicInHandler(t *testing.T) {
	mock := &mockExecutorServiceClient{}
	worker, s := createTestWorker(t, mock)

	// 设置 mock 行为
	jobReturned := false
	mock.acquireFunc = func(ctx context.Context, in *executorpb.AcquireJobRequest, opts ...grpc.CallOption) (*executorpb.AcquireJobResponse, error) {
		if !jobReturned {
			jobReturned = true
			return &executorpb.AcquireJobResponse{
				JobId:         999,
				AttemptNo:     1,
				TargetService: "test-service",
				Method:        "PanicMethod",
				ArgsJson:      `{}`,
				LeaseUntil:    time.Now().Unix() + 30,
			}, nil
		}
		return &executorpb.AcquireJobResponse{JobId: 0}, nil
	}

	// 注册处理器（会 panic）
	err := worker.Register("PanicMethod", func(ctx context.Context, job *AcquiredJob) (interface{}, error) {
		panic("something terrible happened")
	})
	if err != nil {
		t.Fatalf("failed to register handler: %v", err)
	}

	// 启动
	if err := s.Start(); err != nil {
		t.Fatalf("failed to start scheduler: %v", err)
	}
	defer s.Stop()

	if err := worker.Start(); err != nil {
		t.Fatalf("failed to start worker: %v", err)
	}
	defer worker.Stop()

	// 等待执行
	time.Sleep(500 * time.Millisecond)

	// 验证 Ack 调用（panic 应该被捕获并 Ack FAILED）
	ackCalls := mock.getAckCalls()
	if len(ackCalls) != 1 {
		t.Fatalf("expected 1 ack call, got %d", len(ackCalls))
	}

	ackReq := ackCalls[0].req
	if ackReq.JobId != 999 {
		t.Errorf("expected job ID 999, got %d", ackReq.JobId)
	}
	if ackReq.Status != executorpb.JobStatus_JOB_STATUS_FAILED {
		t.Errorf("expected status FAILED, got %v", ackReq.Status)
	}
	if ackReq.Error != "handler panic: something terrible happened" {
		t.Errorf("expected panic error, got %s", ackReq.Error)
	}
}

// TestWorker_AutoRenewLease 测试自动续租
func TestWorker_AutoRenewLease(t *testing.T) {
	mock := &mockExecutorServiceClient{}
	client := &ExecutorClient{
		service: mock,
	}

	// 创建 scheduler
	schedulerConfig := scheduler.DefaultSchedulerConfig()
	s := scheduler.NewScheduler(schedulerConfig)

	// 创建 worker（启用自动续租）
	config := &WorkerConfig{
		TargetService:   "test-service",
		ConsumerID:      "test-worker",
		LeaseDuration:   30,
		EnableAutoRenew: true,
		RenewInterval:   200 * time.Millisecond, // 快速续租以便测试
		TaskTimeout:     2 * time.Second,        // 任务执行较长时间
		PollInterval:    100 * time.Millisecond,
	}

	worker, err := client.NewWorkerWithScheduler(s, config)
	if err != nil {
		t.Fatalf("failed to create worker: %v", err)
	}

	// 设置 mock 行为
	jobReturned := false
	mock.acquireFunc = func(ctx context.Context, in *executorpb.AcquireJobRequest, opts ...grpc.CallOption) (*executorpb.AcquireJobResponse, error) {
		if !jobReturned {
			jobReturned = true
			return &executorpb.AcquireJobResponse{
				JobId:         111,
				AttemptNo:     1,
				TargetService: "test-service",
				Method:        "LongRunningMethod",
				ArgsJson:      `{}`,
				LeaseUntil:    time.Now().Unix() + 30,
			}, nil
		}
		return &executorpb.AcquireJobResponse{JobId: 0}, nil
	}

	// 注册处理器（长时间运行）
	handlerDone := make(chan struct{})
	err = worker.Register("LongRunningMethod", func(ctx context.Context, job *AcquiredJob) (interface{}, error) {
		// 执行 1 秒（应该触发至少一次续租）
		time.Sleep(1 * time.Second)
		close(handlerDone)
		return nil, nil
	})
	if err != nil {
		t.Fatalf("failed to register handler: %v", err)
	}

	// 启动
	if err := s.Start(); err != nil {
		t.Fatalf("failed to start scheduler: %v", err)
	}
	defer s.Stop()

	if err := worker.Start(); err != nil {
		t.Fatalf("failed to start worker: %v", err)
	}
	defer worker.Stop()

	// 等待 handler 完成
	select {
	case <-handlerDone:
	case <-time.After(3 * time.Second):
		t.Fatal("handler did not complete in time")
	}

	// 等待 Ack
	time.Sleep(200 * time.Millisecond)

	// 验证续租调用（应该至少有一次）
	renewCalls := mock.getRenewCalls()
	if len(renewCalls) < 1 {
		t.Errorf("expected at least 1 renew call, got %d", len(renewCalls))
	}

	if len(renewCalls) > 0 {
		renewReq := renewCalls[0].req
		if renewReq.JobId != 111 {
			t.Errorf("expected job ID 111, got %d", renewReq.JobId)
		}
		if renewReq.AttemptNo != 1 {
			t.Errorf("expected attempt no 1, got %d", renewReq.AttemptNo)
		}
		if renewReq.ExtendDuration != 30 {
			t.Errorf("expected extend duration 30, got %d", renewReq.ExtendDuration)
		}
	}

	// 验证 Ack 调用
	ackCalls := mock.getAckCalls()
	if len(ackCalls) != 1 {
		t.Fatalf("expected 1 ack call, got %d", len(ackCalls))
	}
}

// TestWorker_NoJob 测试无任务时的轮询
func TestWorker_NoJob(t *testing.T) {
	mock := &mockExecutorServiceClient{}
	worker, s := createTestWorker(t, mock)

	// 设置 mock 行为：始终返回无任务
	mock.acquireFunc = func(ctx context.Context, in *executorpb.AcquireJobRequest, opts ...grpc.CallOption) (*executorpb.AcquireJobResponse, error) {
		return &executorpb.AcquireJobResponse{JobId: 0}, nil
	}

	// 注册处理器
	var handlerCalled bool
	err := worker.Register("TestMethod", func(ctx context.Context, job *AcquiredJob) (interface{}, error) {
		handlerCalled = true
		return nil, nil
	})
	if err != nil {
		t.Fatalf("failed to register handler: %v", err)
	}

	// 启动
	if err := s.Start(); err != nil {
		t.Fatalf("failed to start scheduler: %v", err)
	}
	defer s.Stop()

	if err := worker.Start(); err != nil {
		t.Fatalf("failed to start worker: %v", err)
	}
	defer worker.Stop()

	// 等待几个轮询周期
	time.Sleep(500 * time.Millisecond)

	// 验证 handler 未被调用
	if handlerCalled {
		t.Error("handler should not be called when no job available")
	}

	// 验证至少有多次 Acquire 调用（轮询）
	acquireCalls := mock.getAcquireCalls()
	if len(acquireCalls) < 2 {
		t.Errorf("expected at least 2 acquire calls, got %d", len(acquireCalls))
	}

	// 验证无 Ack 调用
	ackCalls := mock.getAckCalls()
	if len(ackCalls) != 0 {
		t.Errorf("expected 0 ack calls, got %d", len(ackCalls))
	}
}

// TestWorker_MultipleHandlers 测试多个处理器
func TestWorker_MultipleHandlers(t *testing.T) {
	mock := &mockExecutorServiceClient{}
	worker, s := createTestWorker(t, mock)

	// 跟踪返回的任务
	mu := sync.Mutex{}
	jobs := []string{"Method1", "Method2"}
	jobIndex := 0

	mock.acquireFunc = func(ctx context.Context, in *executorpb.AcquireJobRequest, opts ...grpc.CallOption) (*executorpb.AcquireJobResponse, error) {
		mu.Lock()
		defer mu.Unlock()

		if jobIndex < len(jobs) {
			method := jobs[jobIndex]
			// 只为请求的方法返回任务
			if in.Method == method {
				jobIndex++
				return &executorpb.AcquireJobResponse{
					JobId:         int64(jobIndex),
					AttemptNo:     1,
					TargetService: "test-service",
					Method:        method,
					ArgsJson:      `{}`,
					LeaseUntil:    time.Now().Unix() + 30,
				}, nil
			}
		}
		return &executorpb.AcquireJobResponse{JobId: 0}, nil
	}

	// 注册多个处理器
	method1Called := false
	method2Called := false

	err := worker.Register("Method1", func(ctx context.Context, job *AcquiredJob) (interface{}, error) {
		method1Called = true
		return nil, nil
	})
	if err != nil {
		t.Fatalf("failed to register Method1: %v", err)
	}

	err = worker.Register("Method2", func(ctx context.Context, job *AcquiredJob) (interface{}, error) {
		method2Called = true
		return nil, nil
	})
	if err != nil {
		t.Fatalf("failed to register Method2: %v", err)
	}

	// 启动
	if err := s.Start(); err != nil {
		t.Fatalf("failed to start scheduler: %v", err)
	}
	defer s.Stop()

	if err := worker.Start(); err != nil {
		t.Fatalf("failed to start worker: %v", err)
	}
	defer worker.Stop()

	// 等待执行
	time.Sleep(1 * time.Second)

	// 验证两个 handler 都被调用
	if !method1Called {
		t.Error("Method1 handler was not called")
	}
	if !method2Called {
		t.Error("Method2 handler was not called")
	}

	// 验证有两次 Ack 调用
	ackCalls := mock.getAckCalls()
	if len(ackCalls) < 2 {
		t.Errorf("expected at least 2 ack calls, got %d", len(ackCalls))
	}
}

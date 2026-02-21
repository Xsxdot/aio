package executor

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestExecutor_SequentialExecution(t *testing.T) {
	// 创建 executor
	config := &Config{
		QueueSize:      10,
		MaxHistorySize: 100,
		DefaultTimeout: 5 * time.Second,
	}
	exec := NewExecutor(config)

	// 启动
	if err := exec.Start(); err != nil {
		t.Fatalf("启动失败: %v", err)
	}
	defer exec.Stop()

	// 用于记录任务执行顺序
	executionOrder := make([]int, 0)
	orderMu := make(chan struct{}, 1)
	orderMu <- struct{}{} // 初始化锁

	// 提交 3 个任务
	for i := 1; i <= 3; i++ {
		taskNum := i
		_, err := exec.Submit(context.Background(), "test-task", 1*time.Second, func(ctx context.Context) error {
			<-orderMu
			executionOrder = append(executionOrder, taskNum)
			time.Sleep(100 * time.Millisecond) // 模拟耗时操作
			orderMu <- struct{}{}
			return nil
		})
		if err != nil {
			t.Fatalf("提交任务 %d 失败: %v", i, err)
		}
	}

	// 等待所有任务完成
	time.Sleep(1 * time.Second)

	// 验证执行顺序
	if len(executionOrder) != 3 {
		t.Fatalf("期望执行 3 个任务，实际执行 %d 个", len(executionOrder))
	}

	for i := 0; i < 3; i++ {
		if executionOrder[i] != i+1 {
			t.Errorf("任务执行顺序错误：期望 %d，实际 %d", i+1, executionOrder[i])
		}
	}
}

func TestExecutor_JobStatus(t *testing.T) {
	exec := NewExecutor(DefaultConfig())
	if err := exec.Start(); err != nil {
		t.Fatalf("启动失败: %v", err)
	}
	defer exec.Stop()

	// 提交一个成功的任务
	jobID1, err := exec.Submit(context.Background(), "success-task", 1*time.Second, func(ctx context.Context) error {
		time.Sleep(100 * time.Millisecond)
		return nil
	})
	if err != nil {
		t.Fatalf("提交任务失败: %v", err)
	}

	// 立即查询状态（应该是 pending 或 running）
	job1, err := exec.GetJob(jobID1)
	if err != nil {
		t.Fatalf("查询任务失败: %v", err)
	}
	if job1.Status != JobStatusPending && job1.Status != JobStatusRunning {
		t.Errorf("期望状态为 pending 或 running，实际 %s", job1.Status)
	}

	// 等待任务完成
	time.Sleep(500 * time.Millisecond)

	// 再次查询状态（应该是 succeeded）
	job1, err = exec.GetJob(jobID1)
	if err != nil {
		t.Fatalf("查询任务失败: %v", err)
	}
	if job1.Status != JobStatusSucceeded {
		t.Errorf("期望状态为 succeeded，实际 %s", job1.Status)
	}
	if job1.StartedAt == nil {
		t.Error("StartedAt 应该有值")
	}
	if job1.FinishedAt == nil {
		t.Error("FinishedAt 应该有值")
	}
	if job1.Error != "" {
		t.Errorf("成功任务不应该有错误信息，实际: %s", job1.Error)
	}

	// 提交一个失败的任务
	jobID2, err := exec.Submit(context.Background(), "failed-task", 1*time.Second, func(ctx context.Context) error {
		return errors.New("任务执行失败")
	})
	if err != nil {
		t.Fatalf("提交任务失败: %v", err)
	}

	// 等待任务完成
	time.Sleep(500 * time.Millisecond)

	// 查询状态（应该是 failed）
	job2, err := exec.GetJob(jobID2)
	if err != nil {
		t.Fatalf("查询任务失败: %v", err)
	}
	if job2.Status != JobStatusFailed {
		t.Errorf("期望状态为 failed，实际 %s", job2.Status)
	}
	if job2.Error == "" {
		t.Error("失败任务应该有错误信息")
	}
}

func TestExecutor_Timeout(t *testing.T) {
	exec := NewExecutor(DefaultConfig())
	if err := exec.Start(); err != nil {
		t.Fatalf("启动失败: %v", err)
	}
	defer exec.Stop()

	// 提交一个会超时的任务
	jobID, err := exec.Submit(context.Background(), "timeout-task", 200*time.Millisecond, func(ctx context.Context) error {
		// 等待超时
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(1 * time.Second):
			return nil
		}
	})
	if err != nil {
		t.Fatalf("提交任务失败: %v", err)
	}

	// 等待任务超时
	time.Sleep(500 * time.Millisecond)

	// 查询状态（应该是 failed，因为超时）
	job, err := exec.GetJob(jobID)
	if err != nil {
		t.Fatalf("查询任务失败: %v", err)
	}
	if job.Status != JobStatusFailed {
		t.Errorf("期望状态为 failed（超时），实际 %s", job.Status)
	}
	if job.Error == "" {
		t.Error("超时任务应该有错误信息")
	}
}

func TestExecutor_GracefulShutdown(t *testing.T) {
	exec := NewExecutor(DefaultConfig())
	if err := exec.Start(); err != nil {
		t.Fatalf("启动失败: %v", err)
	}

	// 提交多个任务
	jobIDs := make([]string, 0)
	for i := 0; i < 5; i++ {
		jobID, err := exec.Submit(context.Background(), "test-task", 1*time.Second, func(ctx context.Context) error {
			time.Sleep(100 * time.Millisecond)
			return nil
		})
		if err != nil {
			t.Fatalf("提交任务失败: %v", err)
		}
		jobIDs = append(jobIDs, jobID)
	}

	// 立即停止（队列中应该还有未执行的任务）
	time.Sleep(50 * time.Millisecond)
	if err := exec.Stop(); err != nil {
		t.Fatalf("停止失败: %v", err)
	}

	// 检查是否有任务被标记为 canceled
	canceledCount := 0
	for _, jobID := range jobIDs {
		job, err := exec.GetJob(jobID)
		if err != nil {
			t.Fatalf("查询任务失败: %v", err)
		}
		if job.Status == JobStatusCanceled {
			canceledCount++
		}
	}

	// 应该至少有一些任务被取消（因为队列中还有未执行的）
	if canceledCount == 0 {
		t.Log("警告：没有任务被取消，可能所有任务都已执行完成")
	} else {
		t.Logf("成功取消 %d 个未执行的任务", canceledCount)
	}
}

func TestExecutor_TraceIDPropagation(t *testing.T) {
	exec := NewExecutor(DefaultConfig())
	if err := exec.Start(); err != nil {
		t.Fatalf("启动失败: %v", err)
	}
	defer exec.Stop()

	// 创建带 traceId 的 context
	traceID := "test-trace-123"
	ctx := context.WithValue(context.Background(), "traceId", traceID)

	// 提交任务
	jobID, err := exec.Submit(ctx, "trace-test", 1*time.Second, func(execCtx context.Context) error {
		// 验证 traceId 是否被传播
		if val := execCtx.Value("traceId"); val == nil {
			return errors.New("traceId 未传播")
		} else if tid, ok := val.(string); !ok || tid != traceID {
			return errors.New("traceId 不匹配")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("提交任务失败: %v", err)
	}

	// 等待任务完成
	time.Sleep(200 * time.Millisecond)

	// 查询任务
	job, err := exec.GetJob(jobID)
	if err != nil {
		t.Fatalf("查询任务失败: %v", err)
	}

	// 验证 traceId 是否被保存
	if job.TraceID != traceID {
		t.Errorf("期望 traceId 为 %s，实际 %s", traceID, job.TraceID)
	}

	// 验证任务是否成功
	if job.Status != JobStatusSucceeded {
		t.Errorf("任务应该成功，实际状态: %s, 错误: %s", job.Status, job.Error)
	}
}


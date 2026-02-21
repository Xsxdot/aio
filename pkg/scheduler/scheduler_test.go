package scheduler

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

// TestLongTaskDoesNotBlockShortTask 测试长耗时任务不会阻塞后续短任务的调度
func TestLongTaskDoesNotBlockShortTask(t *testing.T) {
	// 创建调度器，配置足够的 worker 数量以支持并发
	config := &SchedulerConfig{
		NodeID:            "test-scheduler",
		LockKey:           "test/scheduler/leader",
		LockTTL:           30 * time.Second,
		LockRetryInterval: 5 * time.Second,
		MaxWorkers:        5, // 足够的 worker 让任务并发执行
	}
	scheduler := NewScheduler(config)

	err := scheduler.Start()
	if err != nil {
		t.Fatalf("启动调度器失败: %v", err)
	}
	defer scheduler.Stop()

	// 记录任务的实际执行时间
	var taskAStartTime, taskBStartTime atomic.Int64

	startTime := time.Now()

	// 任务A：立即执行，但要阻塞 800ms（模拟长任务）
	taskA := NewOnceTask(
		"long-task-a",
		time.Now(), // 立即执行
		TaskExecuteModeLocal,
		2*time.Second, // 超时时间
		func(ctx context.Context) error {
			taskAStartTime.Store(time.Now().UnixMilli())
			t.Logf("任务A开始执行: %v", time.Since(startTime))
			
			// 模拟长时间处理
			select {
			case <-time.After(800 * time.Millisecond):
				t.Logf("任务A执行完成: %v", time.Since(startTime))
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		},
	)

	// 任务B：100ms 后执行（在任务A执行期间应该被及时调度）
	taskB := NewOnceTask(
		"short-task-b",
		time.Now().Add(100*time.Millisecond),
		TaskExecuteModeLocal,
		2*time.Second,
		func(ctx context.Context) error {
			taskBStartTime.Store(time.Now().UnixMilli())
			elapsed := time.Since(startTime)
			t.Logf("任务B开始执行: %v", elapsed)
			return nil
		},
	)

	// 添加任务
	err = scheduler.AddTask(taskA)
	if err != nil {
		t.Fatalf("添加任务A失败: %v", err)
	}

	err = scheduler.AddTask(taskB)
	if err != nil {
		t.Fatalf("添加任务B失败: %v", err)
	}

	// 等待两个任务都执行完成（最多等待 2 秒）
	time.Sleep(1500 * time.Millisecond)

	// 检查任务B是否被及时执行
	taskBStart := taskBStartTime.Load()
	if taskBStart == 0 {
		t.Fatal("任务B未执行")
	}

	// 计算任务B的实际触发延迟
	taskBDelay := time.Duration(taskBStart-startTime.UnixMilli()) * time.Millisecond
	
	// 任务B应该在 100ms 左右执行（允许一定的调度抖动，比如 +50ms）
	expectedDelay := 100 * time.Millisecond
	maxAllowedDelay := 200 * time.Millisecond // 允许最多 200ms 的抖动

	t.Logf("任务B预期延迟: %v, 实际延迟: %v", expectedDelay, taskBDelay)

	if taskBDelay > maxAllowedDelay {
		t.Errorf("任务B被长任务A阻塞了！预期在 %v 内触发，实际延迟 %v", maxAllowedDelay, taskBDelay)
	}

	// 确保任务A也确实执行了
	taskAStart := taskAStartTime.Load()
	if taskAStart == 0 {
		t.Fatal("任务A未执行")
	}

	// 验证统计信息
	stats := scheduler.GetStats()
	if stats.CompletedTasks < 2 {
		t.Errorf("期望完成2个任务，实际完成 %d 个", stats.CompletedTasks)
	}
}

// TestMultipleTasksWithDifferentDelays 测试多个不同延迟的任务能按时触发
func TestMultipleTasksWithDifferentDelays(t *testing.T) {
	config := &SchedulerConfig{
		NodeID:            "test-scheduler-multi",
		LockKey:           "test/scheduler/multi",
		LockTTL:           30 * time.Second,
		LockRetryInterval: 5 * time.Second,
		MaxWorkers:        10,
	}
	scheduler := NewScheduler(config)

	err := scheduler.Start()
	if err != nil {
		t.Fatalf("启动调度器失败: %v", err)
	}
	defer scheduler.Stop()

	startTime := time.Now()
	var executionTimes [5]atomic.Int64

	// 添加一个长任务（立即执行，阻塞 1 秒）
	longTask := NewOnceTask(
		"long-task",
		time.Now(),
		TaskExecuteModeLocal,
		3*time.Second,
		func(ctx context.Context) error {
			executionTimes[0].Store(time.Now().UnixMilli())
			t.Logf("长任务开始执行: %v", time.Since(startTime))
			select {
			case <-time.After(1 * time.Second):
				t.Logf("长任务执行完成: %v", time.Since(startTime))
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		},
	)

	// 添加多个短任务，分别在不同时间触发
	delays := []time.Duration{50 * time.Millisecond, 100 * time.Millisecond, 200 * time.Millisecond, 400 * time.Millisecond}
	
	err = scheduler.AddTask(longTask)
	if err != nil {
		t.Fatalf("添加长任务失败: %v", err)
	}

	for i, delay := range delays {
		idx := i + 1 // executionTimes[0] 是长任务
		taskName := "short-task-" + string(rune('A'+i))
		
		task := NewOnceTask(
			taskName,
			time.Now().Add(delay),
			TaskExecuteModeLocal,
			2*time.Second,
			func(ctx context.Context) error {
				executionTimes[idx].Store(time.Now().UnixMilli())
				elapsed := time.Since(startTime)
				t.Logf("%s 执行: %v", taskName, elapsed)
				return nil
			},
		)
		
		err = scheduler.AddTask(task)
		if err != nil {
			t.Fatalf("添加任务 %s 失败: %v", taskName, err)
		}
	}

	// 等待所有任务完成
	time.Sleep(1500 * time.Millisecond)

	// 验证所有任务都执行了
	for i := 0; i < 5; i++ {
		if executionTimes[i].Load() == 0 {
			t.Errorf("任务 %d 未执行", i)
		}
	}

	// 验证短任务的执行时间符合预期（在长任务执行期间也能按时触发）
	baseTime := startTime.UnixMilli()
	for i, expectedDelay := range delays {
		idx := i + 1
		actualTime := executionTimes[idx].Load()
		if actualTime == 0 {
			continue
		}
		
		actualDelay := time.Duration(actualTime-baseTime) * time.Millisecond
		maxAllowedDelay := expectedDelay + 150*time.Millisecond // 允许 150ms 抖动
		
		t.Logf("任务 %d: 预期延迟 %v, 实际延迟 %v", i+1, expectedDelay, actualDelay)
		
		if actualDelay > maxAllowedDelay {
			t.Errorf("任务 %d 触发延迟过大: 预期 %v, 实际 %v", i+1, expectedDelay, actualDelay)
		}
	}

	// 验证统计信息
	stats := scheduler.GetStats()
	if stats.CompletedTasks < 5 {
		t.Errorf("期望完成5个任务，实际完成 %d 个", stats.CompletedTasks)
	}
}

// TestSchedulerStopsGracefully 测试调度器能优雅停止
func TestSchedulerStopsGracefully(t *testing.T) {
	config := DefaultSchedulerConfig()
	scheduler := NewScheduler(config)

	err := scheduler.Start()
	if err != nil {
		t.Fatalf("启动调度器失败: %v", err)
	}

	// 添加一个简单任务
	var executed atomic.Bool
	task := NewOnceTask(
		"simple-task",
		time.Now(),
		TaskExecuteModeLocal,
		1*time.Second,
		func(ctx context.Context) error {
			executed.Store(true)
			return nil
		},
	)

	err = scheduler.AddTask(task)
	if err != nil {
		t.Fatalf("添加任务失败: %v", err)
	}

	// 等待任务执行
	time.Sleep(100 * time.Millisecond)

	// 停止调度器
	err = scheduler.Stop()
	if err != nil {
		t.Fatalf("停止调度器失败: %v", err)
	}

	// 验证任务已执行
	if !executed.Load() {
		t.Error("任务未执行")
	}

	// 验证调度器已停止
	if scheduler.isRunning.Load() {
		t.Error("调度器仍在运行")
	}
}

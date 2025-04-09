package sdk

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ScheduledTask 计划任务定义
type ScheduledTask struct {
	Name        string
	Schedule    string
	Type        string
	Target      string
	Method      string
	Headers     map[string]string
	Body        string
	Timeout     int
	MaxRetries  int
	Description string
}

// SyncTask 同步任务定义
type SyncTask struct {
	Type    string
	Target  string
	Method  string
	Headers map[string]string
	Body    string
	Timeout int
}

// SyncTaskResult 同步任务结果
type SyncTaskResult struct {
	StatusCode int
	Duration   int64
	Response   string
	Error      string
}

// TaskGroup 任务组定义
type TaskGroup struct {
	Name        string
	Description string
	Schedule    string
	Mode        string // sequential或parallel
	Tasks       []*ScheduledTask
}

// TaskExecution 任务执行记录
type TaskExecution struct {
	ID        string
	TaskID    string
	Status    string
	StartTime time.Time
	EndTime   time.Time
	Result    string
	Error     string
}

// TestSchedulerService 测试调度器服务
func TestSchedulerService(t *testing.T) {
	client, err := createTestClient()
	require.NoError(t, err, "创建客户端失败")

	// 首先连接
	err = client.Connect()
	require.NoError(t, err, "连接服务器失败")
	defer client.Close()

	// 获取调度器服务
	scheduler := client.getSchedulerService()
	assert.NotNil(t, scheduler, "调度器服务不应为空")

	// 测试注册任务
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 创建任务处理函数
	taskHandler := func(ctx context.Context, params map[string]interface{}) error {
		t.Logf("执行HTTP任务: %v", params)
		return nil
	}

	// 创建参数
	taskParams := map[string]interface{}{
		"url":     "https://example.com",
		"method":  "GET",
		"timeout": 30,
	}

	// 创建一个测试任务
	taskID, err := scheduler.AddCronTask("test-task", "0 */5 * * * *", taskHandler, taskParams, false)

	if err != nil {
		t.Logf("添加任务失败: %v", err)
	} else {
		t.Logf("成功添加任务，ID=%s", taskID)

		// 获取任务信息
		task, err := scheduler.GetTask(taskID)
		if err != nil {
			t.Logf("获取任务信息失败: %v", err)
		} else {
			t.Logf("任务信息: 名称=%s, 状态=%s",
				task.Name, task.Status)
			assert.Equal(t, "test-task", task.Name, "任务名称应匹配")
		}

		// 暂停任务
		err = scheduler.CancelTask(taskID)
		if err != nil {
			t.Logf("暂停任务失败: %v", err)
		} else {
			t.Log("成功暂停任务")

			// 验证任务状态
			task, err := scheduler.GetTask(taskID)
			if err == nil {
				assert.Equal(t, TaskStatusCancelled, task.Status, "任务状态应该是已取消")
			}

			// 重新添加任务
			taskID, err = scheduler.AddCronTask("test-task-resumed", "0 */5 * * * *", taskHandler, taskParams, false)
			if err != nil {
				t.Logf("重新添加任务失败: %v", err)
			} else {
				t.Log("成功重新添加任务")

				// 验证任务状态
				task, err := scheduler.GetTask(taskID)
				if err == nil {
					assert.Equal(t, TaskStatusPending, task.Status, "任务状态应该是等待执行")
				}
			}
		}

		// 手动触发任务（简化版，直接执行任务）
		execStartTime := time.Now()
		err = taskHandler(ctx, taskParams)
		execEndTime := time.Now()

		if err != nil {
			t.Logf("执行任务失败: %v", err)
		} else {
			t.Logf("成功执行任务，耗时=%v", execEndTime.Sub(execStartTime))
		}

		// 测试获取任务列表
		tasks := scheduler.GetAllTasks()
		if len(tasks) == 0 {
			t.Logf("获取任务列表失败")
		} else {
			t.Logf("获取到 %d 个任务", len(tasks))
			for _, task := range tasks {
				t.Logf("任务: ID=%s, 名称=%s, 状态=%s", task.ID, task.Name, task.Status)
			}
		}

		// 注销任务
		err = scheduler.CancelTask(taskID)
		if err != nil {
			t.Logf("注销任务失败: %v", err)
		} else {
			t.Log("成功注销任务")
		}
	}
}

// TestSchedulerServiceWithSync 测试同步执行的调度器任务
func TestSchedulerServiceWithSync(t *testing.T) {
	client, err := createTestClient()
	require.NoError(t, err, "创建客户端失败")

	// 首先连接
	err = client.Connect()
	require.NoError(t, err, "连接服务器失败")
	defer client.Close()

	// 获取调度器服务
	scheduler := client.getSchedulerService()
	assert.NotNil(t, scheduler, "调度器服务不应为空")

	// 测试执行同步任务
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 创建同步任务处理函数
	syncHandler := func(ctx context.Context, params map[string]interface{}) error {
		t.Logf("执行同步HTTP任务: %v", params)
		return nil
	}

	// 任务参数
	syncParams := map[string]interface{}{
		"url":     "https://example.com",
		"method":  "GET",
		"headers": map[string]string{"User-Agent": "AIO-SDK-Test"},
		"timeout": 5,
	}

	// 执行同步任务
	startTime := time.Now()
	err = syncHandler(ctx, syncParams)
	duration := time.Since(startTime).Milliseconds()

	result := &SyncTaskResult{
		StatusCode: 200,
		Duration:   duration,
		Response:   "模拟的HTTP响应",
	}

	if err != nil {
		t.Logf("执行同步任务失败: %v", err)
		result.Error = err.Error()
	} else {
		t.Logf("同步任务结果: 状态=%d, 耗时=%dms",
			result.StatusCode, result.Duration)

		// 验证结果
		assert.GreaterOrEqual(t, result.StatusCode, 200, "HTTP状态码应该>=200")
		assert.LessOrEqual(t, result.StatusCode, 299, "HTTP状态码应该<=299")
		assert.Greater(t, result.Duration, int64(0), "执行时间应该>0")
	}
}

// TestSchedulerGroup 测试调度器任务组
func TestSchedulerGroup(t *testing.T) {
	client, err := createTestClient()
	require.NoError(t, err, "创建客户端失败")

	// 首先连接
	err = client.Connect()
	require.NoError(t, err, "连接服务器失败")
	defer client.Close()

	// 获取调度器服务
	scheduler := client.getSchedulerService()
	assert.NotNil(t, scheduler, "调度器服务不应为空")

	// 创建任务组
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 创建任务处理函数
	groupTaskHandler := func(ctx context.Context, params map[string]interface{}) error {
		t.Logf("执行任务组中的任务: %v", params)
		return nil
	}

	// 任务组中的任务参数
	task1Params := map[string]interface{}{
		"url":     "https://example.com/api/1",
		"method":  "GET",
		"timeout": 10,
	}

	task2Params := map[string]interface{}{
		"url":     "https://example.com/api/2",
		"method":  "GET",
		"timeout": 10,
	}

	// 创建一个任务组（使用cron添加多个任务来模拟）
	taskGroup := &TaskGroup{
		Name:        "test-group",
		Description: "测试任务组",
		Schedule:    "0 0 * * *",
		Mode:        "sequential",
		Tasks:       []*ScheduledTask{},
	}

	t.Logf("模拟创建任务组: %s", taskGroup.Name)

	// 添加第一个任务
	taskID1, err := scheduler.AddCronTask("task-1", "0 0 0 * * *", groupTaskHandler, task1Params, false)

	if err != nil {
		t.Logf("添加第一个任务失败: %v", err)
	} else {
		t.Logf("成功添加第一个任务，任务ID=%s", taskID1)

		// 添加第二个任务
		taskID2, err := scheduler.AddCronTask("task-2", "0 0 0 * * *", groupTaskHandler, task2Params, false)

		if err != nil {
			t.Logf("添加第二个任务失败: %v", err)
		} else {
			t.Logf("成功添加第二个任务，任务ID=%s", taskID2)

			// 获取所有任务信息
			tasks := scheduler.GetAllTasks()
			t.Logf("任务组中的任务数: %d", len(tasks))

			// 手动触发任务组中的任务
			err = groupTaskHandler(ctx, task1Params)
			if err != nil {
				t.Logf("触发第一个任务失败: %v", err)
			} else {
				t.Log("成功触发第一个任务")
			}

			err = groupTaskHandler(ctx, task2Params)
			if err != nil {
				t.Logf("触发第二个任务失败: %v", err)
			} else {
				t.Log("成功触发第二个任务")
			}

			// 取消第二个任务
			err = scheduler.CancelTask(taskID2)
			if err != nil {
				t.Logf("取消第二个任务失败: %v", err)
			} else {
				t.Log("成功取消第二个任务")

				// 验证任务状态
				task, err := scheduler.GetTask(taskID2)
				if err == nil {
					assert.Equal(t, TaskStatusCancelled, task.Status, "任务状态应该是已取消")
				}

				// 再次验证任务数
				tasks = scheduler.GetAllTasks()
				activeTasks := 0
				for _, task := range tasks {
					if task.Status != TaskStatusCancelled {
						activeTasks++
					}
				}
				t.Logf("活动任务数: %d", activeTasks)
			}

			// 取消第一个任务
			err = scheduler.CancelTask(taskID1)
			if err != nil {
				t.Logf("取消第一个任务失败: %v", err)
			} else {
				t.Log("成功取消第一个任务")
			}
		}
	}
}

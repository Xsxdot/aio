package scheduler

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/xsxdot/aio/pkg/lock"
)

// ExampleUsage 演示如何使用分布式任务调度器
func ExampleUsage() {
	// 注意：这里需要传入实际的 LockManager 实例
	// lockManager := etcd.NewLockManager(etcdClient)
	var lockManager lock.LockManager // 实际使用时需要传入真实的实例

	// 创建调度器配置
	config := &SchedulerConfig{
		NodeID:        "node-1",
		LockKey:       "aio/scheduler/leader",
		LockTTL:       30 * time.Second,
		CheckInterval: 1 * time.Second,
		MaxWorkers:    5,
	}

	// 创建调度器
	scheduler := NewScheduler(lockManager, config)

	// 启动调度器
	if err := scheduler.Start(); err != nil {
		log.Fatalf("启动调度器失败: %v", err)
	}
	defer scheduler.Stop()

	// 创建一次性任务（不需要传入ID，自动生成UUID）
	onceTask := NewOnceTask(
		"一次性任务示例",
		time.Now().Add(5*time.Second),
		TaskExecuteModeDistributed, // 分布式任务
		10*time.Second,
		func(ctx context.Context) error {
			fmt.Println("执行一次性分布式任务")
			return nil
		},
	)

	// 创建固定间隔任务
	intervalTask := NewIntervalTask(
		"固定间隔任务示例",
		time.Now().Add(2*time.Second),
		10*time.Second,       // 每10秒执行一次
		TaskExecuteModeLocal, // 本地任务
		5*time.Second,
		func(ctx context.Context) error {
			fmt.Println("执行固定间隔本地任务")
			return nil
		},
	)

	// 创建Cron任务
	cronTask, err := NewCronTask(
		"Cron任务示例",
		"0 */1 * * * *",            // 每分钟执行一次
		TaskExecuteModeDistributed, // 分布式任务
		30*time.Second,
		func(ctx context.Context) error {
			fmt.Println("执行Cron分布式任务")
			return nil
		},
	)
	if err != nil {
		log.Fatalf("创建Cron任务失败: %v", err)
	}

	// 添加任务到调度器
	scheduler.AddTask(onceTask)
	scheduler.AddTask(intervalTask)
	scheduler.AddTask(cronTask)

	// 打印生成的任务ID
	fmt.Printf("一次性任务ID: %s\n", onceTask.GetID())
	fmt.Printf("固定间隔任务ID: %s\n", intervalTask.GetID())
	fmt.Printf("Cron任务ID: %s\n", cronTask.GetID())

	// 监控调度器状态
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				stats := scheduler.GetStats()
				fmt.Printf("调度器状态 - 总任务: %d, 已完成: %d, 失败: %d, 分布式: %d, 本地: %d, 是否为领导者: %t\n",
					stats.TotalTasks, stats.CompletedTasks, stats.FailedTasks,
					stats.DistributedTasks, stats.LocalTasks, scheduler.IsLeader())
			}
		}
	}()

	// 运行一段时间后停止
	time.Sleep(60 * time.Second)
}

// ExampleTaskTypes 演示不同类型任务的创建
func ExampleTaskTypes() {
	// 1. 创建一次性任务（自动生成UUID）
	onceTask := NewOnceTask(
		"数据库备份任务",
		time.Now().Add(1*time.Hour), // 1小时后执行
		TaskExecuteModeDistributed,  // 分布式执行
		30*time.Minute,              // 30分钟超时
		func(ctx context.Context) error {
			// 执行数据库备份逻辑
			fmt.Println("开始执行数据库备份...")
			time.Sleep(2 * time.Second) // 模拟备份过程
			fmt.Println("数据库备份完成")
			return nil
		},
	)

	// 2. 创建固定间隔任务（自动生成UUID）
	healthCheckTask := NewIntervalTask(
		"服务健康检查",
		time.Now().Add(10*time.Second), // 10秒后开始执行
		30*time.Second,                 // 每30秒执行一次
		TaskExecuteModeLocal,           // 本地执行
		5*time.Second,                  // 5秒超时
		func(ctx context.Context) error {
			// 执行健康检查逻辑
			fmt.Println("执行健康检查...")
			return nil
		},
	)

	// 3. 创建Cron任务（自动生成UUID）
	reportTask, err := NewCronTask(
		"每日报表生成",
		"0 0 2 * * *",              // 每天凌晨2点执行
		TaskExecuteModeDistributed, // 分布式执行
		1*time.Hour,                // 1小时超时
		func(ctx context.Context) error {
			// 执行报表生成逻辑
			fmt.Println("开始生成每日报表...")
			time.Sleep(5 * time.Second) // 模拟报表生成过程
			fmt.Println("每日报表生成完成")
			return nil
		},
	)
	if err != nil {
		log.Printf("创建Cron任务失败: %v", err)
		return
	}

	// 输出任务信息（显示自动生成的UUID）
	fmt.Printf("一次性任务: %s [ID: %s], 下次执行时间: %s\n",
		onceTask.GetName(), onceTask.GetID(), onceTask.GetNextTime().Format("2006-01-02 15:04:05"))
	fmt.Printf("固定间隔任务: %s [ID: %s], 下次执行时间: %s\n",
		healthCheckTask.GetName(), healthCheckTask.GetID(), healthCheckTask.GetNextTime().Format("2006-01-02 15:04:05"))
	fmt.Printf("Cron任务: %s [ID: %s], 下次执行时间: %s\n",
		reportTask.GetName(), reportTask.GetID(), reportTask.GetNextTime().Format("2006-01-02 15:04:05"))
}

// ExampleTaskManagement 演示任务管理功能
func ExampleTaskManagement() {
	var lockManager lock.LockManager // 实际使用时需要传入真实的实例
	scheduler := NewScheduler(lockManager, DefaultSchedulerConfig())
	scheduler.Start()
	defer scheduler.Stop()

	// 创建一个任务
	task := NewOnceTask(
		"示例管理任务",
		time.Now().Add(1*time.Hour),
		TaskExecuteModeLocal,
		10*time.Minute,
		func(ctx context.Context) error {
			fmt.Println("执行示例任务")
			return nil
		},
	)

	// 添加任务
	scheduler.AddTask(task)
	fmt.Printf("添加任务: %s [ID: %s]\n", task.GetName(), task.GetID())

	// 获取任务信息
	retrievedTask := scheduler.GetTask(task.GetID())
	if retrievedTask != nil {
		fmt.Printf("找到任务: %s, 状态: %d\n", retrievedTask.GetName(), retrievedTask.GetStatus())
	}

	// 列出所有任务
	allTasks := scheduler.ListTasks()
	fmt.Printf("总共有 %d 个任务\n", len(allTasks))

	// 移除任务
	removed := scheduler.RemoveTask(task.GetID())
	if removed {
		fmt.Printf("任务 %s 已移除\n", task.GetID())
	}

	// 获取统计信息
	stats := scheduler.GetStats()
	fmt.Printf("统计信息: 总任务=%d, 已完成=%d, 失败=%d\n",
		stats.TotalTasks, stats.CompletedTasks, stats.FailedTasks)
}

// ExampleDistributedExecution 演示分布式执行的行为
func ExampleDistributedExecution() {
	fmt.Println("分布式任务调度器说明:")
	fmt.Println("1. 调度器启动后会尝试获取分布式锁")
	fmt.Println("2. 获取到锁的节点成为领导者，可以执行分布式任务")
	fmt.Println("3. 没有获取到锁的节点只能执行本地任务")
	fmt.Println("4. 所有任务使用统一的任务堆和单一定时器管理")
	fmt.Println("5. 支持任务超时、重试和故障转移")
	fmt.Println()

	fmt.Println("任务类型说明:")
	fmt.Println("- TaskExecuteModeDistributed: 分布式任务，只有领导者节点执行")
	fmt.Println("- TaskExecuteModeLocal: 本地任务，所有节点都会执行")
	fmt.Println()

	fmt.Println("任务调度说明:")
	fmt.Println("- 使用最小堆按执行时间排序任务")
	fmt.Println("- 只使用一个定时器，避免多定时器的开销")
	fmt.Println("- 支持工作者池限制并发执行的任务数量")
	fmt.Println("- 提供详细的统计信息和日志记录")
	fmt.Println("- 任务ID使用UUID自动生成，保证全局唯一性")
}

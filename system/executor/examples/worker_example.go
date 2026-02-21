package examples

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	pb "github.com/xsxdot/aio/system/executor/api/proto"
	"google.golang.org/grpc"
)

// WorkerExample Worker 示例代码
// 展示如何创建一个 Worker 来拉取并执行任务

// EmailNotificationArgs 邮件通知参数
type EmailNotificationArgs struct {
	UserID int    `json:"user_id"`
	Email  string `json:"email"`
	Title  string `json:"title"`
	Body   string `json:"body"`
}

// PaymentArgs 支付参数
type PaymentArgs struct {
	OrderID string  `json:"order_id"`
	Amount  float64 `json:"amount"`
}

// SimpleWorker 简单的 Worker 实现
type SimpleWorker struct {
	client      pb.ExecutorServiceClient
	serviceName string
	method      string // 可选：指定只处理特定方法的任务
	consumerID  string
}

// NewSimpleWorker 创建 Worker 实例
func NewSimpleWorker(grpcAddr, serviceName, consumerID string) (*SimpleWorker, error) {
	// 连接到 gRPC 服务器
	conn, err := grpc.Dial(grpcAddr, grpc.WithInsecure())
	if err != nil {
		return nil, fmt.Errorf("连接 gRPC 服务器失败: %w", err)
	}

	client := pb.NewExecutorServiceClient(conn)

	return &SimpleWorker{
		client:      client,
		serviceName: serviceName,
		consumerID:  consumerID,
	}, nil
}

// NewMethodWorker 创建只处理特定方法的 Worker 实例
func NewMethodWorker(grpcAddr, serviceName, method, consumerID string) (*SimpleWorker, error) {
	// 连接到 gRPC 服务器
	conn, err := grpc.Dial(grpcAddr, grpc.WithInsecure())
	if err != nil {
		return nil, fmt.Errorf("连接 gRPC 服务器失败: %w", err)
	}

	client := pb.NewExecutorServiceClient(conn)

	return &SimpleWorker{
		client:      client,
		serviceName: serviceName,
		method:      method,
		consumerID:  consumerID,
	}, nil
}

// Start 启动 Worker 主循环
func (w *SimpleWorker) Start(ctx context.Context) {
	if w.method != "" {
		log.Printf("Worker 启动: service=%s, method=%s, consumer=%s", w.serviceName, w.method, w.consumerID)
	} else {
		log.Printf("Worker 启动: service=%s (所有方法), consumer=%s", w.serviceName, w.consumerID)
	}

	for {
		select {
		case <-ctx.Done():
			log.Println("Worker 停止")
			return
		default:
			w.processOneJob(ctx)
		}
	}
}

// processOneJob 处理一个任务
func (w *SimpleWorker) processOneJob(ctx context.Context) {
	// 1. 领取任务（可选按 method 过滤）
	resp, err := w.client.AcquireJob(ctx, &pb.AcquireJobRequest{
		TargetService: w.serviceName,
		Method:        w.method,      // 指定方法名过滤，空表示领取所有方法的任务
		ConsumerId:    w.consumerID,
		LeaseDuration: 30, // 30秒租约
	})

	if err != nil {
		log.Printf("领取任务失败: %v", err)
		time.Sleep(5 * time.Second)
		return
	}

	// 没有任务，等待后重试
	if resp.JobId == 0 {
		time.Sleep(5 * time.Second)
		return
	}

	log.Printf("领取任务成功: job_id=%d, method=%s, attempt=%d", resp.JobId, resp.Method, resp.AttemptNo)

	// 2. 执行任务
	err = w.executeMethod(resp.Method, resp.ArgsJson)

	// 3. 确认任务结果
	var status pb.JobStatus
	var errorMsg string

	if err != nil {
		log.Printf("任务执行失败: job_id=%d, error=%v", resp.JobId, err)
		status = pb.JobStatus_JOB_STATUS_FAILED
		errorMsg = err.Error()
	} else {
		log.Printf("任务执行成功: job_id=%d", resp.JobId)
		status = pb.JobStatus_JOB_STATUS_SUCCEEDED
	}

	_, err = w.client.AckJob(ctx, &pb.AckJobRequest{
		JobId:      resp.JobId,
		AttemptNo:  resp.AttemptNo,
		ConsumerId: w.consumerID,
		Status:     status,
		Error:      errorMsg,
	})

	if err != nil {
		log.Printf("确认任务失败: job_id=%d, error=%v", resp.JobId, err)
	}
}

// executeMethod 执行具体的方法
func (w *SimpleWorker) executeMethod(method, argsJSON string) error {
	switch method {
	case "SendEmailNotification":
		return w.sendEmailNotification(argsJSON)
	case "ProcessPayment":
		return w.processPayment(argsJSON)
	default:
		return fmt.Errorf("未知方法: %s", method)
	}
}

// sendEmailNotification 发送邮件通知
func (w *SimpleWorker) sendEmailNotification(argsJSON string) error {
	var args EmailNotificationArgs
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return fmt.Errorf("解析参数失败: %w", err)
	}

	// 模拟发送邮件
	log.Printf("发送邮件: user_id=%d, email=%s, title=%s", args.UserID, args.Email, args.Title)
	time.Sleep(1 * time.Second) // 模拟耗时操作

	return nil
}

// processPayment 处理支付
func (w *SimpleWorker) processPayment(argsJSON string) error {
	var args PaymentArgs
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return fmt.Errorf("解析参数失败: %w", err)
	}

	// 模拟处理支付
	log.Printf("处理支付: order_id=%s, amount=%.2f", args.OrderID, args.Amount)
	time.Sleep(2 * time.Second) // 模拟耗时操作

	return nil
}

// 使用示例
func ExampleUsage() {
	// 示例 1: 创建通用 Worker（处理所有方法）
	worker, err := NewSimpleWorker(
		"localhost:50051",   // gRPC 服务器地址
		"user-service",      // 服务名（只处理该服务的任务）
		"worker-instance-1", // Worker 实例 ID
	)
	if err != nil {
		log.Fatal(err)
	}

	// 启动 Worker
	ctx := context.Background()
	worker.Start(ctx)
}

// ExampleMethodSpecificWorker 示例：创建只处理特定方法的 Worker
func ExampleMethodSpecificWorker() {
	// 示例 2: 创建只处理 SendEmailNotification 方法的 Worker
	emailWorker, err := NewMethodWorker(
		"localhost:50051",        // gRPC 服务器地址
		"user-service",           // 服务名
		"SendEmailNotification",  // 只处理邮件通知任务
		"email-worker-1",         // Worker 实例 ID
	)
	if err != nil {
		log.Fatal(err)
	}

	// 启动邮件 Worker
	ctx := context.Background()
	go emailWorker.Start(ctx)

	// 示例 3: 创建只处理 ProcessPayment 方法的 Worker
	paymentWorker, err := NewMethodWorker(
		"localhost:50051",   // gRPC 服务器地址
		"user-service",      // 服务名
		"ProcessPayment",    // 只处理支付任务
		"payment-worker-1",  // Worker 实例 ID
	)
	if err != nil {
		log.Fatal(err)
	}

	// 启动支付 Worker
	paymentWorker.Start(ctx)
}

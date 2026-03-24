package utils

import (
	"context"
	"fmt"
	"runtime/debug"
	"sync"
)

// ConcurrencyController 通用并发控制器
// 支持信号量控制最大并发数、上下文取消、错误隔离、Panic恢复
type ConcurrencyController struct {
	maxWorkers int
	semaphore  chan struct{}
}

// TaskResult 任务执行结果
type TaskResult[T any] struct {
	Index  int // 原始索引，便于排序
	Result T
	Error  error
}

// NewConcurrencyController 创建并发控制器
// maxWorkers: 最大并发数，若 <= 0 则不限制并发
func NewConcurrencyController(maxWorkers int) *ConcurrencyController {
	if maxWorkers <= 0 {
		// 不限制并发时，semaphore 为 nil
		return &ConcurrencyController{maxWorkers: 0, semaphore: nil}
	}
	return &ConcurrencyController{
		maxWorkers: maxWorkers,
		semaphore:  make(chan struct{}, maxWorkers),
	}
}

// Run 并发执行任务
// items: 待处理的任务输入列表
// fn: 任务处理函数，接收上下文和单个输入，返回结果和错误
// 返回: 结果列表，按原始索引排列（需调用方按需排序）
//
// 特性：
//   - 信号量控制最大并发数
//   - 上下文取消支持：当 ctx 被取消时，未开始的任务将被跳过
//   - 错误隔离与防崩：单个任务失败或 panic 不会阻断其他任务，不会导致程序崩溃
//   - 无锁并发写入：利用预分配切片独立索引特性，去除了 Mutex 锁，提升吞吐量
func Run[T any, R any](c *ConcurrencyController, ctx context.Context, items []T, fn func(context.Context, T) (R, error)) []TaskResult[R] {
	if len(items) == 0 {
		return nil
	}

	results := make([]TaskResult[R], len(items))
	var wg sync.WaitGroup

	for i, item := range items {
		// 检查上下文是否已取消
		select {
		case <-ctx.Done():
			// 上下文已取消，将未处理的任务标记为取消错误
			results[i] = TaskResult[R]{Index: i, Error: ctx.Err()}
			continue
		default:
		}

		// 获取信号量（若配置了并发限制）
		if c.semaphore != nil {
			select {
			case c.semaphore <- struct{}{}:
				// 成功获取信号量
			case <-ctx.Done():
				// 上下文取消，跳过此任务
				results[i] = TaskResult[R]{Index: i, Error: ctx.Err()}
				continue
			}
		}

		wg.Add(1)
		go func(index int, item T) {
			defer wg.Done()

			// 1. 释放信号量
			defer func() {
				if c.semaphore != nil {
					<-c.semaphore // 释放信号量
				}
			}()

			// 2. 捕获 Panic，防止整个服务崩溃，并记录堆栈便于排查
			defer func() {
				if p := recover(); p != nil {
					err := fmt.Errorf("task panicked: %v\n%s", p, debug.Stack())
					// 无锁安全写入
					results[index] = TaskResult[R]{
						Index: index,
						Error: err,
					}
				}
			}()

			// 再次检查上下文（任务可能在排队时被取消）
			select {
			case <-ctx.Done():
				// 无锁安全写入
				results[index] = TaskResult[R]{Index: index, Error: ctx.Err()}
				return
			default:
			}

			// 执行任务
			result, err := fn(ctx, item)

			// 无锁安全写入
			results[index] = TaskResult[R]{
				Index:  index,
				Result: result,
				Error:  err,
			}
		}(i, item)
	}

	wg.Wait()
	return results
}

// RunWithResults 并发执行任务并收集成功结果
// 返回: 成功结果列表（按原始索引排序）和错误列表
func RunWithResults[T any, R any](c *ConcurrencyController, ctx context.Context, items []T, fn func(context.Context, T) (R, error)) ([]R, []error) {
	results := Run(c, ctx, items, fn)

	var successes []R
	var errors []error

	for _, r := range results {
		if r.Error != nil {
			errors = append(errors, r.Error)
		} else {
			successes = append(successes, r.Result)
		}
	}

	return successes, errors
}

// RunAllOrError 并发执行任务，若有任一失败则返回第一个错误 (Fast-fail 机制)
// 返回: 所有成功结果（按原始索引排序）或第一个错误
func RunAllOrError[T any, R any](c *ConcurrencyController, ctx context.Context, items []T, fn func(context.Context, T) (R, error)) ([]R, error) {
	// 创建一个可取消的上下文，用于实现快速失败
	cancelCtx, cancel := context.WithCancel(ctx)
	defer cancel() // 确保退出时释放资源

	// 包装原始处理函数
	wrappedFn := func(ctx context.Context, item T) (R, error) {
		res, err := fn(ctx, item)
		if err != nil {
			// 一旦遇到错误，立即取消其他排队或监听 ctx 的任务
			cancel()
		}
		return res, err
	}

	results := Run(c, cancelCtx, items, wrappedFn)

	successes := make([]R, 0, len(items))
	for _, r := range results {
		if r.Error != nil {
			// 如果有错误直接返回（可能就是触发 cancel 的那个原始错误）
			return nil, r.Error
		}
		successes = append(successes, r.Result)
	}

	return successes, nil
}

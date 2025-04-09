package utils

import (
	"crypto/rand"
	"fmt"
	"github.com/xsxdot/aio/pkg/common"
	"io"
	"time"
)

// GenerateUUID 生成UUID
func GenerateUUID() string {
	uuid := make([]byte, 16)
	_, err := io.ReadFull(rand.Reader, uuid)
	if err != nil {
		// 如果随机数生成失败，则使用时间戳作为备选
		timestamp := time.Now().UnixNano()
		for i := 0; i < 16; i++ {
			uuid[i] = byte(timestamp >> uint(i*8))
		}
	}

	// 设置版本 (4) 和变体 (8, 9, A, 或 B)
	uuid[6] = (uuid[6] & 0x0F) | 0x40 // 版本 4
	uuid[8] = (uuid[8] & 0x3F) | 0x80 // 变体 RFC4122

	return fmt.Sprintf("%x-%x-%x-%x-%x", uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:16])
}

// IsEmpty 检查字符串是否为空
func IsEmpty(s string) bool {
	return len(s) == 0
}

// DefaultIfEmpty 如果字符串为空，则返回默认值
func DefaultIfEmpty(s, defaultValue string) string {
	if IsEmpty(s) {
		return defaultValue
	}
	return s
}

// Contains 检查切片是否包含指定元素
func Contains[T comparable](slice []T, element T) bool {
	for _, item := range slice {
		if item == element {
			return true
		}
	}
	return false
}

// Map 对切片中的每个元素应用函数
func Map[T, U any](slice []T, fn func(T) U) []U {
	result := make([]U, len(slice))
	for i, item := range slice {
		result[i] = fn(item)
	}
	return result
}

// Filter 过滤切片中的元素
func Filter[T any](slice []T, fn func(T) bool) []T {
	var result []T
	for _, item := range slice {
		if fn(item) {
			result = append(result, item)
		}
	}
	return result
}

// Find 查找切片中的元素
func Find[T any](slice []T, fn func(T) bool) (T, bool) {
	for _, item := range slice {
		if fn(item) {
			return item, true
		}
	}
	var zero T
	return zero, false
}

// Reduce 归约切片
func Reduce[T, U any](slice []T, initial U, fn func(U, T) U) U {
	result := initial
	for _, item := range slice {
		result = fn(result, item)
	}
	return result
}

// SafeGo 安全地启动一个goroutine，捕获并记录panic
func SafeGo(fn func(), logger *common.Logger) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				if logger != nil {
					logger.Errorf("Goroutine panic: %v", r)
				}
			}
		}()
		fn()
	}()
}

// RetryWithBackoff 使用指数退避重试函数
func RetryWithBackoff(fn func() error, maxRetries int, initialBackoff time.Duration) error {
	var err error
	backoff := initialBackoff

	for i := 0; i < maxRetries; i++ {
		err = fn()
		if err == nil {
			return nil
		}

		// 如果不是最后一次尝试，则等待并重试
		if i < maxRetries-1 {
			time.Sleep(backoff)
			backoff *= 2 // 指数退避
		}
	}

	return err
}

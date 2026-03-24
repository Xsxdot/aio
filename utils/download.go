package utils

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// DownloadFile 从 URL 下载文件并保存到本地路径
// 支持 http/https 协议，支持代理配置
// 使用流式写入支持大文件下载
func DownloadFile(ctx context.Context, url string, localPath string, proxy string) error {
	// 构建带代理的 HTTP 客户端
	httpClient, err := BuildHTTPClientWithProxy(proxy, 0)
	if err != nil {
		return fmt.Errorf("构建 HTTP 客户端失败: %w", err)
	}

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}

	// 发送请求
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("下载文件失败: %w", err)
	}
	defer resp.Body.Close()

	// 检查响应状态码
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("下载文件失败: HTTP %d", resp.StatusCode)
	}

	// 确保目标目录存在
	dir := filepath.Dir(localPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	// 创建目标文件
	outFile, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("创建文件失败: %w", err)
	}
	defer outFile.Close()

	// 流式写入文件
	_, err = io.Copy(outFile, resp.Body)
	if err != nil {
		return fmt.Errorf("写入文件失败: %w", err)
	}

	return nil
}

// DownloadFileWithTimeout 从 URL 下载文件并保存到本地路径（带超时控制）
func DownloadFileWithTimeout(ctx context.Context, url string, localPath string, proxy string, timeout time.Duration) error {
	// 构建带代理和超时的 HTTP 客户端
	httpClient, err := BuildHTTPClientWithProxy(proxy, timeout)
	if err != nil {
		return fmt.Errorf("构建 HTTP 客户端失败: %w", err)
	}

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}

	// 发送请求
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("下载文件失败: %w", err)
	}
	defer resp.Body.Close()

	// 检查响应状态码
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("下载文件失败: HTTP %d", resp.StatusCode)
	}

	// 确保目标目录存在
	dir := filepath.Dir(localPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	// 创建目标文件
	outFile, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("创建文件失败: %w", err)
	}
	defer outFile.Close()

	// 流式写入文件
	_, err = io.Copy(outFile, resp.Body)
	if err != nil {
		return fmt.Errorf("写入文件失败: %w", err)
	}

	return nil
}

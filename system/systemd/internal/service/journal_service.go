package service

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	errorc "xiaozhizhang/pkg/core/err"
	"xiaozhizhang/pkg/core/logger"
)

const (
	// DefaultLogLines 默认返回日志行数
	DefaultLogLines = 200
	// MaxLogLines 最大返回日志行数
	MaxLogLines = 10000
)

// JournalService journalctl 日志服务
type JournalService struct {
	timeout time.Duration
	log     *logger.Log
	err     *errorc.ErrorBuilder
}

// NewJournalService 创建 journal 服务
func NewJournalService(timeout time.Duration, log *logger.Log) *JournalService {
	if timeout == 0 {
		timeout = DefaultCommandTimeout
	}
	return &JournalService{
		timeout: timeout,
		log:     log.WithEntryName("JournalService"),
		err:     errorc.NewErrorBuilder("JournalService"),
	}
}

// checkPlatform 检查平台是否为 Linux
func (s *JournalService) checkPlatform() error {
	if runtime.GOOS != "linux" {
		return s.err.New(fmt.Sprintf("journalctl 仅支持 Linux 平台，当前平台: %s", runtime.GOOS), nil).ValidWithCtx()
	}
	return nil
}

// GetLogs 获取服务日志
// lines: 返回行数（默认 200，最大 10000）
// since: 起始时间（如 "2024-01-01" 或 "1h ago"）
// until: 结束时间
func (s *JournalService) GetLogs(ctx context.Context, unitName string, lines int, since, until string) ([]string, error) {
	if err := s.checkPlatform(); err != nil {
		return nil, err
	}

	// 设置默认行数
	if lines <= 0 {
		lines = DefaultLogLines
	}
	if lines > MaxLogLines {
		lines = MaxLogLines
	}

	// 构建命令参数
	args := []string{
		"-u", unitName,
		"--no-pager",
		"-n", fmt.Sprintf("%d", lines),
		"--output=short-iso",
	}

	if since != "" {
		args = append(args, "--since", since)
	}
	if until != "" {
		args = append(args, "--until", until)
	}

	cmdCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, "journalctl", args...)
	output, err := cmd.CombinedOutput()
	outputStr := strings.TrimSpace(string(output))

	s.log.WithFields(map[string]interface{}{
		"command": fmt.Sprintf("journalctl %s", strings.Join(args, " ")),
		"lines":   lines,
	}).Debug("执行 journalctl 命令")

	if err != nil {
		// 检查是否是超时
		if cmdCtx.Err() == context.DeadlineExceeded {
			return nil, s.err.New("获取日志超时", err)
		}
		// journalctl 可能返回非 0 但仍有输出（例如服务不存在）
		if outputStr != "" {
			// 如果有输出，仍然返回
			return strings.Split(outputStr, "\n"), nil
		}
		return nil, s.err.New(fmt.Sprintf("获取服务 %s 日志失败", unitName), err)
	}

	if outputStr == "" {
		return []string{}, nil
	}

	return strings.Split(outputStr, "\n"), nil
}

// GetRecentLogs 获取最近 N 行日志
func (s *JournalService) GetRecentLogs(ctx context.Context, unitName string, lines int) ([]string, error) {
	return s.GetLogs(ctx, unitName, lines, "", "")
}

// FollowLogs 实时跟踪日志（返回 channel）
// 注意：调用方需要负责关闭 context 来停止跟踪
func (s *JournalService) FollowLogs(ctx context.Context, unitName string) (<-chan string, error) {
	if err := s.checkPlatform(); err != nil {
		return nil, err
	}

	args := []string{
		"-u", unitName,
		"--no-pager",
		"-f",
		"--output=short-iso",
	}

	cmd := exec.CommandContext(ctx, "journalctl", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, s.err.New("创建输出管道失败", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, s.err.New("启动 journalctl 失败", err)
	}

	logChan := make(chan string, 100)

	go func() {
		defer close(logChan)
		defer cmd.Wait()

		buf := make([]byte, 4096)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				n, err := stdout.Read(buf)
				if err != nil {
					return
				}
				if n > 0 {
					lines := strings.Split(string(buf[:n]), "\n")
					for _, line := range lines {
						if line = strings.TrimSpace(line); line != "" {
							select {
							case logChan <- line:
							case <-ctx.Done():
								return
							}
						}
					}
				}
			}
		}
	}()

	return logChan, nil
}


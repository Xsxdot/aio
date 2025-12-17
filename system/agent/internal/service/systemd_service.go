package service

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	errorc "xiaozhizhang/pkg/core/err"
	"xiaozhizhang/pkg/core/logger"
)

const (
	DefaultSystemdUnitDir     = "/etc/systemd/system"
	DefaultCommandTimeout     = 30 * time.Second
	ServiceSuffix             = ".service"
	DefaultJournalLogLines    = 200
)

// SystemdUnitInfo systemd unit 文件信息
type SystemdUnitInfo struct {
	Name        string
	Content     string
	Description string
	ModTime     time.Time
}

// SystemdServiceStatus systemd 服务状态
type SystemdServiceStatus struct {
	Name              string
	Description       string
	LoadState         string
	ActiveState       string
	SubState          string
	UnitFileState     string
	MainPID           int32
	ExecMainStartAt   string
	MemoryCurrent     uint64
	Result            string
}

// SystemdService Agent 中的 systemd 管理服务
type SystemdService struct {
	unitDir        string
	commandTimeout time.Duration
	log            *logger.Log
	err            *errorc.ErrorBuilder
}

// NewSystemdService 创建 systemd 服务
func NewSystemdService(unitDir string, timeout time.Duration, log *logger.Log) *SystemdService {
	if unitDir == "" {
		unitDir = DefaultSystemdUnitDir
	}
	if timeout == 0 {
		timeout = DefaultCommandTimeout
	}

	return &SystemdService{
		unitDir:        unitDir,
		commandTimeout: timeout,
		log:            log.WithEntryName("AgentSystemdService"),
		err:            errorc.NewErrorBuilder("AgentSystemdService"),
	}
}

func (s *SystemdService) checkPlatform() error {
	if runtime.GOOS != "linux" {
		return s.err.New(fmt.Sprintf("systemd 仅支持 Linux 平台，当前: %s", runtime.GOOS), nil).ValidWithCtx()
	}
	return nil
}

func (s *SystemdService) validateName(name string) error {
	if name == "" {
		return s.err.New("服务名不能为空", nil).ValidWithCtx()
	}
	if !strings.HasSuffix(name, ServiceSuffix) {
		return s.err.New("服务名必须以 .service 结尾", nil).ValidWithCtx()
	}
	if strings.ContainsAny(name, "/\\") || strings.Contains(name, "..") {
		return s.err.New("服务名包含非法字符", nil).ValidWithCtx()
	}
	if filepath.Base(name) != name {
		return s.err.New("服务名必须是纯文件名", nil).ValidWithCtx()
	}
	validPattern := regexp.MustCompile(`^[a-zA-Z0-9_\-@.]+\.service$`)
	if !validPattern.MatchString(name) {
		return s.err.New("服务名只能包含字母、数字、-、_、@、.", nil).ValidWithCtx()
	}
	return nil
}

func (s *SystemdService) buildPath(name string) (string, error) {
	if err := s.validateName(name); err != nil {
		return "", err
	}
	fullPath := filepath.Join(s.unitDir, name)
	cleanPath := filepath.Clean(fullPath)
	cleanRoot := filepath.Clean(s.unitDir)
	rel, err := filepath.Rel(cleanRoot, cleanPath)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", s.err.New("路径超出允许范围", err).ValidWithCtx()
	}
	return cleanPath, nil
}

// PutUnit 创建或更新 unit 文件
func (s *SystemdService) PutUnit(ctx context.Context, name, content string, daemonReload bool) error {
	if err := s.checkPlatform(); err != nil {
		return err
	}

	fullPath, err := s.buildPath(name)
	if err != nil {
		return err
	}

	// 原子写入
	if err := s.atomicWrite(fullPath, content); err != nil {
		return err
	}

	// daemon-reload
	if daemonReload {
		if _, err := s.runCommand(ctx, "systemctl", "daemon-reload"); err != nil {
			return s.err.New("daemon-reload 失败", err)
		}
	}

	return nil
}

// GetUnit 读取 unit 文件
func (s *SystemdService) GetUnit(name string) (*SystemdUnitInfo, error) {
	if err := s.checkPlatform(); err != nil {
		return nil, err
	}

	fullPath, err := s.buildPath(name)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, s.err.New(fmt.Sprintf("服务 %s 不存在", name), nil).ValidWithCtx()
		}
		return nil, s.err.New("读取文件信息失败", err)
	}

	content, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, s.err.New("读取文件内容失败", err)
	}

	return &SystemdUnitInfo{
		Name:        name,
		Content:     string(content),
		Description: s.parseDescription(string(content)),
		ModTime:     info.ModTime(),
	}, nil
}

// DeleteUnit 删除 unit 文件
func (s *SystemdService) DeleteUnit(ctx context.Context, name string, stopService, disableService, daemonReload bool) error {
	if err := s.checkPlatform(); err != nil {
		return err
	}

	// 停止服务
	if stopService {
		s.runCommand(ctx, "systemctl", "stop", name)
	}

	// 禁用服务
	if disableService {
		s.runCommand(ctx, "systemctl", "disable", name)
	}

	fullPath, err := s.buildPath(name)
	if err != nil {
		return err
	}

	// 删除文件
	if err := os.Remove(fullPath); err != nil {
		if !os.IsNotExist(err) {
			return s.err.New("删除 unit 文件失败", err)
		}
	}

	// daemon-reload
	if daemonReload {
		s.runCommand(ctx, "systemctl", "daemon-reload")
	}

	return nil
}

// ListUnits 列出所有 unit 文件
func (s *SystemdService) ListUnits(keyword string) ([]*SystemdUnitInfo, error) {
	if err := s.checkPlatform(); err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(s.unitDir)
	if err != nil {
		return nil, s.err.New("读取目录失败", err)
	}

	var result []*SystemdUnitInfo
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ServiceSuffix) {
			continue
		}

		name := entry.Name()
		if keyword != "" && !strings.Contains(strings.ToLower(name), strings.ToLower(keyword)) {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		fullPath := filepath.Join(s.unitDir, name)
		content, _ := os.ReadFile(fullPath)
		desc := s.parseDescription(string(content))

		result = append(result, &SystemdUnitInfo{
			Name:        name,
			Description: desc,
			ModTime:     info.ModTime(),
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].ModTime.After(result[j].ModTime)
	})

	return result, nil
}

// DaemonReload 执行 daemon-reload
func (s *SystemdService) DaemonReload(ctx context.Context) (string, error) {
	if err := s.checkPlatform(); err != nil {
		return "", err
	}
	return s.runCommand(ctx, "systemctl", "daemon-reload")
}

// ServiceControl 控制服务（start/stop/restart/reload/enable/disable）
func (s *SystemdService) ServiceControl(ctx context.Context, name, action string) (string, error) {
	if err := s.checkPlatform(); err != nil {
		return "", err
	}

	validActions := map[string]bool{
		"start": true, "stop": true, "restart": true,
		"reload": true, "enable": true, "disable": true,
	}
	if !validActions[action] {
		return "", s.err.New(fmt.Sprintf("不支持的操作: %s", action), nil).ValidWithCtx()
	}

	return s.runCommand(ctx, "systemctl", action, name)
}

// GetServiceStatus 获取服务状态
func (s *SystemdService) GetServiceStatus(ctx context.Context, name string) (*SystemdServiceStatus, error) {
	if err := s.checkPlatform(); err != nil {
		return nil, err
	}

	properties := []string{
		"Description", "LoadState", "ActiveState", "SubState",
		"UnitFileState", "MainPID", "ExecMainStartTimestamp",
		"MemoryCurrent", "Result",
	}

	output, err := s.runCommand(ctx, "systemctl", "show", name, "--no-page", "--property="+strings.Join(properties, ","))
	if err != nil {
		return nil, err
	}

	status := &SystemdServiceStatus{Name: name}
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		parts := strings.SplitN(strings.TrimSpace(line), "=", 2)
		if len(parts) != 2 {
			continue
		}

		key, value := parts[0], parts[1]
		switch key {
		case "Description":
			status.Description = value
		case "LoadState":
			status.LoadState = value
		case "ActiveState":
			status.ActiveState = value
		case "SubState":
			status.SubState = value
		case "UnitFileState":
			status.UnitFileState = value
		case "MainPID":
			if pid, err := strconv.Atoi(value); err == nil {
				status.MainPID = int32(pid)
			}
		case "ExecMainStartTimestamp":
			status.ExecMainStartAt = value
		case "MemoryCurrent":
			if value != "[not set]" {
				if mem, err := strconv.ParseUint(value, 10, 64); err == nil {
					status.MemoryCurrent = mem
				}
			}
		case "Result":
			status.Result = value
		}
	}

	return status, nil
}

// GetServiceLogs 获取服务日志
func (s *SystemdService) GetServiceLogs(ctx context.Context, name string, lines int, since, until string) ([]string, error) {
	if err := s.checkPlatform(); err != nil {
		return nil, err
	}

	if lines <= 0 {
		lines = DefaultJournalLogLines
	}

	args := []string{"-u", name, "--no-pager", "-n", strconv.Itoa(lines), "--output=short-iso"}
	if since != "" {
		args = append(args, "--since", since)
	}
	if until != "" {
		args = append(args, "--until", until)
	}

	output, err := s.runCommand(ctx, "journalctl", args...)
	if err != nil {
		// journalctl may return non-zero but still have output
		if output != "" {
			return strings.Split(output, "\n"), nil
		}
		return nil, err
	}

	if output == "" {
		return []string{}, nil
	}

	return strings.Split(output, "\n"), nil
}

func (s *SystemdService) atomicWrite(targetPath, content string) error {
	dir := filepath.Dir(targetPath)
	tempFile, err := os.CreateTemp(dir, ".unit-*.tmp")
	if err != nil {
		return s.err.New("创建临时文件失败", err)
	}
	tempPath := tempFile.Name()
	defer os.Remove(tempPath)

	if _, err := tempFile.WriteString(content); err != nil {
		tempFile.Close()
		return s.err.New("写入临时文件失败", err)
	}

	if err := tempFile.Sync(); err != nil {
		tempFile.Close()
		return s.err.New("同步文件失败", err)
	}

	if err := tempFile.Close(); err != nil {
		return s.err.New("关闭临时文件失败", err)
	}

	if err := os.Chmod(tempPath, fs.FileMode(0644)); err != nil {
		return s.err.New("设置文件权限失败", err)
	}

	if err := os.Rename(tempPath, targetPath); err != nil {
		return s.err.New("重命名文件失败", err)
	}

	return nil
}

func (s *SystemdService) runCommand(ctx context.Context, name string, args ...string) (string, error) {
	cmdCtx, cancel := context.WithTimeout(ctx, s.commandTimeout)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, name, args...)
	output, err := cmd.CombinedOutput()
	outputStr := strings.TrimSpace(string(output))

	if err != nil {
		if cmdCtx.Err() == context.DeadlineExceeded {
			return outputStr, s.err.New("命令执行超时", err)
		}
		return outputStr, s.err.New(fmt.Sprintf("命令执行失败: %s", outputStr), err)
	}

	return outputStr, nil
}

func (s *SystemdService) parseDescription(content string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Description=") {
			return strings.TrimPrefix(line, "Description=")
		}
	}
	return ""
}


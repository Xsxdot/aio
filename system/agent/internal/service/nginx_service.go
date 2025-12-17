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
	"strings"
	"time"

	errorc "xiaozhizhang/pkg/core/err"
	"xiaozhizhang/pkg/core/logger"
)

const (
	DefaultNginxConfDir       = "/etc/nginx/conf.d"
	DefaultNginxValidateCmd   = "nginx -t"
	DefaultNginxReloadCmd     = "nginx -s reload"
	DefaultNginxCommandTimeout = 30 * time.Second
	ConfSuffix                = ".conf"
)

// NginxConfigInfo nginx 配置文件信息
type NginxConfigInfo struct {
	Name        string
	Content     string
	Description string
	ModTime     time.Time
}

// NginxService Agent 中的 nginx 管理服务
type NginxService struct {
	rootDir        string
	fileMode       fs.FileMode
	validateCmd    string
	reloadCmd      string
	commandTimeout time.Duration
	log            *logger.Log
	err            *errorc.ErrorBuilder
}

// NewNginxService 创建 nginx 服务
func NewNginxService(rootDir, fileMode, validateCmd, reloadCmd string, timeout time.Duration, log *logger.Log) *NginxService {
	if rootDir == "" {
		rootDir = DefaultNginxConfDir
	}
	if validateCmd == "" {
		validateCmd = DefaultNginxValidateCmd
	}
	if reloadCmd == "" {
		reloadCmd = DefaultNginxReloadCmd
	}
	if timeout == 0 {
		timeout = DefaultNginxCommandTimeout
	}

	mode := fs.FileMode(0644)
	if fileMode != "" {
		if parsed, err := parseFileMode(fileMode); err == nil {
			mode = parsed
		}
	}

	return &NginxService{
		rootDir:        rootDir,
		fileMode:       mode,
		validateCmd:    validateCmd,
		reloadCmd:      reloadCmd,
		commandTimeout: timeout,
		log:            log.WithEntryName("AgentNginxService"),
		err:            errorc.NewErrorBuilder("AgentNginxService"),
	}
}

func parseFileMode(s string) (fs.FileMode, error) {
	var mode uint32
	_, err := fmt.Sscanf(s, "%o", &mode)
	if err != nil {
		return 0, err
	}
	return fs.FileMode(mode), nil
}

func (s *NginxService) checkPlatform() error {
	if runtime.GOOS != "linux" {
		return s.err.New(fmt.Sprintf("nginx 仅支持 Linux 平台，当前: %s", runtime.GOOS), nil).ValidWithCtx()
	}
	return nil
}

func (s *NginxService) validateName(name string) error {
	if name == "" {
		return s.err.New("配置文件名不能为空", nil).ValidWithCtx()
	}
	if !strings.HasSuffix(name, ConfSuffix) {
		return s.err.New("配置文件名必须以 .conf 结尾", nil).ValidWithCtx()
	}
	if strings.ContainsAny(name, "/\\") || strings.Contains(name, "..") {
		return s.err.New("配置文件名包含非法字符", nil).ValidWithCtx()
	}
	if filepath.Base(name) != name {
		return s.err.New("配置文件名必须是纯文件名", nil).ValidWithCtx()
	}
	validPattern := regexp.MustCompile(`^[a-zA-Z0-9_\-.]+\.conf$`)
	if !validPattern.MatchString(name) {
		return s.err.New("配置文件名只能包含字母、数字、-、_、.", nil).ValidWithCtx()
	}
	return nil
}

func (s *NginxService) buildPath(name string) (string, error) {
	if err := s.validateName(name); err != nil {
		return "", err
	}
	fullPath := filepath.Join(s.rootDir, name)
	cleanPath := filepath.Clean(fullPath)
	cleanRoot := filepath.Clean(s.rootDir)
	rel, err := filepath.Rel(cleanRoot, cleanPath)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", s.err.New("路径超出允许范围", err).ValidWithCtx()
	}
	return cleanPath, nil
}

// PutConfig 创建或更新配置文件（原子写入）
func (s *NginxService) PutConfig(ctx context.Context, name, content string, validate, reload bool) (validateOut, reloadOut string, err error) {
	if err := s.checkPlatform(); err != nil {
		return "", "", err
	}

	fullPath, err := s.buildPath(name)
	if err != nil {
		return "", "", err
	}

	// 原子写入
	if err := s.atomicWrite(fullPath, content); err != nil {
		return "", "", err
	}

	// 校验
	if validate {
		validateOut, err = s.runCommand(ctx, s.validateCmd)
		if err != nil {
			// 回滚：删除文件
			os.Remove(fullPath)
			return validateOut, "", s.err.New("nginx 配置校验失败", err)
		}
	}

	// 重载
	if reload {
		reloadOut, err = s.runCommand(ctx, s.reloadCmd)
		if err != nil {
			return validateOut, reloadOut, s.err.New("nginx 重载失败", err)
		}
	}

	return validateOut, reloadOut, nil
}

// GetConfig 读取配置文件
func (s *NginxService) GetConfig(name string) (*NginxConfigInfo, error) {
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
			return nil, s.err.New(fmt.Sprintf("配置文件 %s 不存在", name), nil).ValidWithCtx()
		}
		return nil, s.err.New("读取文件信息失败", err)
	}

	content, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, s.err.New("读取文件内容失败", err)
	}

	return &NginxConfigInfo{
		Name:        name,
		Content:     string(content),
		Description: s.parseDescription(string(content)),
		ModTime:     info.ModTime(),
	}, nil
}

// DeleteConfig 删除配置文件
func (s *NginxService) DeleteConfig(ctx context.Context, name string, validate, reload bool) error {
	if err := s.checkPlatform(); err != nil {
		return err
	}

	fullPath, err := s.buildPath(name)
	if err != nil {
		return err
	}

	// 备份旧内容
	var oldContent string
	if data, err := os.ReadFile(fullPath); err == nil {
		oldContent = string(data)
	}

	// 删除文件
	if err := os.Remove(fullPath); err != nil {
		if !os.IsNotExist(err) {
			return s.err.New("删除配置文件失败", err)
		}
	}

	// 校验
	if validate {
		if _, err := s.runCommand(ctx, s.validateCmd); err != nil {
			// 回滚：恢复文件
			if oldContent != "" {
				os.WriteFile(fullPath, []byte(oldContent), s.fileMode)
			}
			return s.err.New("删除后配置校验失败", err)
		}
	}

	// 重载
	if reload {
		if _, err := s.runCommand(ctx, s.reloadCmd); err != nil {
			return s.err.New("nginx 重载失败", err)
		}
	}

	return nil
}

// ListConfigs 列出所有配置文件
func (s *NginxService) ListConfigs(keyword string) ([]*NginxConfigInfo, error) {
	if err := s.checkPlatform(); err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(s.rootDir)
	if err != nil {
		return nil, s.err.New("读取目录失败", err)
	}

	var result []*NginxConfigInfo
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ConfSuffix) {
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

		fullPath := filepath.Join(s.rootDir, name)
		content, _ := os.ReadFile(fullPath)
		desc := s.parseDescription(string(content))

		result = append(result, &NginxConfigInfo{
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

// Validate 执行 nginx -t 校验
func (s *NginxService) Validate(ctx context.Context) (string, error) {
	if err := s.checkPlatform(); err != nil {
		return "", err
	}
	return s.runCommand(ctx, s.validateCmd)
}

// Reload 执行 nginx -s reload 重载
func (s *NginxService) Reload(ctx context.Context) (string, error) {
	if err := s.checkPlatform(); err != nil {
		return "", err
	}
	return s.runCommand(ctx, s.reloadCmd)
}

func (s *NginxService) atomicWrite(targetPath, content string) error {
	dir := filepath.Dir(targetPath)
	tempFile, err := os.CreateTemp(dir, ".nginx-*.tmp")
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

	if err := os.Chmod(tempPath, s.fileMode); err != nil {
		return s.err.New("设置文件权限失败", err)
	}

	if err := os.Rename(tempPath, targetPath); err != nil {
		return s.err.New("重命名文件失败", err)
	}

	return nil
}

func (s *NginxService) runCommand(ctx context.Context, command string) (string, error) {
	cmdCtx, cancel := context.WithTimeout(ctx, s.commandTimeout)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, "sh", "-c", command)
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

func (s *NginxService) parseDescription(content string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			comment := strings.TrimSpace(strings.TrimPrefix(line, "#"))
			if strings.HasPrefix(comment, "Description:") {
				return strings.TrimSpace(strings.TrimPrefix(comment, "Description:"))
			}
			if strings.HasPrefix(comment, "描述:") {
				return strings.TrimSpace(strings.TrimPrefix(comment, "描述:"))
			}
		}
	}
	return ""
}


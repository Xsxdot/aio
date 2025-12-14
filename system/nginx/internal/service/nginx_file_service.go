package service

import (
	"fmt"
	"io/fs"
	"os"
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
	// DefaultNginxConfDir 默认 nginx 配置文件目录
	DefaultNginxConfDir = "/etc/nginx/conf.d"
	// ConfSuffix 配置文件后缀
	ConfSuffix = ".conf"
)

// ConfigFileInfo 配置文件信息
type ConfigFileInfo struct {
	Name        string    // 文件名（含 .conf）
	FullPath    string    // 完整路径
	Content     string    // 文件内容
	Description string    // 从配置中解析的描述（如 # Description: xxx）
	ModTime     time.Time // 修改时间
}

// NginxFileService nginx 配置文件服务
// 负责对 /etc/nginx/conf.d 下的 *.conf 文件进行 CRUD
type NginxFileService struct {
	rootDir  string
	fileMode fs.FileMode
	log      *logger.Log
	err      *errorc.ErrorBuilder
}

// NewNginxFileService 创建 nginx 文件服务
func NewNginxFileService(rootDir string, fileMode string, log *logger.Log) *NginxFileService {
	if rootDir == "" {
		rootDir = DefaultNginxConfDir
	}

	// 解析文件权限
	mode := fs.FileMode(0644)
	if fileMode != "" {
		if parsed, err := parseFileMode(fileMode); err == nil {
			mode = parsed
		}
	}

	return &NginxFileService{
		rootDir:  rootDir,
		fileMode: mode,
		log:      log.WithEntryName("NginxFileService"),
		err:      errorc.NewErrorBuilder("NginxFileService"),
	}
}

// parseFileMode 解析文件权限字符串（如 "0644"）
func parseFileMode(s string) (fs.FileMode, error) {
	var mode uint32
	_, err := fmt.Sscanf(s, "%o", &mode)
	if err != nil {
		return 0, err
	}
	return fs.FileMode(mode), nil
}

// checkPlatform 检查平台是否为 Linux
func (s *NginxFileService) checkPlatform() error {
	if runtime.GOOS != "linux" {
		return s.err.New(fmt.Sprintf("nginx 配置管理仅支持 Linux 平台，当前平台: %s", runtime.GOOS), nil).ValidWithCtx()
	}
	return nil
}

// ValidateName 校验配置文件名称
// - 必须以 .conf 结尾
// - 不能包含路径分隔符或 ..
// - 只能是文件名，不能是路径
func (s *NginxFileService) ValidateName(name string) error {
	if name == "" {
		return s.err.New("配置文件名称不能为空", nil).ValidWithCtx()
	}

	// 必须以 .conf 结尾
	if !strings.HasSuffix(name, ConfSuffix) {
		return s.err.New("配置文件名称必须以 .conf 结尾", nil).ValidWithCtx()
	}

	// 不能包含路径分隔符
	if strings.ContainsAny(name, "/\\") {
		return s.err.New("配置文件名称不能包含路径分隔符", nil).ValidWithCtx()
	}

	// 不能包含 ..
	if strings.Contains(name, "..") {
		return s.err.New("配置文件名称不能包含 '..'", nil).ValidWithCtx()
	}

	// 必须等于 filepath.Base
	if filepath.Base(name) != name {
		return s.err.New("配置文件名称必须是纯文件名", nil).ValidWithCtx()
	}

	// 配置文件名至少有一个字符（除了 .conf 后缀）
	baseName := strings.TrimSuffix(name, ConfSuffix)
	if len(baseName) == 0 {
		return s.err.New("配置文件名称不能仅为 .conf", nil).ValidWithCtx()
	}

	// 只允许合法字符：字母、数字、-、_、.
	validPattern := regexp.MustCompile(`^[a-zA-Z0-9_\-.]+\.conf$`)
	if !validPattern.MatchString(name) {
		return s.err.New("配置文件名称只能包含字母、数字、-、_、. 字符", nil).ValidWithCtx()
	}

	return nil
}

// buildFullPath 构建完整路径并校验安全性
func (s *NginxFileService) buildFullPath(name string) (string, error) {
	if err := s.ValidateName(name); err != nil {
		return "", err
	}

	fullPath := filepath.Join(s.rootDir, name)

	// 再次校验路径确实在 rootDir 下
	cleanPath := filepath.Clean(fullPath)
	cleanRoot := filepath.Clean(s.rootDir)

	rel, err := filepath.Rel(cleanRoot, cleanPath)
	if err != nil {
		return "", s.err.New("路径计算失败", err)
	}
	if strings.HasPrefix(rel, "..") {
		return "", s.err.New("路径超出允许范围", nil).ValidWithCtx()
	}

	return cleanPath, nil
}

// Exists 检查配置文件是否存在
func (s *NginxFileService) Exists(name string) (bool, error) {
	if err := s.checkPlatform(); err != nil {
		return false, err
	}

	fullPath, err := s.buildFullPath(name)
	if err != nil {
		return false, err
	}

	_, err = os.Stat(fullPath)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, s.err.New("检查文件是否存在失败", err)
}

// Read 读取配置文件
func (s *NginxFileService) Read(name string) (*ConfigFileInfo, error) {
	if err := s.checkPlatform(); err != nil {
		return nil, err
	}

	fullPath, err := s.buildFullPath(name)
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

	return &ConfigFileInfo{
		Name:        name,
		FullPath:    fullPath,
		Content:     string(content),
		Description: s.parseDescription(string(content)),
		ModTime:     info.ModTime(),
	}, nil
}

// Create 创建配置文件（原子写入）
func (s *NginxFileService) Create(name, content string) error {
	if err := s.checkPlatform(); err != nil {
		return err
	}

	fullPath, err := s.buildFullPath(name)
	if err != nil {
		return err
	}

	// 检查是否已存在
	if _, err := os.Stat(fullPath); err == nil {
		return s.err.New(fmt.Sprintf("配置文件 %s 已存在", name), nil).ValidWithCtx()
	}

	// 原子写入：先写临时文件，再 rename
	return s.atomicWrite(fullPath, content)
}

// Update 更新配置文件（原子写入）
func (s *NginxFileService) Update(name, content string) error {
	if err := s.checkPlatform(); err != nil {
		return err
	}

	fullPath, err := s.buildFullPath(name)
	if err != nil {
		return err
	}

	// 检查是否存在
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return s.err.New(fmt.Sprintf("配置文件 %s 不存在", name), nil).ValidWithCtx()
	}

	// 原子写入
	return s.atomicWrite(fullPath, content)
}

// Delete 删除配置文件
func (s *NginxFileService) Delete(name string) error {
	if err := s.checkPlatform(); err != nil {
		return err
	}

	fullPath, err := s.buildFullPath(name)
	if err != nil {
		return err
	}

	// 检查是否存在
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		s.log.WithField("name", name).Warn("配置文件不存在，跳过删除")
		return nil
	}

	if err := os.Remove(fullPath); err != nil {
		return s.err.New("删除配置文件失败", err)
	}

	s.log.WithField("name", name).Info("配置文件删除成功")
	return nil
}

// List 列出所有 *.conf 文件
func (s *NginxFileService) List(keyword string) ([]*ConfigFileInfo, error) {
	if err := s.checkPlatform(); err != nil {
		return nil, err
	}

	var result []*ConfigFileInfo

	entries, err := os.ReadDir(s.rootDir)
	if err != nil {
		return nil, s.err.New("读取目录失败", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(name, ConfSuffix) {
			continue
		}

		// 关键字过滤
		if keyword != "" && !strings.Contains(strings.ToLower(name), strings.ToLower(keyword)) {
			// 尝试读取描述进行匹配
			info, err := s.Read(name)
			if err != nil {
				continue
			}
			if !strings.Contains(strings.ToLower(info.Description), strings.ToLower(keyword)) {
				continue
			}
			result = append(result, info)
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		// 读取描述
		fullPath := filepath.Join(s.rootDir, name)
		content, err := os.ReadFile(fullPath)
		desc := ""
		if err == nil {
			desc = s.parseDescription(string(content))
		}

		result = append(result, &ConfigFileInfo{
			Name:        name,
			FullPath:    fullPath,
			Description: desc,
			ModTime:     info.ModTime(),
		})
	}

	// 按修改时间倒序
	sort.Slice(result, func(i, j int) bool {
		return result[i].ModTime.After(result[j].ModTime)
	})

	return result, nil
}

// atomicWrite 原子写入文件
func (s *NginxFileService) atomicWrite(targetPath, content string) error {
	// 创建临时文件
	dir := filepath.Dir(targetPath)
	tempFile, err := os.CreateTemp(dir, ".nginx-*.tmp")
	if err != nil {
		return s.err.New("创建临时文件失败", err)
	}
	tempPath := tempFile.Name()

	// 确保清理临时文件
	defer func() {
		os.Remove(tempPath)
	}()

	// 写入内容
	if _, err := tempFile.WriteString(content); err != nil {
		tempFile.Close()
		return s.err.New("写入临时文件失败", err)
	}

	// 同步到磁盘
	if err := tempFile.Sync(); err != nil {
		tempFile.Close()
		return s.err.New("同步文件失败", err)
	}

	if err := tempFile.Close(); err != nil {
		return s.err.New("关闭临时文件失败", err)
	}

	// 设置权限
	if err := os.Chmod(tempPath, s.fileMode); err != nil {
		return s.err.New("设置文件权限失败", err)
	}

	// 原子 rename
	if err := os.Rename(tempPath, targetPath); err != nil {
		return s.err.New("重命名文件失败", err)
	}

	s.log.WithField("path", targetPath).Info("配置文件写入成功")
	return nil
}

// parseDescription 从配置内容解析描述
// 支持格式：# Description: xxx 或 # 描述: xxx
func (s *NginxFileService) parseDescription(content string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			comment := strings.TrimPrefix(line, "#")
			comment = strings.TrimSpace(comment)
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

// GetRootDir 获取根目录
func (s *NginxFileService) GetRootDir() string {
	return s.rootDir
}




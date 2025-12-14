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
	// DefaultUnitDir 默认 unit 文件目录
	DefaultUnitDir = "/etc/systemd/system"
	// ServiceSuffix 服务文件后缀
	ServiceSuffix = ".service"
)

// UnitFileInfo unit 文件信息
type UnitFileInfo struct {
	Name        string    // 文件名（含 .service）
	FullPath    string    // 完整路径
	Content     string    // 文件内容
	Description string    // 从 [Unit] Description 解析
	ModTime     time.Time // 修改时间
}

// UnitFileService unit 文件服务
// 负责对 /etc/systemd/system 下的 *.service 文件进行 CRUD
type UnitFileService struct {
	rootDir string
	log     *logger.Log
	err     *errorc.ErrorBuilder
}

// NewUnitFileService 创建 unit 文件服务
func NewUnitFileService(rootDir string, log *logger.Log) *UnitFileService {
	if rootDir == "" {
		rootDir = DefaultUnitDir
	}
	return &UnitFileService{
		rootDir: rootDir,
		log:     log.WithEntryName("UnitFileService"),
		err:     errorc.NewErrorBuilder("UnitFileService"),
	}
}

// checkPlatform 检查平台是否为 Linux
func (s *UnitFileService) checkPlatform() error {
	if runtime.GOOS != "linux" {
		return s.err.New(fmt.Sprintf("systemd 仅支持 Linux 平台，当前平台: %s", runtime.GOOS), nil).ValidWithCtx()
	}
	return nil
}

// ValidateName 校验 unit 名称
// - 必须以 .service 结尾
// - 不能包含路径分隔符或 ..
// - 只能是文件名，不能是路径
func (s *UnitFileService) ValidateName(name string) error {
	if name == "" {
		return s.err.New("服务名称不能为空", nil).ValidWithCtx()
	}

	// 必须以 .service 结尾
	if !strings.HasSuffix(name, ServiceSuffix) {
		return s.err.New("服务名称必须以 .service 结尾", nil).ValidWithCtx()
	}

	// 不能包含路径分隔符
	if strings.ContainsAny(name, "/\\") {
		return s.err.New("服务名称不能包含路径分隔符", nil).ValidWithCtx()
	}

	// 不能包含 ..
	if strings.Contains(name, "..") {
		return s.err.New("服务名称不能包含 '..'", nil).ValidWithCtx()
	}

	// 必须等于 filepath.Base
	if filepath.Base(name) != name {
		return s.err.New("服务名称必须是纯文件名", nil).ValidWithCtx()
	}

	// 服务名至少有一个字符（除了 .service 后缀）
	baseName := strings.TrimSuffix(name, ServiceSuffix)
	if len(baseName) == 0 {
		return s.err.New("服务名称不能仅为 .service", nil).ValidWithCtx()
	}

	// 只允许合法字符：字母、数字、-、_、@、.
	validPattern := regexp.MustCompile(`^[a-zA-Z0-9_\-@.]+\.service$`)
	if !validPattern.MatchString(name) {
		return s.err.New("服务名称只能包含字母、数字、-、_、@、. 字符", nil).ValidWithCtx()
	}

	return nil
}

// buildFullPath 构建完整路径并校验安全性
func (s *UnitFileService) buildFullPath(name string) (string, error) {
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

// Exists 检查 unit 文件是否存在
func (s *UnitFileService) Exists(name string) (bool, error) {
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

// Read 读取 unit 文件
func (s *UnitFileService) Read(name string) (*UnitFileInfo, error) {
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
			return nil, s.err.New(fmt.Sprintf("服务 %s 不存在", name), nil).ValidWithCtx()
		}
		return nil, s.err.New("读取文件信息失败", err)
	}

	content, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, s.err.New("读取文件内容失败", err)
	}

	return &UnitFileInfo{
		Name:        name,
		FullPath:    fullPath,
		Content:     string(content),
		Description: s.parseDescription(string(content)),
		ModTime:     info.ModTime(),
	}, nil
}

// Create 创建 unit 文件（原子写入）
func (s *UnitFileService) Create(name, content string) error {
	if err := s.checkPlatform(); err != nil {
		return err
	}

	fullPath, err := s.buildFullPath(name)
	if err != nil {
		return err
	}

	// 检查是否已存在
	if _, err := os.Stat(fullPath); err == nil {
		return s.err.New(fmt.Sprintf("服务 %s 已存在", name), nil).ValidWithCtx()
	}

	// 原子写入：先写临时文件，再 rename
	return s.atomicWrite(fullPath, content)
}

// Update 更新 unit 文件（原子写入）
func (s *UnitFileService) Update(name, content string) error {
	if err := s.checkPlatform(); err != nil {
		return err
	}

	fullPath, err := s.buildFullPath(name)
	if err != nil {
		return err
	}

	// 检查是否存在
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return s.err.New(fmt.Sprintf("服务 %s 不存在", name), nil).ValidWithCtx()
	}

	// 原子写入
	return s.atomicWrite(fullPath, content)
}

// Delete 删除 unit 文件
func (s *UnitFileService) Delete(name string) error {
	if err := s.checkPlatform(); err != nil {
		return err
	}

	fullPath, err := s.buildFullPath(name)
	if err != nil {
		return err
	}

	// 检查是否存在
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		s.log.WithField("name", name).Warn("服务文件不存在，跳过删除")
		return nil
	}

	if err := os.Remove(fullPath); err != nil {
		return s.err.New("删除服务文件失败", err)
	}

	s.log.WithField("name", name).Info("服务文件删除成功")
	return nil
}

// List 列出所有 *.service 文件
func (s *UnitFileService) List(keyword string) ([]*UnitFileInfo, error) {
	if err := s.checkPlatform(); err != nil {
		return nil, err
	}

	var result []*UnitFileInfo

	entries, err := os.ReadDir(s.rootDir)
	if err != nil {
		return nil, s.err.New("读取目录失败", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(name, ServiceSuffix) {
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

		result = append(result, &UnitFileInfo{
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
func (s *UnitFileService) atomicWrite(targetPath, content string) error {
	// 创建临时文件
	dir := filepath.Dir(targetPath)
	tempFile, err := os.CreateTemp(dir, ".unit-*.tmp")
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

	// 设置权限 0644
	if err := os.Chmod(tempPath, fs.FileMode(0644)); err != nil {
		return s.err.New("设置文件权限失败", err)
	}

	// 原子 rename
	if err := os.Rename(tempPath, targetPath); err != nil {
		return s.err.New("重命名文件失败", err)
	}

	s.log.WithField("path", targetPath).Info("unit 文件写入成功")
	return nil
}

// parseDescription 从 unit 内容解析 Description
func (s *UnitFileService) parseDescription(content string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Description=") {
			return strings.TrimPrefix(line, "Description=")
		}
	}
	return ""
}

// GetRootDir 获取根目录
func (s *UnitFileService) GetRootDir() string {
	return s.rootDir
}


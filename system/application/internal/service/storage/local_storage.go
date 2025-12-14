package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	errorc "xiaozhizhang/pkg/core/err"
	"xiaozhizhang/pkg/core/logger"
)

// LocalStorage 本地文件存储实现
type LocalStorage struct {
	baseDir string
	log     *logger.Log
	err     *errorc.ErrorBuilder
}

// NewLocalStorage 创建本地存储实例
func NewLocalStorage(baseDir string, log *logger.Log) (*LocalStorage, error) {
	// 确保目录存在
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("创建存储目录失败: %w", err)
	}

	return &LocalStorage{
		baseDir: baseDir,
		log:     log.WithEntryName("LocalStorage"),
		err:     errorc.NewErrorBuilder("LocalStorage"),
	}, nil
}

// Mode 返回存储模式标识
func (s *LocalStorage) Mode() string {
	return "local"
}

// Put 上传文件到本地
func (s *LocalStorage) Put(ctx context.Context, key string, reader io.Reader, size int64, contentType string) (*StoredObject, error) {
	// 安全检查：防止目录穿越
	if err := s.validateKey(key); err != nil {
		return nil, err
	}

	fullPath := filepath.Join(s.baseDir, key)

	// 确保父目录存在
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, s.err.New("创建目录失败", err)
	}

	// 创建临时文件
	tmpFile, err := os.CreateTemp(dir, ".upload-*")
	if err != nil {
		return nil, s.err.New("创建临时文件失败", err)
	}
	tmpPath := tmpFile.Name()
	defer func() {
		tmpFile.Close()
		os.Remove(tmpPath) // 清理临时文件
	}()

	// 写入并计算 SHA256
	hash := sha256.New()
	writer := io.MultiWriter(tmpFile, hash)

	written, err := io.Copy(writer, reader)
	if err != nil {
		return nil, s.err.New("写入文件失败", err)
	}

	tmpFile.Close()

	// 验证大小
	if size > 0 && written != size {
		return nil, s.err.New(fmt.Sprintf("文件大小不匹配: 期望 %d, 实际 %d", size, written), nil).ValidWithCtx()
	}

	// 移动到目标位置
	if err := os.Rename(tmpPath, fullPath); err != nil {
		return nil, s.err.New("移动文件失败", err)
	}

	sha256Sum := hex.EncodeToString(hash.Sum(nil))

	s.log.WithFields(map[string]interface{}{
		"key":  key,
		"size": written,
		"sha256": sha256Sum,
	}).Info("文件上传成功")

	return &StoredObject{
		Key:         key,
		Size:        written,
		ContentType: contentType,
		SHA256:      sha256Sum,
		CreatedAt:   time.Now(),
	}, nil
}

// Open 打开文件用于读取
func (s *LocalStorage) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	if err := s.validateKey(key); err != nil {
		return nil, err
	}

	fullPath := filepath.Join(s.baseDir, key)
	file, err := os.Open(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, s.err.New("文件不存在", err).WithCode(errorc.ErrorCodeNotFound)
		}
		return nil, s.err.New("打开文件失败", err)
	}

	return file, nil
}

// Delete 删除文件
func (s *LocalStorage) Delete(ctx context.Context, key string) error {
	if err := s.validateKey(key); err != nil {
		return err
	}

	fullPath := filepath.Join(s.baseDir, key)
	err := os.Remove(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // 文件不存在不算错误
		}
		return s.err.New("删除文件失败", err)
	}

	s.log.WithField("key", key).Info("文件删除成功")
	return nil
}

// Stat 获取文件信息
func (s *LocalStorage) Stat(ctx context.Context, key string) (*StoredObject, error) {
	if err := s.validateKey(key); err != nil {
		return nil, err
	}

	fullPath := filepath.Join(s.baseDir, key)
	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, s.err.New("文件不存在", err).WithCode(errorc.ErrorCodeNotFound)
		}
		return nil, s.err.New("获取文件信息失败", err)
	}

	return &StoredObject{
		Key:       key,
		Size:      info.Size(),
		CreatedAt: info.ModTime(),
	}, nil
}

// Exists 检查文件是否存在
func (s *LocalStorage) Exists(ctx context.Context, key string) (bool, error) {
	if err := s.validateKey(key); err != nil {
		return false, err
	}

	fullPath := filepath.Join(s.baseDir, key)
	_, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, s.err.New("检查文件是否存在失败", err)
	}

	return true, nil
}

// validateKey 验证 key 是否安全
func (s *LocalStorage) validateKey(key string) error {
	// 防止目录穿越攻击
	if strings.Contains(key, "..") {
		return s.err.New("非法的存储键", nil).ValidWithCtx()
	}

	// 清理路径
	cleanPath := filepath.Clean(key)
	if cleanPath != key && cleanPath != "./"+key {
		// 允许 key 和 ./key 的情况
		if strings.HasPrefix(key, "/") || strings.HasPrefix(cleanPath, "..") {
			return s.err.New("非法的存储键", nil).ValidWithCtx()
		}
	}

	return nil
}


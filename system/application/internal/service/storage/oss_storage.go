package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"time"

	errorc "xiaozhizhang/pkg/core/err"
	"xiaozhizhang/pkg/core/logger"
	"xiaozhizhang/pkg/oss"
)

// OSSStorage OSS 存储实现
type OSSStorage struct {
	ossService *oss.AliyunService
	prefix     string // key 前缀，用于隔离
	log        *logger.Log
	err        *errorc.ErrorBuilder
}

// NewOSSStorage 创建 OSS 存储实例
func NewOSSStorage(ossService *oss.AliyunService, prefix string, log *logger.Log) *OSSStorage {
	return &OSSStorage{
		ossService: ossService,
		prefix:     prefix,
		log:        log.WithEntryName("OSSStorage"),
		err:        errorc.NewErrorBuilder("OSSStorage"),
	}
}

// Mode 返回存储模式标识
func (s *OSSStorage) Mode() string {
	return "oss"
}

// fullKey 获取完整的 OSS key
func (s *OSSStorage) fullKey(key string) string {
	if s.prefix == "" {
		return key
	}
	return s.prefix + "/" + key
}

// Put 上传文件到 OSS
func (s *OSSStorage) Put(ctx context.Context, key string, reader io.Reader, size int64, contentType string) (*StoredObject, error) {
	fullKey := s.fullKey(key)

	// 读取内容并计算 SHA256（需要先读取以计算哈希）
	content, err := io.ReadAll(reader)
	if err != nil {
		return nil, s.err.New("读取文件内容失败", err)
	}

	// 验证大小
	if size > 0 && int64(len(content)) != size {
		return nil, s.err.New("文件大小不匹配", nil).ValidWithCtx()
	}

	hash := sha256.Sum256(content)
	sha256Sum := hex.EncodeToString(hash[:])

	// 上传到 OSS
	err = s.ossService.UploadFile(ctx, fullKey, bytes.NewReader(content))
	if err != nil {
		return nil, s.err.New("上传到 OSS 失败", err)
	}

	s.log.WithFields(map[string]interface{}{
		"key":    fullKey,
		"size":   len(content),
		"sha256": sha256Sum,
	}).Info("文件上传到 OSS 成功")

	return &StoredObject{
		Key:         key,
		Size:        int64(len(content)),
		ContentType: contentType,
		SHA256:      sha256Sum,
		CreatedAt:   time.Now(),
	}, nil
}

// Open 从 OSS 下载文件
func (s *OSSStorage) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	fullKey := s.fullKey(key)

	reader, err := s.ossService.DownloadFile(ctx, fullKey)
	if err != nil {
		return nil, s.err.New("从 OSS 下载失败", err)
	}

	return reader, nil
}

// Delete 从 OSS 删除文件
func (s *OSSStorage) Delete(ctx context.Context, key string) error {
	fullKey := s.fullKey(key)

	err := s.ossService.DeleteFile(ctx, fullKey)
	if err != nil {
		return s.err.New("从 OSS 删除失败", err)
	}

	s.log.WithField("key", fullKey).Info("文件从 OSS 删除成功")
	return nil
}

// Stat 获取 OSS 文件信息
func (s *OSSStorage) Stat(ctx context.Context, key string) (*StoredObject, error) {
	// 尝试下载来验证是否存在（OSS 没有直接的 stat 方法）
	reader, err := s.Open(ctx, key)
	if err != nil {
		return nil, s.err.New("文件不存在或无法访问", err).WithCode(errorc.ErrorCodeNotFound)
	}
	reader.Close()

	return &StoredObject{
		Key: key,
	}, nil
}

// Exists 检查 OSS 文件是否存在
func (s *OSSStorage) Exists(ctx context.Context, key string) (bool, error) {
	// 尝试下载来验证是否存在
	reader, err := s.Open(ctx, key)
	if err != nil {
		// 假设错误表示文件不存在
		return false, nil
	}
	reader.Close()

	return true, nil
}

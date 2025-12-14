package storage

import (
	"context"
	"io"
	"time"
)

// StoredObject 存储对象元信息
type StoredObject struct {
	Key         string    `json:"key"`
	Size        int64     `json:"size"`
	ContentType string    `json:"contentType"`
	SHA256      string    `json:"sha256"`
	CreatedAt   time.Time `json:"createdAt"`
}

// Storage 存储接口
// 抽象产物存储能力，支持本地和 OSS 两种实现
type Storage interface {
	// Put 上传文件
	// key: 存储键
	// reader: 文件内容读取器
	// size: 文件大小
	// contentType: MIME 类型
	Put(ctx context.Context, key string, reader io.Reader, size int64, contentType string) (*StoredObject, error)

	// Open 打开文件用于读取
	Open(ctx context.Context, key string) (io.ReadCloser, error)

	// Delete 删除文件
	Delete(ctx context.Context, key string) error

	// Stat 获取文件信息
	Stat(ctx context.Context, key string) (*StoredObject, error)

	// Exists 检查文件是否存在
	Exists(ctx context.Context, key string) (bool, error)

	// Mode 返回存储模式标识
	Mode() string
}


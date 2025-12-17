package sdk

import (
	"context"
	"io"

	applicationpb "xiaozhizhang/system/application/api/proto"
)

// UploadArtifactResponse 上传产物响应
type UploadArtifactResponse struct {
	ArtifactID  int64
	ObjectKey   string
	StorageMode string
}

// UploadArtifactFromReader 流式上传产物（易用封装）
// meta: 产物元数据（必须在第一个chunk发送）
// r: 数据流
// chunkSize: 分块大小（字节），建议 64KB - 1MB，如果为 0 则使用默认值 512KB
func (c *ApplicationClient) UploadArtifactFromReader(
	ctx context.Context,
	meta *applicationpb.ArtifactMeta,
	r io.Reader,
	chunkSize int,
) (*UploadArtifactResponse, error) {
	// 设置默认分块大小
	if chunkSize <= 0 {
		chunkSize = 512 * 1024 // 512KB
	}

	// 创建流式上传客户端
	stream, err := c.service.UploadArtifact(ctx)
	if err != nil {
		return nil, WrapError(err, "create upload stream failed")
	}

	// 第一个 chunk 包含元数据
	firstChunk := &applicationpb.UploadArtifactChunk{
		Meta: meta,
		Data: nil,
	}

	if err := stream.Send(firstChunk); err != nil {
		return nil, WrapError(err, "send meta chunk failed")
	}

	// 循环读取并发送数据块
	buffer := make([]byte, chunkSize)
	for {
		n, err := r.Read(buffer)
		if n > 0 {
			chunk := &applicationpb.UploadArtifactChunk{
				Meta: nil,
				Data: buffer[:n],
			}
			if sendErr := stream.Send(chunk); sendErr != nil {
				return nil, WrapError(sendErr, "send data chunk failed")
			}
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, WrapError(err, "read data failed")
		}
	}

	// 关闭发送并接收响应
	resp, err := stream.CloseAndRecv()
	if err != nil {
		return nil, WrapError(err, "close and receive failed")
	}

	return &UploadArtifactResponse{
		ArtifactID:  resp.ArtifactId,
		ObjectKey:   resp.ObjectKey,
		StorageMode: resp.StorageMode,
	}, nil
}


package app

import (
	"context"
	"fmt"
	"io"
	"time"

	"xiaozhizhang/system/application/internal/model"
)

// UploadArtifactRequest 上传产物请求
type UploadArtifactRequest struct {
	ApplicationID int64
	Type          model.ArtifactType
	FileName      string
	Size          int64
	ContentType   string
	Reader        io.Reader
}

// UploadArtifact 上传产物
func (a *App) UploadArtifact(ctx context.Context, req *UploadArtifactRequest) (*model.Artifact, error) {
	a.log.WithFields(map[string]interface{}{
		"applicationId": req.ApplicationID,
		"fileName":      req.FileName,
		"size":          req.Size,
		"type":          req.Type,
	}).Info("开始上传产物")

	// 1. 验证应用存在
	_, err := a.ApplicationSvc.FindByID(ctx, req.ApplicationID)
	if err != nil {
		return nil, err
	}

	// 2. 生成存储 key
	objectKey := fmt.Sprintf("%d/%s/%s", req.ApplicationID, time.Now().Format("20060102150405"), req.FileName)

	// 3. 上传到存储
	storedObject, err := a.Storage.Put(ctx, objectKey, req.Reader, req.Size, req.ContentType)
	if err != nil {
		return nil, a.err.New("上传产物失败", err)
	}

	// 4. 创建产物记录
	artifact := &model.Artifact{
		ApplicationID: req.ApplicationID,
		Type:          req.Type,
		StorageMode:   a.StorageMode,
		ObjectKey:     objectKey,
		FileName:      req.FileName,
		Size:          storedObject.Size,
		SHA256:        storedObject.SHA256,
		ContentType:   req.ContentType,
	}

	if err := a.ArtifactSvc.Create(ctx, artifact); err != nil {
		// 尝试删除已上传的文件
		a.Storage.Delete(ctx, objectKey)
		return nil, err
	}

	a.log.WithFields(map[string]interface{}{
		"artifactId": artifact.ID,
		"objectKey":  objectKey,
		"sha256":     storedObject.SHA256,
	}).Info("产物上传成功")

	return artifact, nil
}

// GetArtifact 获取产物信息
func (a *App) GetArtifact(ctx context.Context, id int64) (*model.Artifact, error) {
	return a.ArtifactSvc.FindByID(ctx, id)
}

// ListArtifacts 列出应用的产物
func (a *App) ListArtifacts(ctx context.Context, applicationID int64) ([]*model.Artifact, error) {
	return a.ArtifactSvc.ListByApplicationID(ctx, applicationID)
}

// DeleteArtifact 删除产物
func (a *App) DeleteArtifact(ctx context.Context, id int64) error {
	// 获取产物信息
	artifact, err := a.ArtifactSvc.FindByID(ctx, id)
	if err != nil {
		return err
	}

	// 从存储删除
	if err := a.Storage.Delete(ctx, artifact.ObjectKey); err != nil {
		a.log.WithErr(err).Warn("删除存储文件失败")
		// 继续删除数据库记录
	}

	// 删除数据库记录
	return a.ArtifactSvc.Delete(ctx, id)
}


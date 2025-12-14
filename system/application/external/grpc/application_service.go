package grpc

import (
	"context"
	"encoding/json"
	"io"

	errorc "xiaozhizhang/pkg/core/err"
	"xiaozhizhang/pkg/core/logger"
	"xiaozhizhang/system/application/api/proto"
	"xiaozhizhang/system/application/internal/app"
	"xiaozhizhang/system/application/internal/model"
	"xiaozhizhang/system/application/internal/model/dto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ApplicationService gRPC Application 服务实现
type ApplicationService struct {
	proto.UnimplementedApplicationServiceServer
	app *app.App
	err *errorc.ErrorBuilder
	log *logger.Log
}

// NewApplicationService 创建 Application gRPC 服务实例
func NewApplicationService(appInstance *app.App) *ApplicationService {
	return &ApplicationService{
		app: appInstance,
		err: errorc.NewErrorBuilder("ApplicationGRPCService"),
		log: logger.GetLogger().WithEntryName("ApplicationGRPCService"),
	}
}

// ServiceName 返回服务名称
func (s *ApplicationService) ServiceName() string {
	return "application.v1.ApplicationService"
}

// ServiceVersion 返回服务版本
func (s *ApplicationService) ServiceVersion() string {
	return "v1.0.0"
}

// RegisterService 注册服务到 gRPC 服务器
func (s *ApplicationService) RegisterService(server *grpc.Server) error {
	proto.RegisterApplicationServiceServer(server, s)
	return nil
}

// Deploy 触发部署
func (s *ApplicationService) Deploy(ctx context.Context, req *proto.DeployRequest) (*proto.DeployResponse, error) {
	s.log.WithFields(map[string]interface{}{
		"applicationId": req.ApplicationId,
		"version":       req.Version,
	}).Info("gRPC Deploy 请求")

	// 解析 spec JSON
	var spec *dto.DeploySpec
	if req.SpecJson != "" {
		if err := json.Unmarshal([]byte(req.SpecJson), &spec); err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "解析 spec 失败: %v", err)
		}
	}

	// 调用内部 app 部署
	deployReq := &dto.DeployRequest{
		ApplicationID:      req.ApplicationId,
		Version:            req.Version,
		BackendArtifactID:  req.BackendArtifactId,
		FrontendArtifactID: req.FrontendArtifactId,
		Spec:               spec,
		Operator:           req.Operator,
	}

	deployment, err := s.app.Deploy(ctx, deployReq)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "部署失败: %v", err)
	}

	return &proto.DeployResponse{
		DeploymentId: deployment.ID,
		ReleaseId:    deployment.ReleaseID,
		Status:       string(deployment.Status),
	}, nil
}

// GetDeployment 获取部署状态
func (s *ApplicationService) GetDeployment(ctx context.Context, req *proto.GetDeploymentRequest) (*proto.DeploymentInfo, error) {
	deployment, err := s.app.GetDeployment(ctx, req.DeploymentId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "部署记录不存在: %v", err)
	}

	// 获取关联的 release 信息
	release, _ := s.app.ReleaseSvc.FindByID(ctx, deployment.ReleaseID)
	version := ""
	if release != nil {
		version = release.Version
	}

	// 转换日志
	var logs []string
	if deployment.Logs != nil {
		for _, v := range deployment.Logs {
			if str, ok := v.(string); ok {
				logs = append(logs, str)
			}
		}
	}

	// 转换时间
	var startedAt, finishedAt int64
	if deployment.StartedAt != nil {
		startedAt = deployment.StartedAt.Unix()
	}
	if deployment.FinishedAt != nil {
		finishedAt = deployment.FinishedAt.Unix()
	}

	return &proto.DeploymentInfo{
		Id:            deployment.ID,
		ApplicationId: deployment.ApplicationID,
		ReleaseId:     deployment.ReleaseID,
		Version:       version,
		Action:        string(deployment.Action),
		Status:        string(deployment.Status),
		StartedAt:     startedAt,
		FinishedAt:    finishedAt,
		Logs:          logs,
		ErrorMessage:  deployment.ErrorMessage,
		Operator:      deployment.Operator,
	}, nil
}

// UploadArtifact 流式上传产物
func (s *ApplicationService) UploadArtifact(stream grpc.ClientStreamingServer[proto.UploadArtifactChunk, proto.UploadArtifactResponse]) error {
	ctx := stream.Context()

	var meta *proto.ArtifactMeta
	pr, pw := io.Pipe()
	defer pr.Close()

	// 启动一个 goroutine 接收数据
	errCh := make(chan error, 1)
	var artifact *model.Artifact

	go func() {
		defer pw.Close()

		for {
			chunk, err := stream.Recv()
			if err == io.EOF {
				return
			}
			if err != nil {
				errCh <- err
				return
			}

			// 第一个 chunk 包含元数据
			if meta == nil && chunk.Meta != nil {
				meta = chunk.Meta
			}

			// 写入数据
			if len(chunk.Data) > 0 {
				if _, err := pw.Write(chunk.Data); err != nil {
					errCh <- err
					return
				}
			}
		}
	}()

	// 等待元数据
	for meta == nil {
		select {
		case err := <-errCh:
			return status.Errorf(codes.Internal, "接收数据失败: %v", err)
		case <-ctx.Done():
			return status.Errorf(codes.Canceled, "请求已取消")
		default:
			// 继续等待
		}
	}

	// 调用上传
	req := &app.UploadArtifactRequest{
		ApplicationID: meta.ApplicationId,
		Type:          model.ArtifactType(meta.ArtifactType),
		FileName:      meta.FileName,
		Size:          meta.Size,
		ContentType:   meta.ContentType,
		Reader:        pr,
	}

	var uploadErr error
	artifact, uploadErr = s.app.UploadArtifact(ctx, req)

	// 检查接收错误
	select {
	case err := <-errCh:
		if err != nil {
			return status.Errorf(codes.Internal, "接收数据失败: %v", err)
		}
	default:
	}

	if uploadErr != nil {
		return status.Errorf(codes.Internal, "上传失败: %v", uploadErr)
	}

	// 发送响应
	return stream.SendAndClose(&proto.UploadArtifactResponse{
		ArtifactId:  artifact.ID,
		ObjectKey:   artifact.ObjectKey,
		StorageMode: string(artifact.StorageMode),
	})
}

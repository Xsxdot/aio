package sdk

import (
	"context"

	applicationpb "xiaozhizhang/system/application/api/proto"

	"google.golang.org/grpc"
)

// ApplicationClient 应用部署客户端
type ApplicationClient struct {
	service applicationpb.ApplicationServiceClient
}

// newApplicationClient 创建应用部署客户端
func newApplicationClient(conn *grpc.ClientConn) *ApplicationClient {
	return &ApplicationClient{
		service: applicationpb.NewApplicationServiceClient(conn),
	}
}

// DeployRequest 部署请求（SDK 简化版）
type DeployRequest struct {
	ApplicationID      int64
	Version            string
	BackendArtifactID  int64
	FrontendArtifactID int64
	SpecJSON           string // JSON 格式的 DeploySpec
	Operator           string
}

// DeployResponse 部署响应
type DeployResponse struct {
	DeploymentID int64
	ReleaseID    int64
	Status       string
}

// Deploy 触发部署
func (c *ApplicationClient) Deploy(ctx context.Context, req *DeployRequest) (*DeployResponse, error) {
	pbReq := &applicationpb.DeployRequest{
		ApplicationId:      req.ApplicationID,
		Version:            req.Version,
		BackendArtifactId:  req.BackendArtifactID,
		FrontendArtifactId: req.FrontendArtifactID,
		SpecJson:           req.SpecJSON,
		Operator:           req.Operator,
	}

	resp, err := c.service.Deploy(ctx, pbReq)
	if err != nil {
		return nil, WrapError(err, "deploy failed")
	}

	return &DeployResponse{
		DeploymentID: resp.DeploymentId,
		ReleaseID:    resp.ReleaseId,
		Status:       resp.Status,
	}, nil
}

// DeploymentInfo 部署状态信息
type DeploymentInfo struct {
	ID            int64
	ApplicationID int64
	ReleaseID     int64
	Version       string
	Action        string
	Status        string
	StartedAt     int64
	FinishedAt    int64
	Logs          []string
	ErrorMessage  string
	Operator      string
}

// GetDeployment 获取部署状态
func (c *ApplicationClient) GetDeployment(ctx context.Context, deploymentID int64) (*DeploymentInfo, error) {
	req := &applicationpb.GetDeploymentRequest{
		DeploymentId: deploymentID,
	}

	resp, err := c.service.GetDeployment(ctx, req)
	if err != nil {
		return nil, WrapError(err, "get deployment failed")
	}

	return &DeploymentInfo{
		ID:            resp.Id,
		ApplicationID: resp.ApplicationId,
		ReleaseID:     resp.ReleaseId,
		Version:       resp.Version,
		Action:        resp.Action,
		Status:        resp.Status,
		StartedAt:     resp.StartedAt,
		FinishedAt:    resp.FinishedAt,
		Logs:          resp.Logs,
		ErrorMessage:  resp.ErrorMessage,
		Operator:      resp.Operator,
	}, nil
}


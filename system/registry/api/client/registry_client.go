package client

import (
	"context"

	errorc "github.com/xsxdot/aio/pkg/core/err"
	"github.com/xsxdot/aio/system/registry/api/dto"
	internalapp "github.com/xsxdot/aio/system/registry/internal/app"
)

// RegistryClient 注册中心对外客户端（进程内调用）
// 对外只暴露 DTO，禁止泄漏 internal/model。
type RegistryClient struct {
	app *internalapp.App
	err *errorc.ErrorBuilder
}

func NewRegistryClient(app *internalapp.App) *RegistryClient {
	return &RegistryClient{
		app: app,
		err: errorc.NewErrorBuilder("RegistryClient"),
	}
}

// ==================== Service Definition CRUD ====================

// CreateService 创建服务定义
func (c *RegistryClient) CreateService(ctx context.Context, req *dto.CreateServiceReq) (*dto.ServiceDTO, error) {
	return c.app.CreateService(ctx, req.Project, req.Name, req.Owner, req.Description, req.Spec)
}

// UpdateService 更新服务定义
func (c *RegistryClient) UpdateService(ctx context.Context, id int64, req *dto.UpdateServiceReq) (*dto.ServiceDTO, error) {
	return c.app.UpdateService(ctx, id, req.Project, req.Name, req.Owner, req.Description, req.Spec)
}

// GetServiceDefByID 根据 ID 获取服务定义（不包含实例）
func (c *RegistryClient) GetServiceDefByID(ctx context.Context, id int64) (*dto.ServiceDTO, error) {
	return c.app.GetServiceDefByID(ctx, id)
}

// ListServiceDefs 查询服务定义列表（不包含实例）
func (c *RegistryClient) ListServiceDefs(ctx context.Context, project string) ([]*dto.ServiceDTO, error) {
	return c.app.ListServiceDefs(ctx, project)
}

// DeleteService 删除服务定义
func (c *RegistryClient) DeleteService(ctx context.Context, id int64) error {
	return c.app.DeleteService(ctx, id)
}

// EnsureService 确保服务定义存在，不存在则创建，存在则返回现有记录
// 返回值：服务DTO, 是否新创建, 错误
func (c *RegistryClient) EnsureService(ctx context.Context, req *dto.CreateServiceReq) (*dto.ServiceDTO, bool, error) {
	// 先尝试按 project+name 查找
	services, err := c.app.ListServiceDefs(ctx, req.Project)
	if err != nil {
		return nil, false, err
	}

	// 查找同名服务
	for _, svc := range services {
		if svc.Name == req.Name {
			return svc, false, nil // 已存在
		}
	}

	// 不存在，创建新服务
	newSvc, err := c.CreateService(ctx, req)
	if err != nil {
		return nil, false, err
	}

	return newSvc, true, nil
}

// ==================== Service With Instances ====================

// ListServices 供 agent 拉取服务列表（包含在线实例）
func (c *RegistryClient) ListServices(ctx context.Context, project, env string) ([]*dto.ServiceWithInstancesDTO, error) {
	return c.app.ListServices(ctx, project, env)
}

// GetServiceByID 供 agent 拉取单服务详情
func (c *RegistryClient) GetServiceByID(ctx context.Context, id int64) (*dto.ServiceWithInstancesDTO, error) {
	return c.app.GetServiceByID(ctx, id)
}

// ==================== Instance Management ====================

// RegisterInstance 注册或更新实例
func (c *RegistryClient) RegisterInstance(ctx context.Context, req *dto.RegisterInstanceReq) (*dto.RegisterInstanceResp, error) {
	return c.app.RegisterInstance(ctx, req)
}

// HeartbeatInstance 实例心跳续租
func (c *RegistryClient) HeartbeatInstance(ctx context.Context, req *dto.HeartbeatReq) (*dto.HeartbeatResp, error) {
	return c.app.HeartbeatInstance(ctx, req)
}

// DeregisterInstance 实例下线（可选）
func (c *RegistryClient) DeregisterInstance(ctx context.Context, req *dto.DeregisterInstanceReq) error {
	return c.app.DeregisterInstance(ctx, req)
}

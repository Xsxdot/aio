package app

import (
	"context"
	"time"

	errorc "github.com/xsxdot/aio/pkg/core/err"
	"github.com/xsxdot/aio/system/registry/api/dto"
	"github.com/xsxdot/aio/system/registry/internal/model"
)

func (a *App) toServiceDTO(s *model.RegistryService) *dto.ServiceDTO {
	if s == nil {
		return nil
	}
	return &dto.ServiceDTO{
		ID:          s.ID,
		Project:     s.Project,
		Name:        s.Name,
		Owner:       s.Owner,
		Description: s.Description,
		Spec:        map[string]interface{}(s.Spec),
		CreatedAt:   s.CreatedAt,
		UpdatedAt:   s.UpdatedAt,
	}
}

// ===== admin: service definition CRUD =====

func (a *App) CreateService(ctx context.Context, project, name, owner, description string, spec map[string]interface{}) (*dto.ServiceDTO, error) {
	m := &model.RegistryService{
		Project:     project,
		Name:        name,
		Owner:       owner,
		Description: description,
		Spec:        spec,
	}
	if err := a.ServiceDao.Create(ctx, m); err != nil {
		return nil, err
	}
	return a.toServiceDTO(m), nil
}

func (a *App) ListServiceDefs(ctx context.Context, project string) ([]*dto.ServiceDTO, error) {
	list, err := a.ServiceSvc.ListByFilter(ctx, project)
	if err != nil {
		return nil, err
	}
	out := make([]*dto.ServiceDTO, 0, len(list))
	for _, s := range list {
		out = append(out, a.toServiceDTO(s))
	}
	return out, nil
}

func (a *App) GetServiceDefByID(ctx context.Context, id int64) (*dto.ServiceDTO, error) {
	svc, err := a.ServiceDao.FindById(ctx, id)
	if err != nil {
		return nil, err
	}
	return a.toServiceDTO(svc), nil
}

func (a *App) UpdateService(ctx context.Context, id int64, project, name, owner, description string, spec map[string]interface{}) (*dto.ServiceDTO, error) {
	exist, err := a.ServiceDao.FindById(ctx, id)
	if err != nil {
		return nil, err
	}

	update := &model.RegistryService{}
	// 字符串字段：非空才更新
	if project != "" {
		update.Project = project
	}
	if name != "" {
		update.Name = name
	}
	if owner != "" {
		update.Owner = owner
	}
	if description != "" {
		update.Description = description
	}
	if spec != nil {
		update.Spec = spec
	}

	_, err = a.ServiceDao.UpdateById(ctx, id, update)
	if err != nil {
		return nil, err
	}

	// 重新读取（避免部分更新导致返回值不完整）
	after, err := a.ServiceDao.FindById(ctx, id)
	if err != nil {
		return nil, err
	}
	// 保留 create time 等
	after.CreatedAt = exist.CreatedAt
	return a.toServiceDTO(after), nil
}

func (a *App) DeleteService(ctx context.Context, id int64) error {
	return a.ServiceDao.DeleteById(ctx, id)
}

// ===== agent: pull services + online instances =====

func (a *App) ListServices(ctx context.Context, project, env string) ([]*dto.ServiceWithInstancesDTO, error) {
	services, err := a.ServiceSvc.ListByFilter(ctx, project)
	if err != nil {
		return nil, err
	}

	out := make([]*dto.ServiceWithInstancesDTO, 0, len(services))
	for _, s := range services {
		instances, err := a.listOnlineInstancesByServiceIDAndEnv(ctx, s.ID, env)
		if err != nil {
			return nil, err
		}
		out = append(out, &dto.ServiceWithInstancesDTO{
			Service:   a.toServiceDTO(s),
			Instances: instances,
		})
	}
	return out, nil
}

func (a *App) GetServiceByID(ctx context.Context, id int64) (*dto.ServiceWithInstancesDTO, error) {
	svc, err := a.ServiceDao.FindById(ctx, id)
	if err != nil {
		return nil, err
	}
	instances, err := a.listOnlineInstancesByServiceID(ctx, svc.ID)
	if err != nil {
		return nil, err
	}
	return &dto.ServiceWithInstancesDTO{Service: a.toServiceDTO(svc), Instances: instances}, nil
}

func (a *App) isAliveByHeartbeat(now time.Time, inst *model.RegistryInstance) bool {
	if inst == nil {
		return false
	}
	ttl := inst.TTLSeconds
	if ttl <= 0 {
		ttl = 60
	}
	return now.Sub(inst.LastHeartbeatAt) <= time.Duration(ttl)*time.Second
}

func (a *App) listOnlineInstancesByServiceID(ctx context.Context, serviceID int64) ([]*dto.InstanceDTO, error) {
	return a.listOnlineInstancesByServiceIDAndEnv(ctx, serviceID, "")
}

func (a *App) listOnlineInstancesByServiceIDAndEnv(ctx context.Context, serviceID int64, env string) ([]*dto.InstanceDTO, error) {
	// 优先从 Redis 取在线实例
	if a.rdb != nil {
		instances, ok, err := a.listOnlineInstancesFromRedis(ctx, serviceID)
		if err != nil {
			// Redis 异常时回退到 MySQL
			a.log.WithErr(err).Warn("读取 Redis 在线实例失败，回退 MySQL")
		} else if ok {
			// 按 env 过滤
			if env != "" {
				filtered := make([]*dto.InstanceDTO, 0, len(instances))
				for _, inst := range instances {
					if inst.Env == env {
						filtered = append(filtered, inst)
					}
				}
				return filtered, nil
			}
			return instances, nil
		}
	}

	// 回退：从 MySQL 读取并按 lastHeartbeatAt + ttlSeconds 判断在线
	list, err := a.InstanceSvc.ListByServiceIDAndEnv(ctx, serviceID, env)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	out := make([]*dto.InstanceDTO, 0, len(list))
	for _, inst := range list {
		if !a.isAliveByHeartbeat(now, inst) {
			continue
		}
		out = append(out, a.toInstanceDTO(inst))
	}
	return out, nil
}

func (a *App) ensureServiceExists(ctx context.Context, serviceID int64) error {
	_, err := a.ServiceDao.FindById(ctx, serviceID)
	if err != nil {
		return err
	}
	return nil
}

func (a *App) notFound(msg string) error {
	return a.err.New(msg, nil).WithCode(errorc.ErrorCodeNotFound)
}

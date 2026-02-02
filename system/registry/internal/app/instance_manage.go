package app

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	errorc "github.com/xsxdot/aio/pkg/core/err"
	"github.com/xsxdot/aio/system/registry/api/dto"
	"github.com/xsxdot/aio/system/registry/internal/model"

	"github.com/google/uuid"
)

const (
	defaultInstanceTTLSeconds int64 = 60
)

func (a *App) instanceRedisKey(serviceID int64, instanceKey string) string {
	return fmt.Sprintf("registry:instance:%d:%s", serviceID, instanceKey)
}

func (a *App) instanceRedisPattern(serviceID int64) string {
	return fmt.Sprintf("registry:instance:%d:*", serviceID)
}

func (a *App) toInstanceDTO(inst *model.RegistryInstance) *dto.InstanceDTO {
	if inst == nil {
		return nil
	}
	return &dto.InstanceDTO{
		ID:              inst.ID,
		ServiceID:       inst.ServiceID,
		InstanceKey:     inst.InstanceKey,
		Env:             inst.Env,
		Host:            inst.Host,
		Endpoint:        inst.Endpoint,
		Meta:            map[string]interface{}(inst.Meta),
		TTLSeconds:      inst.TTLSeconds,
		LastHeartbeatAt: inst.LastHeartbeatAt,
		CreatedAt:       inst.CreatedAt,
		UpdatedAt:       inst.UpdatedAt,
	}
}

func (a *App) RegisterInstance(ctx context.Context, req *dto.RegisterInstanceReq) (*dto.RegisterInstanceResp, error) {
	if req == nil {
		return nil, a.err.New("请求不能为空", nil).ValidWithCtx()
	}
	if req.Env == "" {
		return nil, a.err.New("env 不能为空", nil).ValidWithCtx()
	}
	if err := a.ensureServiceExists(ctx, req.ServiceID); err != nil {
		return nil, err
	}

	instanceKey := req.InstanceKey
	if instanceKey == "" {
		instanceKey = uuid.NewString()
	}

	ttl := req.TTLSeconds
	if ttl <= 0 {
		ttl = defaultInstanceTTLSeconds
	}

	now := time.Now()
	inst := &model.RegistryInstance{
		ServiceID:       req.ServiceID,
		InstanceKey:     instanceKey,
		Env:             req.Env,
		Host:            req.Host,
		Endpoint:        req.Endpoint,
		Meta:            req.Meta,
		TTLSeconds:      ttl,
		LastHeartbeatAt: now,
	}
	if err := a.InstanceSvc.Upsert(ctx, inst); err != nil {
		return nil, err
	}

	expiresAt := now.Add(time.Duration(ttl) * time.Second)
	if err := a.saveInstanceToRedis(ctx, inst, expiresAt); err != nil {
		// Redis 写失败不影响主流程（MySQL 仍是持久化真相源）
		a.log.WithErr(err).Warn("写入 Redis 实例租约失败")
	}

	return &dto.RegisterInstanceResp{InstanceKey: instanceKey, ExpiresAt: expiresAt}, nil
}

func (a *App) HeartbeatInstance(ctx context.Context, req *dto.HeartbeatReq) (*dto.HeartbeatResp, error) {
	if req == nil {
		return nil, a.err.New("请求不能为空", nil).ValidWithCtx()
	}

	now := time.Now()
	// 先更新 MySQL 心跳
	if err := a.InstanceSvc.UpdateHeartbeat(ctx, req.ServiceID, req.InstanceKey, now); err != nil {
		return nil, err
	}

	// 获取实例信息（用于 ttl 与 redis value）
	inst, err := a.InstanceSvc.FindByServiceAndKey(ctx, req.ServiceID, req.InstanceKey)
	if err != nil {
		return nil, err
	}

	ttl := inst.TTLSeconds
	if ttl <= 0 {
		ttl = defaultInstanceTTLSeconds
	}
	expiresAt := now.Add(time.Duration(ttl) * time.Second)

	if err := a.saveInstanceToRedis(ctx, inst, expiresAt); err != nil {
		a.log.WithErr(err).Warn("写入 Redis 实例心跳失败")
	}

	return &dto.HeartbeatResp{ExpiresAt: expiresAt}, nil
}

func (a *App) DeregisterInstance(ctx context.Context, req *dto.DeregisterInstanceReq) error {
	if req == nil {
		return a.err.New("请求不能为空", nil).ValidWithCtx()
	}

	// 删除 Redis 租约
	if a.rdb != nil {
		_ = a.rdb.Del(ctx, a.instanceRedisKey(req.ServiceID, req.InstanceKey)).Err()
	}

	// 删除 MySQL 记录（软删）
	return a.InstanceSvc.DeleteByServiceAndKey(ctx, req.ServiceID, req.InstanceKey)
}

func (a *App) saveInstanceToRedis(ctx context.Context, inst *model.RegistryInstance, expiresAt time.Time) error {
	if a.rdb == nil {
		return nil
	}

	payload := a.toInstanceDTO(inst)
	if payload == nil {
		return a.err.New("实例数据为空", nil).WithCode(errorc.ErrorCodeInternal)
	}

	b, err := json.Marshal(payload)
	if err != nil {
		return a.err.New("序列化实例数据失败", err)
	}

	ttl := time.Until(expiresAt)
	if ttl <= 0 {
		ttl = time.Duration(defaultInstanceTTLSeconds) * time.Second
	}

	return a.rdb.Set(ctx, a.instanceRedisKey(inst.ServiceID, inst.InstanceKey), string(b), ttl).Err()
}

// listOnlineInstancesFromRedis 尝试从 Redis 读取在线实例
// ok=false 表示 Redis 中没有数据（而不是错误）。
func (a *App) listOnlineInstancesFromRedis(ctx context.Context, serviceID int64) (instances []*dto.InstanceDTO, ok bool, err error) {
	if a.rdb == nil {
		return nil, false, nil
	}

	pattern := a.instanceRedisPattern(serviceID)
	iter := a.rdb.Scan(ctx, 0, pattern, 200).Iterator()
	keys := make([]string, 0, 32)
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}
	if err := iter.Err(); err != nil {
		return nil, false, err
	}
	if len(keys) == 0 {
		return nil, false, nil
	}

	values, err := a.rdb.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, false, err
	}

	out := make([]*dto.InstanceDTO, 0, len(values))
	for _, v := range values {
		if v == nil {
			continue
		}
		s, _ := v.(string)
		if s == "" {
			continue
		}
		var item dto.InstanceDTO
		if err := json.Unmarshal([]byte(s), &item); err != nil {
			// 单条坏数据不影响整体
			continue
		}
		out = append(out, &item)
	}

	return out, true, nil
}

// ===== admin: instance management =====

// ListInstancesForAdmin 管理员查看实例列表（支持查看所有或仅在线）
func (a *App) ListInstancesForAdmin(ctx context.Context, serviceID int64, env string, aliveOnly bool) ([]*dto.InstanceDTO, error) {
	if aliveOnly {
		// 只看在线实例（复用现有逻辑：优先 Redis，回退 MySQL + alive 判断）
		return a.listOnlineInstancesByServiceIDAndEnv(ctx, serviceID, env)
	}

	// 查看所有实例（MySQL 中的记录，不过滤在线状态）
	list, err := a.InstanceSvc.ListByServiceIDAndEnv(ctx, serviceID, env)
	if err != nil {
		return nil, err
	}

	out := make([]*dto.InstanceDTO, 0, len(list))
	for _, inst := range list {
		out = append(out, a.toInstanceDTO(inst))
	}
	return out, nil
}

// ForceOfflineInstance 管理员强制下线实例（删除 Redis 租约 + 将 MySQL 心跳时间置为极早，确保回退 MySQL 时也判定为离线）
func (a *App) ForceOfflineInstance(ctx context.Context, serviceID int64, instanceKey string) error {
	// 删除 Redis 租约
	if a.rdb != nil {
		_ = a.rdb.Del(ctx, a.instanceRedisKey(serviceID, instanceKey)).Err()
	}

	// 将 MySQL 中的 last_heartbeat_at 更新为极早时间（1970-01-01），确保 isAliveByHeartbeat 判定为离线
	return a.InstanceSvc.UpdateHeartbeat(ctx, serviceID, instanceKey, time.Unix(0, 0))
}

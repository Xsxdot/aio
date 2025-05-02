package etcd

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/xsxdot/aio/pkg/utils"

	"github.com/gofiber/fiber/v2"
	"go.etcd.io/etcd/api/v3/etcdserverpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"
)

// API 表示ETCD API处理器
type API struct {
	server *EtcdServer
	client *EtcdClient
	logger *zap.Logger
}

// NewAPI 创建一个新的ETCD API处理器
func NewAPI(server *EtcdServer, client *EtcdClient, logger *zap.Logger) *API {
	if logger == nil {
		var err error
		logger, err = zap.NewProduction()
		if err != nil {
			panic(fmt.Sprintf("创建日志记录器失败: %v", err))
		}
	}

	return &API{
		server: server,
		client: client,
		logger: logger,
	}
}

// RegisterRoutes 注册API路由
func (a *API) RegisterRoutes(router fiber.Router, authHandler func(*fiber.Ctx) error, adminRoleHandler func(*fiber.Ctx) error) {
	api := router.Group("/etcd")

	// 集群状态管理
	api.Get("/status", authHandler, adminRoleHandler, a.GetClusterStatus)

	// 成员管理
	api.Get("/members", authHandler, adminRoleHandler, a.GetMembers)
	api.Post("/members", authHandler, adminRoleHandler, a.AddMember)
	api.Put("/members/:id", authHandler, adminRoleHandler, a.UpdateMember)
	api.Delete("/members/:id", authHandler, adminRoleHandler, a.RemoveMember)

	// 键值管理
	api.Get("/kv/:key", authHandler, adminRoleHandler, a.GetValue)
	api.Get("/kv", authHandler, adminRoleHandler, a.GetValuesWithPrefix)
	api.Put("/kv/:key", authHandler, adminRoleHandler, a.PutValue)
	api.Delete("/kv/:key", authHandler, adminRoleHandler, a.DeleteValue)

	// 租约管理
	api.Get("/leases", authHandler, adminRoleHandler, a.GetAllLeases)
	api.Post("/leases", authHandler, adminRoleHandler, a.CreateLease)
	api.Put("/leases/:id/keepalive", authHandler, adminRoleHandler, a.KeepAlive)
	api.Delete("/leases/:id", authHandler, adminRoleHandler, a.RevokeLease)

	// 认证管理
	api.Put("/auth/enable", authHandler, adminRoleHandler, a.EnableAuth)
	api.Put("/auth/disable", authHandler, adminRoleHandler, a.DisableAuth)
}

// =================================
// 集群状态管理
// =================================

// GetClusterStatus 获取集群状态
func (a *API) GetClusterStatus(c *fiber.Ctx) error {
	if a.client == nil || a.client.Client == nil {
		return utils.FailResponse(c, utils.StatusInternalError, "ETCD客户端未初始化")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 获取成员列表
	memberListResp, err := a.client.Client.MemberList(ctx)
	if err != nil {
		a.logger.Error("获取成员列表失败", zap.Error(err))
		return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("获取成员列表失败: %v", err))
	}

	// 获取当前集群状态
	endpoint := a.client.Client.Endpoints()[0]
	statusResp, err := a.client.Client.Status(ctx, endpoint)
	if err != nil {
		a.logger.Error("获取集群状态失败", zap.Error(err))
		return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("获取集群状态失败: %v", err))
	}

	// 转换成员数据
	members := make([]map[string]interface{}, 0, len(memberListResp.Members))
	for _, m := range memberListResp.Members {
		members = append(members, map[string]interface{}{
			"id":          fmt.Sprintf("%x", m.ID),
			"name":        m.Name,
			"peer_urls":   m.PeerURLs,
			"client_urls": m.ClientURLs,
			"is_learner":  m.IsLearner,
		})
	}

	// 构造响应
	statusData := map[string]interface{}{
		"cluster_id":     fmt.Sprintf("%x", statusResp.Header.ClusterId),
		"member_id":      fmt.Sprintf("%x", statusResp.Header.MemberId),
		"leader":         fmt.Sprintf("%x", statusResp.Leader),
		"members":        members,
		"version":        statusResp.Version,
		"db_size":        statusResp.DbSize,
		"db_size_in_use": statusResp.DbSizeInUse,
		"is_learner":     statusResp.IsLearner,
		"raft_index":     statusResp.RaftIndex,
		"raft_term":      statusResp.RaftTerm,
	}

	return utils.SuccessResponse(c, statusData)
}

// =================================
// 成员管理
// =================================

// GetMembers 获取集群成员列表
func (a *API) GetMembers(c *fiber.Ctx) error {
	if a.client == nil || a.client.Client == nil {
		return utils.FailResponse(c, utils.StatusInternalError, "ETCD客户端未初始化")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 获取成员列表
	resp, err := a.client.Client.MemberList(ctx)
	if err != nil {
		a.logger.Error("获取成员列表失败", zap.Error(err))
		return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("获取成员列表失败: %v", err))
	}

	// 转换成员数据
	members := make([]map[string]interface{}, 0, len(resp.Members))
	for _, m := range resp.Members {
		members = append(members, map[string]interface{}{
			"id":          fmt.Sprintf("%x", m.ID),
			"name":        m.Name,
			"peer_urls":   m.PeerURLs,
			"client_urls": m.ClientURLs,
			"is_learner":  m.IsLearner,
		})
	}

	return utils.SuccessResponse(c, map[string]interface{}{
		"members": members,
	})
}

// MemberAddRequest 表示添加成员请求
type MemberAddRequest struct {
	Name     string   `json:"name"`
	PeerURLs []string `json:"peer_urls"`
}

// AddMember 添加集群成员
func (a *API) AddMember(c *fiber.Ctx) error {
	if a.client == nil || a.client.Client == nil {
		return utils.FailResponse(c, utils.StatusInternalError, "ETCD客户端未初始化")
	}

	var req MemberAddRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.FailResponse(c, utils.StatusBadRequest, fmt.Sprintf("无效的请求体: %v", err))
	}

	// 验证请求
	if req.Name == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "成员名称不能为空")
	}
	if len(req.PeerURLs) == 0 {
		return utils.FailResponse(c, utils.StatusBadRequest, "至少需要提供一个PeerURL")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 添加成员
	resp, err := a.client.Client.MemberAdd(ctx, req.PeerURLs)
	if err != nil {
		a.logger.Error("添加成员失败", zap.Error(err))
		return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("添加成员失败: %v", err))
	}

	// 查找新添加的成员
	var newMember *etcdserverpb.Member
	for _, m := range resp.Members {
		for _, url := range m.PeerURLs {
			for _, reqURL := range req.PeerURLs {
				if url == reqURL {
					newMember = m
					break
				}
			}
			if newMember != nil {
				break
			}
		}
		if newMember != nil {
			break
		}
	}

	if newMember == nil {
		return utils.FailResponse(c, utils.StatusInternalError, "无法识别新添加的成员")
	}

	// 返回新成员信息
	return utils.SuccessResponse(c, map[string]interface{}{
		"member": map[string]interface{}{
			"id":          fmt.Sprintf("%x", newMember.ID),
			"name":        newMember.Name,
			"peer_urls":   newMember.PeerURLs,
			"client_urls": newMember.ClientURLs,
			"is_learner":  newMember.IsLearner,
		},
	})
}

// MemberUpdateRequest 表示更新成员请求
type MemberUpdateRequest struct {
	PeerURLs []string `json:"peer_urls"`
}

// UpdateMember 更新集群成员
func (a *API) UpdateMember(c *fiber.Ctx) error {
	if a.client == nil || a.client.Client == nil {
		return utils.FailResponse(c, utils.StatusInternalError, "ETCD客户端未初始化")
	}

	// 获取成员ID
	idStr := c.Params("id")
	id, err := strconv.ParseUint(idStr, 16, 64)
	if err != nil {
		return utils.FailResponse(c, utils.StatusBadRequest, fmt.Sprintf("无效的成员ID: %v", err))
	}

	var req MemberUpdateRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.FailResponse(c, utils.StatusBadRequest, fmt.Sprintf("无效的请求体: %v", err))
	}

	// 验证请求
	if len(req.PeerURLs) == 0 {
		return utils.FailResponse(c, utils.StatusBadRequest, "至少需要提供一个PeerURL")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 更新成员
	_, err = a.client.Client.MemberUpdate(ctx, id, req.PeerURLs)
	if err != nil {
		a.logger.Error("更新成员失败", zap.Error(err))
		return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("更新成员失败: %v", err))
	}

	// 获取更新后的成员信息
	resp, err := a.client.Client.MemberList(ctx)
	if err != nil {
		a.logger.Error("获取成员列表失败", zap.Error(err))
		return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("获取成员列表失败: %v", err))
	}

	// 查找更新的成员
	var updatedMember *etcdserverpb.Member
	for _, m := range resp.Members {
		if m.ID == id {
			updatedMember = m
			break
		}
	}

	if updatedMember == nil {
		return utils.FailResponse(c, utils.StatusNotFound, "找不到指定的成员")
	}

	// 返回更新后的成员信息
	return utils.SuccessResponse(c, map[string]interface{}{
		"member": map[string]interface{}{
			"id":          fmt.Sprintf("%x", updatedMember.ID),
			"name":        updatedMember.Name,
			"peer_urls":   updatedMember.PeerURLs,
			"client_urls": updatedMember.ClientURLs,
			"is_learner":  updatedMember.IsLearner,
		},
	})
}

// RemoveMember 删除集群成员
func (a *API) RemoveMember(c *fiber.Ctx) error {
	if a.client == nil || a.client.Client == nil {
		return utils.FailResponse(c, utils.StatusInternalError, "ETCD客户端未初始化")
	}

	// 获取成员ID
	idStr := c.Params("id")
	id, err := strconv.ParseUint(idStr, 16, 64)
	if err != nil {
		return utils.FailResponse(c, utils.StatusBadRequest, fmt.Sprintf("无效的成员ID: %v", err))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 删除成员
	_, err = a.client.Client.MemberRemove(ctx, id)
	if err != nil {
		a.logger.Error("删除成员失败", zap.Error(err))
		return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("删除成员失败: %v", err))
	}

	return utils.SuccessResponse(c, map[string]interface{}{
		"message": "成员已成功删除",
	})
}

// =================================
// 键值管理
// =================================

// GetValue 获取指定键的值
func (a *API) GetValue(c *fiber.Ctx) error {
	if a.client == nil || a.client.Client == nil {
		return utils.FailResponse(c, utils.StatusInternalError, "ETCD客户端未初始化")
	}

	// 获取键
	key := c.Params("key")
	if key == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "键不能为空")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 获取键值
	resp, err := a.client.Client.Get(ctx, key)
	if err != nil {
		a.logger.Error("获取键值失败", zap.Error(err))
		return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("获取键值失败: %v", err))
	}

	// 检查键是否存在
	if len(resp.Kvs) == 0 {
		return utils.FailResponse(c, utils.StatusNotFound, fmt.Sprintf("键不存在: %s", key))
	}

	// 返回键值信息
	kv := resp.Kvs[0]
	return utils.SuccessResponse(c, map[string]interface{}{
		"key":             string(kv.Key),
		"value":           string(kv.Value),
		"create_revision": kv.CreateRevision,
		"mod_revision":    kv.ModRevision,
		"version":         kv.Version,
	})
}

// GetValuesWithPrefix 获取具有指定前缀的所有键值
func (a *API) GetValuesWithPrefix(c *fiber.Ctx) error {
	if a.client == nil || a.client.Client == nil {
		return utils.FailResponse(c, utils.StatusInternalError, "ETCD客户端未初始化")
	}

	// 获取前缀
	prefix := c.Query("prefix")
	if prefix == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "前缀参数不能为空")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 获取前缀键值
	resp, err := a.client.Client.Get(ctx, prefix, clientv3.WithPrefix())
	if err != nil {
		a.logger.Error("获取前缀键值失败", zap.Error(err))
		return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("获取前缀键值失败: %v", err))
	}

	// 构造响应
	kvs := make([]map[string]interface{}, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		kvs = append(kvs, map[string]interface{}{
			"key":             string(kv.Key),
			"value":           string(kv.Value),
			"create_revision": kv.CreateRevision,
			"mod_revision":    kv.ModRevision,
			"version":         kv.Version,
		})
	}

	return utils.SuccessResponse(c, map[string]interface{}{
		"kvs":   kvs,
		"count": len(kvs),
	})
}

// PutValueRequest 表示放置键值请求
type PutValueRequest struct {
	Value string `json:"value"`
}

// PutValue 创建或更新键值
func (a *API) PutValue(c *fiber.Ctx) error {
	if a.client == nil || a.client.Client == nil {
		return utils.FailResponse(c, utils.StatusInternalError, "ETCD客户端未初始化")
	}

	// 获取键
	key := c.Params("key")
	if key == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "键不能为空")
	}

	var req PutValueRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.FailResponse(c, utils.StatusBadRequest, fmt.Sprintf("无效的请求体: %v", err))
	}

	// 获取可选参数
	leaseStr := c.Query("lease")
	prevKV := c.Query("prev_kv") == "true"

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var opts []clientv3.OpOption
	if leaseStr != "" {
		leaseID, err := strconv.ParseInt(leaseStr, 16, 64)
		if err != nil {
			return utils.FailResponse(c, utils.StatusBadRequest, fmt.Sprintf("无效的租约ID: %v", err))
		}
		opts = append(opts, clientv3.WithLease(clientv3.LeaseID(leaseID)))
	}
	if prevKV {
		opts = append(opts, clientv3.WithPrevKV())
	}

	// 放置键值
	_, err := a.client.Client.Put(ctx, key, req.Value, opts...)
	if err != nil {
		a.logger.Error("放置键值失败", zap.Error(err))
		return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("放置键值失败: %v", err))
	}

	// 获取更新后的键值信息
	getResp, err := a.client.Client.Get(ctx, key)
	if err != nil {
		a.logger.Error("获取更新后的键值失败", zap.Error(err))
		return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("获取更新后的键值失败: %v", err))
	}

	// 检查键是否存在
	if len(getResp.Kvs) == 0 {
		return utils.FailResponse(c, utils.StatusInternalError, "键值更新异常：找不到更新后的键")
	}

	// 返回键值信息
	kv := getResp.Kvs[0]
	return utils.SuccessResponse(c, map[string]interface{}{
		"key":             string(kv.Key),
		"value":           string(kv.Value),
		"create_revision": kv.CreateRevision,
		"mod_revision":    kv.ModRevision,
		"version":         kv.Version,
	})
}

// DeleteValue 删除指定键值
func (a *API) DeleteValue(c *fiber.Ctx) error {
	if a.client == nil || a.client.Client == nil {
		return utils.FailResponse(c, utils.StatusInternalError, "ETCD客户端未初始化")
	}

	// 获取键
	key := c.Params("key")
	if key == "" {
		return utils.FailResponse(c, utils.StatusBadRequest, "键不能为空")
	}

	// 检查是否按前缀删除
	isPrefix := c.Query("prefix") == "true"

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var opts []clientv3.OpOption
	if isPrefix {
		opts = append(opts, clientv3.WithPrefix())
	}

	// 删除键值
	resp, err := a.client.Client.Delete(ctx, key, opts...)
	if err != nil {
		a.logger.Error("删除键值失败", zap.Error(err))
		return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("删除键值失败: %v", err))
	}

	// 检查是否删除成功
	if resp.Deleted == 0 && !isPrefix {
		return utils.FailResponse(c, utils.StatusNotFound, fmt.Sprintf("键不存在: %s", key))
	}

	return utils.SuccessResponse(c, map[string]interface{}{
		"deleted": resp.Deleted,
	})
}

// =================================
// 租约管理
// =================================

// LeaseCreateRequest 表示创建租约请求
type LeaseCreateRequest struct {
	TTL int64 `json:"ttl"`
}

// CreateLease 创建租约
func (a *API) CreateLease(c *fiber.Ctx) error {
	if a.client == nil || a.client.Client == nil {
		return utils.FailResponse(c, utils.StatusInternalError, "ETCD客户端未初始化")
	}

	var req LeaseCreateRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.FailResponse(c, utils.StatusBadRequest, fmt.Sprintf("无效的请求体: %v", err))
	}

	// 验证TTL
	if req.TTL <= 0 {
		return utils.FailResponse(c, utils.StatusBadRequest, "TTL必须大于0")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 创建租约
	resp, err := a.client.Client.Grant(ctx, req.TTL)
	if err != nil {
		a.logger.Error("创建租约失败", zap.Error(err))
		return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("创建租约失败: %v", err))
	}

	return utils.SuccessResponse(c, map[string]interface{}{
		"id":  fmt.Sprintf("%x", resp.ID),
		"ttl": resp.TTL,
	})
}

// KeepAlive 续约指定租约
func (a *API) KeepAlive(c *fiber.Ctx) error {
	if a.client == nil || a.client.Client == nil {
		return utils.FailResponse(c, utils.StatusInternalError, "ETCD客户端未初始化")
	}

	// 获取租约ID
	idStr := c.Params("id")
	id, err := strconv.ParseInt(idStr, 16, 64)
	if err != nil {
		return utils.FailResponse(c, utils.StatusBadRequest, fmt.Sprintf("无效的租约ID: %v", err))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 续约
	resp, err := a.client.Client.KeepAliveOnce(ctx, clientv3.LeaseID(id))
	if err != nil {
		a.logger.Error("续约租约失败", zap.Error(err))
		return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("续约租约失败: %v", err))
	}

	return utils.SuccessResponse(c, map[string]interface{}{
		"id":  fmt.Sprintf("%x", resp.ID),
		"ttl": resp.TTL,
	})
}

// RevokeLease 撤销指定租约
func (a *API) RevokeLease(c *fiber.Ctx) error {
	if a.client == nil || a.client.Client == nil {
		return utils.FailResponse(c, utils.StatusInternalError, "ETCD客户端未初始化")
	}

	// 获取租约ID
	idStr := c.Params("id")
	id, err := strconv.ParseInt(idStr, 16, 64)
	if err != nil {
		return utils.FailResponse(c, utils.StatusBadRequest, fmt.Sprintf("无效的租约ID: %v", err))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 撤销租约
	_, err = a.client.Client.Revoke(ctx, clientv3.LeaseID(id))
	if err != nil {
		a.logger.Error("撤销租约失败", zap.Error(err))
		return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("撤销租约失败: %v", err))
	}

	return utils.SuccessResponse(c, map[string]interface{}{
		"message": "租约已撤销",
	})
}

// =================================
// 认证管理
// =================================

// EnableAuth 启用认证
func (a *API) EnableAuth(c *fiber.Ctx) error {
	if a.client == nil || a.client.Client == nil {
		return utils.FailResponse(c, utils.StatusInternalError, "ETCD客户端未初始化")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 启用认证
	_, err := a.client.Client.AuthEnable(ctx)
	if err != nil {
		a.logger.Error("启用认证失败", zap.Error(err))
		return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("启用认证失败: %v", err))
	}

	return utils.SuccessResponse(c, map[string]interface{}{
		"message": "认证已启用",
	})
}

// DisableAuth 禁用认证
func (a *API) DisableAuth(c *fiber.Ctx) error {
	if a.client == nil || a.client.Client == nil {
		return utils.FailResponse(c, utils.StatusInternalError, "ETCD客户端未初始化")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 禁用认证
	_, err := a.client.Client.AuthDisable(ctx)
	if err != nil {
		a.logger.Error("禁用认证失败", zap.Error(err))
		return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("禁用认证失败: %v", err))
	}

	return utils.SuccessResponse(c, map[string]interface{}{
		"message": "认证已禁用",
	})
}

// GetAllLeases 获取所有租约
func (a *API) GetAllLeases(c *fiber.Ctx) error {
	if a.client == nil || a.client.Client == nil {
		return utils.FailResponse(c, utils.StatusInternalError, "ETCD客户端未初始化")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 获取所有租约
	resp, err := a.client.Client.Lease.Leases(ctx)
	if err != nil {
		a.logger.Error("获取租约列表失败", zap.Error(err))
		return utils.FailResponse(c, utils.StatusInternalError, fmt.Sprintf("获取租约列表失败: %v", err))
	}

	// 构造响应
	leases := make([]map[string]interface{}, 0, len(resp.Leases))
	for _, lease := range resp.Leases {
		// 获取租约详细信息，包括TTL
		ttlResp, err := a.client.Client.Lease.TimeToLive(ctx, clientv3.LeaseID(lease.ID), clientv3.WithAttachedKeys())
		if err != nil {
			a.logger.Warn("获取租约详情失败", zap.String("id", fmt.Sprintf("%x", lease.ID)), zap.Error(err))
			continue
		}

		leaseInfo := map[string]interface{}{
			"id":          fmt.Sprintf("%x", lease.ID),
			"ttl":         ttlResp.TTL,
			"granted_ttl": ttlResp.GrantedTTL,
		}

		// 添加关联的键
		if len(ttlResp.Keys) > 0 {
			keys := make([]string, 0, len(ttlResp.Keys))
			for _, key := range ttlResp.Keys {
				keys = append(keys, string(key))
			}
			leaseInfo["keys"] = keys
		}

		leases = append(leases, leaseInfo)
	}

	return utils.SuccessResponse(c, map[string]interface{}{
		"leases": leases,
		"count":  len(leases),
	})
}

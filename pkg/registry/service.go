package registry

import (
	"encoding/json"
	"time"
)

// 服务状态常量定义
const (
	StatusUp          = "up"          // 服务正常运行
	StatusDown        = "down"        // 服务已下线（逻辑删除，保留记录）
	StatusStarting    = "starting"    // 服务启动中
	StatusStopping    = "stopping"    // 服务停止中
	StatusMaintenance = "maintenance" // 维护状态
	StatusUnhealthy   = "unhealthy"   // 不健康状态
)

// IsOnlineStatus 检查状态是否为在线状态
func IsOnlineStatus(status string) bool {
	switch status {
	case StatusUp:
		return true
	default:
		return false
	}
}

// IsOfflineStatus 检查状态是否为下线状态
func IsOfflineStatus(status string) bool {
	return status == StatusDown
}

// Environment 环境类型枚举
type Environment string

// 环境常量定义
const (
	EnvAll  Environment = "all"  // 适用于所有环境
	EnvDev  Environment = "dev"  // 开发环境
	EnvTest Environment = "test" // 测试环境
	EnvProd Environment = "prod" // 生产环境
)

// String 返回环境的字符串表示
func (e Environment) String() string {
	return string(e)
}

// IsValid 检查环境值是否有效
func (e Environment) IsValid() bool {
	switch e {
	case EnvAll, EnvDev, EnvTest, EnvProd:
		return true
	default:
		return false
	}
}

// GetValidEnvironments 获取所有有效的环境值
func GetValidEnvironments() []Environment {
	return []Environment{EnvAll, EnvDev, EnvTest, EnvProd}
}

// ParseEnvironment 从字符串解析环境类型
func ParseEnvironment(s string) Environment {
	env := Environment(s)
	if env.IsValid() {
		return env
	}
	// 如果不是有效的环境值，返回all作为默认值
	return EnvAll
}

// ServiceInstance 表示一个服务实例
type ServiceInstance struct {
	// 基本信息
	ID       string      `json:"id"`       // 服务实例唯一ID
	Name     string      `json:"name"`     // 服务名称
	Address  string      `json:"address"`  // 服务地址 (host:port)
	Protocol string      `json:"protocol"` // 协议类型 (http, grpc, tcp等)
	Env      Environment `json:"env"`      // 环境标识

	// 时间信息
	RegisterTime  time.Time `json:"register_time"`   // 注册时间
	StartTime     time.Time `json:"start_time"`      // 服务启动时间
	OfflineTime   time.Time `json:"offline_time"`    // 下线时间
	LastRenewTime time.Time `json:"last_renew_time"` // 最后续约时间

	// 元数据
	Metadata map[string]string `json:"metadata,omitempty"` // 服务元数据

	// 权重和状态
	Weight int    `json:"weight"` // 负载均衡权重
	Status string `json:"status"` // 服务状态 (up, down, starting, stopping, maintenance, unhealthy)
}

// ToJSON 将服务实例转换为JSON字符串
func (s *ServiceInstance) ToJSON() (string, error) {
	data, err := json.Marshal(s)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// FromJSON 从JSON字符串创建服务实例
func FromJSON(data string) (*ServiceInstance, error) {
	var instance ServiceInstance
	err := json.Unmarshal([]byte(data), &instance)
	if err != nil {
		return nil, err
	}
	return &instance, nil
}

// GetUptime 获取服务运行时长
func (s *ServiceInstance) GetUptime() time.Duration {
	if s.StartTime.IsZero() {
		return 0
	}
	return time.Since(s.StartTime)
}

// GetRegisterDuration 获取注册时长
func (s *ServiceInstance) GetRegisterDuration() time.Duration {
	if s.RegisterTime.IsZero() {
		return 0
	}
	return time.Since(s.RegisterTime)
}

// GetOfflineDuration 获取下线时长
func (s *ServiceInstance) GetOfflineDuration() time.Duration {
	if s.OfflineTime.IsZero() || !IsOfflineStatus(s.Status) {
		return 0
	}
	return time.Since(s.OfflineTime)
}

// GetLastRenewDuration 获取距离最后续约的时长
func (s *ServiceInstance) GetLastRenewDuration() time.Duration {
	if s.LastRenewTime.IsZero() {
		// 如果没有续约记录，返回注册以来的时长
		return s.GetRegisterDuration()
	}
	return time.Since(s.LastRenewTime)
}

// IsExpired 检查服务是否过期（基于续约间隔）
func (s *ServiceInstance) IsExpired(renewInterval time.Duration) bool {
	return s.GetLastRenewDuration() > renewInterval
}

// IsHealthy 检查服务是否健康（基于状态）
func (s *ServiceInstance) IsHealthy() bool {
	return IsOnlineStatus(s.Status)
}

// IsOnline 检查服务是否在线
func (s *ServiceInstance) IsOnline() bool {
	return IsOnlineStatus(s.Status)
}

// IsOffline 检查服务是否已下线
func (s *ServiceInstance) IsOffline() bool {
	return IsOfflineStatus(s.Status)
}

// Copy 创建服务实例的副本
func (s *ServiceInstance) Copy() *ServiceInstance {
	metadata := make(map[string]string)
	for k, v := range s.Metadata {
		metadata[k] = v
	}

	return &ServiceInstance{
		ID:            s.ID,
		Name:          s.Name,
		Address:       s.Address,
		Protocol:      s.Protocol,
		Env:           s.Env,
		RegisterTime:  s.RegisterTime,
		StartTime:     s.StartTime,
		OfflineTime:   s.OfflineTime,
		LastRenewTime: s.LastRenewTime,
		Metadata:      metadata,
		Weight:        s.Weight,
		Status:        s.Status,
	}
}

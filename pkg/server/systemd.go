package server

import (
	"context"
	"time"
)

// SystemdService systemd服务管理接口
type SystemdServiceManager interface {
	// 服务生命周期管理
	StartService(ctx context.Context, serverID, serviceName string) (*ServiceOperationResult, error)
	StopService(ctx context.Context, serverID, serviceName string) (*ServiceOperationResult, error)
	RestartService(ctx context.Context, serverID, serviceName string) (*ServiceOperationResult, error)
	ReloadService(ctx context.Context, serverID, serviceName string) (*ServiceOperationResult, error)

	// 服务状态查询
	GetService(ctx context.Context, serverID, serviceName string) (*SystemdService, error)
	ListServices(ctx context.Context, serverID string, req *ServiceListRequest) ([]*SystemdService, int, error)
	GetServiceStatus(ctx context.Context, serverID, serviceName string) (*ServiceOperationResult, error)

	// 服务配置管理
	CreateService(ctx context.Context, serverID string, req *ServiceCreateRequest) (*SystemdService, error)
	UpdateService(ctx context.Context, serverID, serviceName string, req *ServiceUpdateRequest) (*SystemdService, error)
	DeleteService(ctx context.Context, serverID, serviceName string) (*ServiceOperationResult, error)

	// 服务启用/禁用管理
	EnableService(ctx context.Context, serverID, serviceName string) (*ServiceOperationResult, error)
	DisableService(ctx context.Context, serverID, serviceName string) (*ServiceOperationResult, error)

	// 服务日志管理
	GetServiceLogs(ctx context.Context, req *ServiceLogRequest) (*ServiceLogResult, error)

	// 服务文件管理
	GetServiceFileContent(ctx context.Context, serverID, serviceName string) (*ServiceFileResult, error)

	// 系统管理
	ReloadSystemd(ctx context.Context, serverID string) (*ServiceOperationResult, error)
	DaemonReload(ctx context.Context, serverID string) (*ServiceOperationResult, error)
}

// ServiceState systemd服务状态
type ServiceState string

const (
	ServiceStateActive       ServiceState = "active"       // 活动状态
	ServiceStateInactive     ServiceState = "inactive"     // 非活动状态
	ServiceStateFailed       ServiceState = "failed"       // 失败状态
	ServiceStateActivating   ServiceState = "activating"   // 启动中
	ServiceStateDeactivating ServiceState = "deactivating" // 停止中
)

// ServiceType systemd服务类型
type ServiceType string

const (
	ServiceTypeSimple  ServiceType = "simple"  // 简单服务
	ServiceTypeForking ServiceType = "forking" // 分叉服务
	ServiceTypeOneshot ServiceType = "oneshot" // 一次性服务
	ServiceTypeNotify  ServiceType = "notify"  // 通知服务
	ServiceTypeDbus    ServiceType = "dbus"    // D-Bus服务
)

// SystemdService systemd服务信息
type SystemdService struct {
	Name        string            `json:"name"`        // 服务名称
	Status      ServiceState      `json:"status"`      // 服务状态
	Enabled     bool              `json:"enabled"`     // 是否开机启动
	Description string            `json:"description"` // 服务描述
	Type        ServiceType       `json:"type"`        // 服务类型
	ExecStart   string            `json:"execStart"`   // 启动命令
	ExecReload  string            `json:"execReload"`  // 重载命令
	ExecStop    string            `json:"execStop"`    // 停止命令
	WorkingDir  string            `json:"workingDir"`  // 工作目录
	User        string            `json:"user"`        // 运行用户
	Group       string            `json:"group"`       // 运行组
	Environment map[string]string `json:"environment"` // 环境变量
	PIDFile     string            `json:"pidFile"`     // PID文件路径
	Restart     string            `json:"restart"`     // 重启策略
	CreatedAt   time.Time         `json:"createdAt"`   // 创建时间
	UpdatedAt   time.Time         `json:"updatedAt"`   // 更新时间
}

// ServiceCreateRequest 创建服务请求
type ServiceCreateRequest struct {
	Name        string            `json:"name" validate:"required"`      // 服务名称
	Description string            `json:"description"`                   // 服务描述
	Type        ServiceType       `json:"type"`                          // 服务类型，默认simple
	ExecStart   string            `json:"execStart" validate:"required"` // 启动命令
	ExecReload  string            `json:"execReload"`                    // 重载命令
	ExecStop    string            `json:"execStop"`                      // 停止命令
	WorkingDir  string            `json:"workingDir"`                    // 工作目录
	User        string            `json:"user"`                          // 运行用户
	Group       string            `json:"group"`                         // 运行组
	Environment map[string]string `json:"environment"`                   // 环境变量
	PIDFile     string            `json:"pidFile"`                       // PID文件路径
	Restart     string            `json:"restart"`                       // 重启策略
	Enabled     bool              `json:"enabled"`                       // 是否开机启动
}

// ServiceUpdateRequest 更新服务请求
type ServiceUpdateRequest struct {
	Description *string            `json:"description,omitempty"` // 服务描述
	Type        *ServiceType       `json:"type,omitempty"`        // 服务类型
	ExecStart   *string            `json:"execStart,omitempty"`   // 启动命令
	ExecReload  *string            `json:"execReload,omitempty"`  // 重载命令
	ExecStop    *string            `json:"execStop,omitempty"`    // 停止命令
	WorkingDir  *string            `json:"workingDir,omitempty"`  // 工作目录
	User        *string            `json:"user,omitempty"`        // 运行用户
	Group       *string            `json:"group,omitempty"`       // 运行组
	Environment *map[string]string `json:"environment,omitempty"` // 环境变量
	PIDFile     *string            `json:"pidFile,omitempty"`     // PID文件路径
	Restart     *string            `json:"restart,omitempty"`     // 重启策略
	Enabled     *bool              `json:"enabled,omitempty"`     // 是否开机启动
}

// ServiceListRequest 服务列表查询请求
type ServiceListRequest struct {
	Status   ServiceState `json:"status"`   // 状态过滤
	Enabled  *bool        `json:"enabled"`  // 启用状态过滤
	Pattern  string       `json:"pattern"`  // 名称模式匹配
	UserOnly bool         `json:"userOnly"` // 仅显示用户创建的服务
	Limit    int          `json:"limit"`    // 分页大小
	Offset   int          `json:"offset"`   // 分页偏移
}

// ServiceOperationRequest 服务操作请求
type ServiceOperationRequest struct {
	ServerID string `json:"serverId" validate:"required"` // 服务器ID
	Name     string `json:"name" validate:"required"`     // 服务名称
}

// ServiceOperationResult 服务操作结果
type ServiceOperationResult struct {
	Success  bool   `json:"success"`          // 是否成功
	Message  string `json:"message"`          // 结果消息
	Error    string `json:"error,omitempty"`  // 错误信息
	Output   string `json:"output,omitempty"` // 命令输出
	ExitCode int    `json:"exitCode"`         // 退出码
}

// ServiceLogRequest 服务日志查询请求
type ServiceLogRequest struct {
	ServerID string `json:"serverId" validate:"required"` // 服务器ID
	Name     string `json:"name" validate:"required"`     // 服务名称
	Lines    int    `json:"lines"`                        // 获取行数，默认100
	Follow   bool   `json:"follow"`                       // 是否跟踪日志
}

// ServiceLogResult 服务日志结果
type ServiceLogResult struct {
	Logs  []string `json:"logs"`            // 日志内容
	Error string   `json:"error,omitempty"` // 错误信息
}

// ServiceFileResult 服务文件内容结果
type ServiceFileResult struct {
	ServiceName string `json:"serviceName"`     // 服务名称
	FilePath    string `json:"filePath"`        // 文件路径
	Content     string `json:"content"`         // 文件内容
	Exists      bool   `json:"exists"`          // 文件是否存在
	Error       string `json:"error,omitempty"` // 错误信息
}

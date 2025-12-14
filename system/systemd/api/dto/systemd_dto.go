package dto

// ------------------- Request DTOs -------------------

// CreateServiceReq 创建 service 请求
type CreateServiceReq struct {
	Name    string `json:"name" validate:"required,max=200" comment:"服务名称（需以 .service 结尾）"`
	Content string `json:"content" validate:"required" comment:"unit 文件内容"`
}

// UpdateServiceReq 更新 service 请求
type UpdateServiceReq struct {
	Content string `json:"content" validate:"required" comment:"unit 文件内容"`
}

// QueryServiceReq 查询 service 列表请求
type QueryServiceReq struct {
	Keyword       string `query:"keyword" comment:"关键字（按名称/描述搜索）"`
	IncludeStatus bool   `query:"includeStatus" comment:"是否包含运行状态信息"`
	PageNum       int    `query:"pageNum" validate:"min=1" comment:"页码"`
	Size          int    `query:"size" validate:"min=1,max=100" comment:"每页数量"`
}

// ------------------- Response DTOs -------------------

// ServiceInfoDTO 服务信息
type ServiceInfoDTO struct {
	Name        string `json:"name" comment:"服务名称"`
	Content     string `json:"content,omitempty" comment:"unit 文件内容"`
	Description string `json:"description,omitempty" comment:"服务描述"`
	ModTime     string `json:"modTime,omitempty" comment:"最后修改时间"`
}

// ServiceListItemDTO 服务列表项
type ServiceListItemDTO struct {
	Name        string `json:"name" comment:"服务名称"`
	Description string `json:"description,omitempty" comment:"服务描述"`
	ModTime     string `json:"modTime,omitempty" comment:"最后修改时间"`
	// 以下字段在 includeStatus=true 时填充
	ActiveState   string `json:"activeState,omitempty" comment:"活动状态（active/inactive/failed 等）"`
	SubState      string `json:"subState,omitempty" comment:"子状态（running/dead/exited 等）"`
	UnitFileState string `json:"unitFileState,omitempty" comment:"unit 文件状态（enabled/disabled 等）"`
}

// ServiceStatusDTO 服务状态
type ServiceStatusDTO struct {
	Name            string `json:"name" comment:"服务名称"`
	Description     string `json:"description,omitempty" comment:"服务描述"`
	LoadState       string `json:"loadState" comment:"加载状态"`
	ActiveState     string `json:"activeState" comment:"活动状态"`
	SubState        string `json:"subState" comment:"子状态"`
	UnitFileState   string `json:"unitFileState" comment:"unit 文件状态"`
	MainPID         int    `json:"mainPID,omitempty" comment:"主进程 PID"`
	ExecMainStartAt string `json:"execMainStartAt,omitempty" comment:"主进程启动时间"`
	MemoryCurrent   uint64 `json:"memoryCurrent,omitempty" comment:"当前内存使用（字节）"`
	Result          string `json:"result,omitempty" comment:"最后执行结果"`
}

// ServiceLogsDTO 服务日志
type ServiceLogsDTO struct {
	Name  string   `json:"name" comment:"服务名称"`
	Lines []string `json:"lines" comment:"日志行"`
}

// LogsReq 日志查询请求
type LogsReq struct {
	Lines int    `query:"n" validate:"omitempty,min=1,max=10000" comment:"返回行数（默认200）"`
	Since string `query:"since" comment:"起始时间（如 2024-01-01 或 1h ago）"`
	Until string `query:"until" comment:"结束时间"`
}

// ------------------- Unit 生成相关 DTO -------------------

// ServiceUnitParamsDTO systemd service unit 参数（结构化 + 扩展行）
type ServiceUnitParamsDTO struct {
	// [Unit] 段
	Description   string   `json:"description" comment:"服务描述"`
	Documentation string   `json:"documentation,omitempty" comment:"文档链接"`
	After         []string `json:"after,omitempty" comment:"在哪些 unit 之后启动（如 network.target）"`
	Wants         []string `json:"wants,omitempty" comment:"弱依赖的 unit"`
	Requires      []string `json:"requires,omitempty" comment:"强依赖的 unit"`

	// [Service] 段
	Type             string   `json:"type,omitempty" comment:"服务类型（simple/forking/oneshot/notify/idle），默认 simple"`
	ExecStart        string   `json:"execStart" validate:"required" comment:"启动命令（必填）"`
	ExecStartPre     []string `json:"execStartPre,omitempty" comment:"启动前命令"`
	ExecStartPost    []string `json:"execStartPost,omitempty" comment:"启动后命令"`
	ExecStop         string   `json:"execStop,omitempty" comment:"停止命令"`
	ExecReload       string   `json:"execReload,omitempty" comment:"重载命令"`
	WorkingDirectory string   `json:"workingDirectory,omitempty" comment:"工作目录"`
	User             string   `json:"user,omitempty" comment:"运行用户"`
	Group            string   `json:"group,omitempty" comment:"运行用户组"`
	Environment      []string `json:"environment,omitempty" comment:"环境变量（KEY=VALUE 格式）"`
	EnvironmentFile  string   `json:"environmentFile,omitempty" comment:"环境变量文件路径"`
	Restart          string   `json:"restart,omitempty" comment:"重启策略（no/always/on-success/on-failure/on-abnormal/on-abort/on-watchdog），默认 always"`
	RestartSec       int      `json:"restartSec,omitempty" comment:"重启间隔秒数"`
	TimeoutStartSec  int      `json:"timeoutStartSec,omitempty" comment:"启动超时秒数"`
	TimeoutStopSec   int      `json:"timeoutStopSec,omitempty" comment:"停止超时秒数"`
	LimitNOFILE      int      `json:"limitNOFILE,omitempty" comment:"最大文件描述符数"`
	LimitNPROC       int      `json:"limitNPROC,omitempty" comment:"最大进程数"`

	// [Install] 段
	WantedBy   []string `json:"wantedBy,omitempty" comment:"被哪些 target 依赖（如 multi-user.target），默认 multi-user.target"`
	RequiredBy []string `json:"requiredBy,omitempty" comment:"被哪些 target 强依赖"`
	Alias      []string `json:"alias,omitempty" comment:"别名"`

	// 扩展行（允许补充任意 key=value）
	ExtraUnitLines    []string `json:"extraUnitLines,omitempty" comment:"[Unit] 段额外行（key=value 格式）"`
	ExtraServiceLines []string `json:"extraServiceLines,omitempty" comment:"[Service] 段额外行（key=value 格式）"`
	ExtraInstallLines []string `json:"extraInstallLines,omitempty" comment:"[Install] 段额外行（key=value 格式）"`
}

// GenerateServiceReq 生成 service unit 内容请求（仅预览，不落盘）
type GenerateServiceReq struct {
	Params ServiceUnitParamsDTO `json:"params" validate:"required" comment:"unit 参数"`
}

// GenerateServiceResp 生成 service unit 内容响应
type GenerateServiceResp struct {
	Content string `json:"content" comment:"生成的 unit 文件内容"`
}

// CreateServiceFromParamsReq 按参数创建 service 请求
type CreateServiceFromParamsReq struct {
	Name   string               `json:"name" validate:"required,max=200" comment:"服务名称（需以 .service 结尾）"`
	Params ServiceUnitParamsDTO `json:"params" validate:"required" comment:"unit 参数"`
}

// UpdateServiceFromParamsReq 按参数更新 service 请求
type UpdateServiceFromParamsReq struct {
	Params ServiceUnitParamsDTO `json:"params" validate:"required" comment:"unit 参数"`
}


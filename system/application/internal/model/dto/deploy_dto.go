package dto

// DeploySpec 部署规格（对应 factory.yaml 的概念）
type DeploySpec struct {
	// 后端配置
	Backend *BackendSpec `json:"backend,omitempty"`
	// 前端配置
	Frontend *FrontendSpec `json:"frontend,omitempty"`
	// 通用配置
	Domain      string `json:"domain,omitempty"`
	SSL         bool   `json:"ssl,omitempty"`
	SSLKeyPath  string `json:"sslKeyPath,omitempty"`
	SSLCertPath string `json:"sslCertPath,omitempty"`
}

// BackendSpec 后端部署规格
type BackendSpec struct {
	Port         int      `json:"port"`
	StartCommand string   `json:"startCommand"`
	HealthURL    string   `json:"healthUrl,omitempty"`
	EnvVars      []string `json:"envVars,omitempty"`
	WorkingDir   string   `json:"workingDir,omitempty"`
}

// FrontendSpec 前端部署规格
type FrontendSpec struct {
	RootPath   string `json:"rootPath"`
	IndexFile  string `json:"indexFile,omitempty"`
	TryFiles   string `json:"tryFiles,omitempty"`
	CacheRules string `json:"cacheRules,omitempty"`
}

// DeployRequest 部署请求
type DeployRequest struct {
	ApplicationID       int64       `json:"applicationId" validate:"required"`
	Version             string      `json:"version" validate:"required"`
	BackendArtifactID   int64       `json:"backendArtifactId"`
	FrontendArtifactID  int64       `json:"frontendArtifactId"`
	Spec                *DeploySpec `json:"spec"`
	Operator            string      `json:"operator"`
}

// RollbackRequest 回滚请求
type RollbackRequest struct {
	ApplicationID   int64  `json:"applicationId" validate:"required"`
	TargetReleaseID int64  `json:"targetReleaseId" validate:"required"`
	Operator        string `json:"operator"`
}

// DeploymentInfo 部署信息（返回）
type DeploymentInfo struct {
	ID            int64  `json:"id"`
	ApplicationID int64  `json:"applicationId"`
	ReleaseID     int64  `json:"releaseId"`
	Version       string `json:"version"`
	Action        string `json:"action"`
	Status        string `json:"status"`
	StartedAt     string `json:"startedAt,omitempty"`
	FinishedAt    string `json:"finishedAt,omitempty"`
	Logs          []string `json:"logs,omitempty"`
	ErrorMessage  string `json:"errorMessage,omitempty"`
	Operator      string `json:"operator"`
}


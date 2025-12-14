package model

import (
	"xiaozhizhang/pkg/core/model/common"
)

// DeployTarget 部署目标模型
type DeployTarget struct {
	common.Model
	Name        string           `gorm:"size:100;not null;index" json:"name" comment:"部署目标名称"`
	Domain      string           `gorm:"size:200;not null;index" json:"domain" comment:"绑定域名或通配符模式（如 b.a.com、*.a.com）"`
	Type        DeployTargetType `gorm:"size:50;not null;index" json:"type" comment:"部署类型(local/ssh/aliyun_cas)"`
	Config      string           `gorm:"type:json;not null" json:"config" comment:"部署配置（JSON，包含路径/主机/凭证等，敏感字段加密）"`
	Status      int              `gorm:"default:1;not null;index" json:"status" comment:"状态：1=启用，0=禁用"`
	Description string           `gorm:"size:500" json:"description" comment:"部署目标描述"`
}

// TableName 指定表名
func (DeployTarget) TableName() string {
	return "ssl_deploy_targets"
}

// LocalDeployConfig 本机文件部署配置
type LocalDeployConfig struct {
	BasePath      string `json:"base_path" comment:"证书存放基础路径"`
	FullchainName string `json:"fullchain_name" comment:"fullchain.pem 文件名"`
	PrivkeyName   string `json:"privkey_name" comment:"privkey.pem 文件名"`
	FileMode      string `json:"file_mode" comment:"文件权限（如 0600）"`
	ReloadCommand string `json:"reload_command" comment:"部署后执行的重载命令（可选，如 nginx -s reload）"`
}

// SSHDeployConfig SSH 远端部署配置
type SSHDeployConfig struct {
	Host          string `json:"host" comment:"SSH 主机地址"`
	Port          int    `json:"port" comment:"SSH 端口"`
	Username      string `json:"username" comment:"SSH 用户名"`
	AuthMethod    string `json:"auth_method" comment:"认证方式：password/privatekey"`
	Password      string `json:"password" comment:"密码（加密存储）"`
	PrivateKey    string `json:"private_key" comment:"SSH 私钥（加密存储）"`
	RemotePath    string `json:"remote_path" comment:"远程证书存放路径"`
	FullchainName string `json:"fullchain_name" comment:"fullchain.pem 文件名"`
	PrivkeyName   string `json:"privkey_name" comment:"privkey.pem 文件名"`
	FileMode      string `json:"file_mode" comment:"文件权限（如 0600）"`
	ReloadCommand string `json:"reload_command" comment:"部署后执行的重载命令（可选）"`
}

// AliyunCASDeployConfig 阿里云证书服务部署配置
type AliyunCASDeployConfig struct {
	AccessKeyID     string `json:"access_key_id" comment:"阿里云 AccessKeyID"`
	AccessKeySecret string `json:"access_key_secret" comment:"阿里云 AccessKeySecret（加密存储）"`
	Region          string `json:"region" comment:"地域（如 cn-hangzhou）"`
	CertName        string `json:"cert_name" comment:"上传到 CAS 的证书名称"`
	AutoDeploy      bool   `json:"auto_deploy" comment:"是否自动部署到云产品（根据域名自动查询CDN/SLB/DCDN等服务并部署）"`
}

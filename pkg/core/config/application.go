package config

// ApplicationConfig Application 组件配置
type ApplicationConfig struct {
	// StorageMode 存储模式：local 或 oss
	StorageMode string `yaml:"storageMode"`
	// LocalArtifactDir 本地存储模式下的产物存储目录
	LocalArtifactDir string `yaml:"localArtifactDir"`
	// OSSPrefix OSS 存储模式下的 key 前缀
	OSSPrefix string `yaml:"ossPrefix"`
	// ReleaseDir 应用发布目录（解压后的文件存放位置）
	ReleaseDir string `yaml:"releaseDir"`
	// KeepReleases 保留的版本数量
	KeepReleases int `yaml:"keepReleases"`
	// UploadMaxBytes 上传文件大小限制（字节）
	UploadMaxBytes int64 `yaml:"uploadMaxBytes"`
}

// DefaultApplicationConfig 返回默认配置
func DefaultApplicationConfig() ApplicationConfig {
	return ApplicationConfig{
		StorageMode:      "local",
		LocalArtifactDir: "/opt/apps/artifacts",
		OSSPrefix:        "application/artifacts",
		ReleaseDir:       "/opt/apps/releases",
		KeepReleases:     5,
		UploadMaxBytes:   536870912, // 512MB
	}
}


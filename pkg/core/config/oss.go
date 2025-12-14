package config

// OssConfig OSS配置结构体
type OssConfig struct {
	AccessKeyID     string `yaml:"access-key"`       // 访问密钥ID
	AccessKeySecret string `yaml:"access-secret"`    // 访问密钥Secret
	Bucket          string `yaml:"bucket-name"`      // 存储空间名称
	Domain          string `yaml:"domain"`           //绑定的自定义域名
	Region          string `yaml:"region,omitempty"` // 区域
}

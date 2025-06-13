package config

type SSLConfig struct {
	Email   string `yaml:"email" json:"email"` // Let's Encrypt账户邮箱
	CertDir string `yaml:"cert_dir" json:"cert_dir"`
}

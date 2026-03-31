package bootstrap

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// LocalBootstrap 本地引导配置（极其轻量，只管连接配置中心）
type LocalBootstrap struct {
	AppName string    `yaml:"app-name"`
	Env     string    `yaml:"env"`
	Host    string    `yaml:"host"`   // 绑定IP
	Port    int       `yaml:"port"`   // HTTP 端口
	Domain  string    `yaml:"domain"` // 域名
	Aio     SdkConfig `yaml:"sdk"`
}

// loadBootstrap 解析本地引导文件
func loadBootstrap(path string) (*LocalBootstrap, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取本地引导文件失败: %w", err)
	}

	var boot LocalBootstrap
	if err := yaml.Unmarshal(data, &boot); err != nil {
		return nil, fmt.Errorf("解析引导文件失败: %w", err)
	}

	if boot.Aio.RegistryAddr == "" || boot.Aio.ClientKey == "" {
		return nil, fmt.Errorf("引导文件缺失必要的 aio 连接信息")
	}

	return &boot, nil
}

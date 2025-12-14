package config

type LogConfig struct {
	Level        string `yaml:"level"`
	Sls          bool   `yaml:"sls"`
	Endpoint     string `yaml:"endpoint"`
	Project      string `yaml:"project"`
	Logstore     string `yaml:"logstore"`
	AccessKey    string `yaml:"access-key"`
	AccessSecret string `yaml:"access-secret"`
}

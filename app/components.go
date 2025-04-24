package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	configInternal "github.com/xsxdot/aio/pkg/config"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/xsxdot/aio/app/config"
	consts "github.com/xsxdot/aio/app/const"
	"github.com/xsxdot/aio/pkg/common"
	"github.com/xsxdot/aio/pkg/distributed/discovery"
)

type Component interface {
	Name() string
	Status() consts.ComponentStatus
	Init(config *config.BaseConfig, cfgBody []byte) error
	Start(ctx context.Context) error
	Restart(ctx context.Context) error
	Stop(ctx context.Context) error
	RegisterMetadata() (bool, int, map[string]string)
	DefaultConfig(baseConfig *config.BaseConfig) interface{} // 返回组件的默认配置，用于前端初始化展示
	GetClientConfig() (bool, *config.ClientConfig)           // 返回客户端配置：是否需要返回配置和配置内容
}

type ComponentEntity struct {
	Component
	Enable bool
	cfg    *config.ComponentConfig
	Type   ComponentType
}

func NewBaseComponentEntity(component Component, name string) *ComponentEntity {
	return &ComponentEntity{
		Component: component,
		cfg:       config.NewComponentType(name, config.ReadTypeFile),
		Type:      TypeBase,
		Enable:    true,
	}
}

func NewBaseComponentEntityWithNilConfig(component Component) *ComponentEntity {
	return &ComponentEntity{
		Component: component,
		cfg:       config.NewComponentType("", config.ReadTypeNil),
		Type:      TypeBase,
		Enable:    true,
	}
}

func NewMustComponentEntity(component Component, t config.ReadType) *ComponentEntity {
	return &ComponentEntity{
		Component: component,
		cfg:       config.NewComponentType(component.Name(), t),
		Type:      TypeMust,
		Enable:    true,
	}
}

func NewNormalComponentEntity(component Component, t config.ReadType) *ComponentEntity {
	return &ComponentEntity{
		Component: component,
		Type:      TypeNormal,
		cfg:       config.NewComponentType(component.Name(), t),
		Enable:    false,
	}
}

func NewComponentEntity(enable bool, component Component, readType config.ReadType, readName string, componentType ComponentType) *ComponentEntity {
	return &ComponentEntity{
		Component: component,
		Type:      componentType,
		cfg:       config.NewComponentType(readName, readType),
		Enable:    enable,
	}
}

type ComponentType string

const (
	TypeBase   ComponentType = "base"
	TypeMust   ComponentType = "must"
	TypeNormal ComponentType = "normal"
)

// ComponentManager 组件注册表
type ComponentManager struct {
	registerFunc []func() (*ComponentEntity, error)
	components   map[string]*ComponentEntity
	order        []*ComponentEntity
	mu           sync.RWMutex
	ctx          context.Context
	app          *App
	logger       *zap.Logger
	reinitConfig map[string][]byte
	enables      map[string]bool
}

// NewComponentRegistry 创建一个新的组件注册表
func NewComponentRegistry(app *App) *ComponentManager {
	return &ComponentManager{
		components:   make(map[string]*ComponentEntity),
		order:        make([]*ComponentEntity, 0),
		ctx:          context.Background(),
		app:          app,
		logger:       common.GetLogger().GetZapLogger("ComponentManager"),
		reinitConfig: make(map[string][]byte),
		enables:      make(map[string]bool),
	}
}

func (r *ComponentManager) WithReinitConfig(cfg map[string][]byte, enables map[string]bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.reinitConfig = cfg
	r.enables = enables
}

// Register 注册组件
func (r *ComponentManager) Register(f func() (*ComponentEntity, error)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.registerFunc = append(r.registerFunc, f)
}

// Get 获取组件
func (r *ComponentManager) Get(name string) *ComponentEntity {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.components[name]
}

// ListComponents 列出所有组件及其状态
func (r *ComponentManager) ListComponents() map[string]*ComponentInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]*ComponentInfo)
	for name, component := range r.components {
		result[name] = &ComponentInfo{
			ComponentType: component.Type,
			Enable:        component.Enable,
			Name:          component.Name(),
			Status:        componentStatusToString(component.Status()),
		}
	}

	return result
}

// StartAll 启动所有组件,这个会启动两次，第一次启动基础组件，例如etcd和配置中心，第二次启动其他组件。因为有判断逻辑，不会启动两次或者配置两次。找不到配置的默认为被禁用了
func (r *ComponentManager) StartAll(ctx context.Context) error {
	r.ctx = ctx
	for _, f := range r.registerFunc {
		componentEntity, err := f()
		if err != nil {
			return err
		}
		r.components[componentEntity.Name()] = componentEntity
		r.order = append(r.order, componentEntity)
		err = r.Start(context.Background(), componentEntity.Name())
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *ComponentManager) RegisterAllService() {
	for _, component := range r.order {
		if component.Status() != consts.StatusRunning {
			continue
		}
		needReg, port, meta := component.RegisterMetadata()
		if needReg {
			r.RegisterService(component.Name(), port, meta)
		}
	}
}

func (r *ComponentManager) RegisterService(name string, port int, meta map[string]string) {
	// 创建服务信息
	serviceInfo := discovery.ServiceInfo{
		ID:      r.app.BaseConfig.System.NodeId,
		Name:    name,
		Address: r.app.BaseConfig.Network.BindIP, // 使用统一的系统IP
		Port:    port,
		Metadata: map[string]string{
			"version":   "1.0.0",
			"nodeId":    r.app.BaseConfig.System.NodeId,
			"public_ip": r.app.BaseConfig.Network.PublicIp,
		},
	}
	for k, v := range meta {
		serviceInfo.Metadata[k] = v
	}
	if err := r.app.Discovery.Register(r.ctx, serviceInfo); err != nil {
		r.logger.Error("注册服务失败", zap.Error(err))
	} else {
		r.logger.Info("注册服务成功", zap.String("service", serviceInfo.Name), zap.String("id", serviceInfo.ID))
	}
}

func (r *ComponentManager) StopAll(ctx context.Context) error {
	for i := len(r.order) - 1; i >= 0; i-- {
		component := r.order[i]
		err := r.Stop(ctx, component.Name())
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *ComponentManager) Stop(ctx context.Context, name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	component := r.components[name]
	if component != nil {
		if component.Status() == consts.StatusRunning {
			reg, _, _ := component.RegisterMetadata()
			if reg {
				if err := r.app.Discovery.Deregister(ctx, component.Name()); err != nil {
					r.logger.Error("注销服务失败", zap.Error(err))
				} else {
					r.logger.Info("注销服务成功", zap.String("service", component.Name()))
				}
			}

			err := component.Stop(ctx)
			if err != nil {
				r.logger.Error("停止组件失败", zap.Error(err))
				return err
			}
		}
	}
	return nil
}

func (r *ComponentManager) GetConfigData(component *ComponentEntity) []byte {
	if cfg, ok := r.reinitConfig[component.Name()]; ok {
		_ = r.loadInitConfig(component, cfg)
	}

	var bytes []byte
	var err error
	switch component.cfg.ReadType {
	case config.ReadTypeFile:
		bytes, err = os.ReadFile(filepath.Join(r.app.configDirPath, component.cfg.Name))
	case config.ReadTypeCenter:
		json, err := r.app.ConfigService.ExportConfigAsJSON(context.Background(), component.Name())
		if err == nil {
			bytes = []byte(json)
		}
	default:
		bytes = []byte("")
	}
	if err != nil {
		defaultConfig := component.DefaultConfig(r.app.BaseConfig)
		bytes, err = json.Marshal(defaultConfig)
		if err != nil {
			return nil
		}
	}
	component.cfg.Body = bytes
	return bytes
}

func (r *ComponentManager) LoadEnable(ctx context.Context, entity *ComponentEntity) bool {
	if entity.Type != TypeNormal {
		entity.Enable = true
		return true
	}
	entity.Enable = false

	if enable, ok := r.enables[entity.Name()]; ok {
		entity.Enable = enable
		return enable
	}

	configItem, err := r.app.ConfigService.Get(ctx, entity.Name())
	if err != nil {
		return entity.Enable
	}

	entity.Enable = configItem.Metadata["enable"] == "true"
	return entity.Enable
}

func (r *ComponentManager) Start(ctx context.Context, name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	component := r.components[name]
	if component != nil {

		if !r.LoadEnable(ctx, component) {
			r.logger.Info("组件已禁用，跳过启动", zap.String("component", component.Name()))
			return nil
		}

		data := r.GetConfigData(component)
		if component.Status() == consts.StatusNotInitialized {
			err := component.Init(r.app.BaseConfig, data)
			if err != nil {
				r.logger.Error("初始化组件配置失败,退出", zap.Error(err))
				return err
			}
		}

		if component.Status() == consts.StatusInitialized {
			err := component.Start(ctx)
			if err != nil {
				r.logger.Error("组件启动失败,退出", zap.Error(err))
				return err
			}
			r.logger.Info("组件启动成功", zap.String("component", component.Name()))
		}
	}
	return nil
}

func (r *ComponentManager) Restart(ctx context.Context, name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	component := r.components[name]
	if component != nil {
		if component.Status() == consts.StatusRunning {
			err := component.Restart(ctx)
			if err != nil {
				return err
			}
		} else if component.Status() == consts.StatusInitialized || component.Status() == consts.StatusStopped {
			err := component.Start(ctx)
			if err != nil {
				return err
			}
		} else {
			return errors.New("组件状态不正确")
		}

	}
	return nil
}

// GetDefaultConfig 获取指定组件的默认配置
func (r *ComponentManager) GetDefaultConfig(name string) (interface{}, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	component := r.components[name]
	if component == nil {
		return nil, errors.New("组件不存在")
	}
	return component.DefaultConfig(r.app.BaseConfig), nil
}

// GetAllDefaultConfigs 获取所有组件的默认配置
func (r *ComponentManager) GetAllDefaultConfigs() map[string]*ConfigInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	configs := make(map[string]*ConfigInfo)
	for name, component := range r.components {
		if component != nil {
			configs[name] = &ConfigInfo{
				Name:   component.Name(),
				Data:   component.DefaultConfig(r.app.BaseConfig),
				Enable: component.Enable,
				Type:   component.Type,
			}
		}
	}
	return configs
}

func (r *ComponentManager) loadDefaultConfig(entity *ComponentEntity) error {
	if entity.Type == TypeNormal {
		entity.Enable = false
	}

	defaultConfig := entity.DefaultConfig(r.app.BaseConfig)
	bytes, err := json.Marshal(defaultConfig)
	entity.cfg.Body = bytes
	return err
}

func (r *ComponentManager) loadInitConfig(component *ComponentEntity, cfg []byte) error {
	switch component.cfg.ReadType {
	case config.ReadTypeFile:
		// 构建文件路径
		path := filepath.Join(r.app.configDirPath, component.cfg.Name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return fmt.Errorf("创建配置目录失败: %w", err)
		}
		if err := os.WriteFile(path, cfg, 0644); err != nil {
			return fmt.Errorf("写入配置文件失败: %w", err)
		}
	case config.ReadTypeCenter:
		c := new(configInternal.ConfigItem)
		err := json.Unmarshal(cfg, c)
		if err != nil {
			return fmt.Errorf("解析初始化保存的配置失败: %w", err)
		}
		err = r.app.ConfigService.Set(context.Background(), component.Name(), c.Value, c.Metadata)
		if err != nil {
			return fmt.Errorf("保存配置到配置中心失败: %w", err)
		}
		if component.Type == TypeNormal {
			enable, ok := c.Metadata["enable"]
			if ok {
				component.Enable = enable == "true"
			}
		}
	}
	return nil
}

func (r *ComponentManager) RegisterClientConfig() {
	for _, component := range r.order {
		if component.Status() != consts.StatusRunning {
			continue
		}
		ok, clientConfig := component.GetClientConfig()
		if ok && clientConfig != nil {
			// 将客户端配置注册到配置中心
			if r.app.ConfigService != nil {
				// 使用组件名称作为配置键，将客户端配置写入配置中心
				configKey := fmt.Sprintf("%s%s", consts.ClientConfigPrefix, component.Name())
				metadata := map[string]string{
					"type":        "client_config",
					"component":   component.Name(),
					"create_time": time.Now().Format(time.RFC3339),
				}

				// 将ClientConfig转换为配置服务需要的格式
				configValue := make(map[string]*configInternal.ConfigValue)

				// 添加用户名
				if clientConfig.Value.Username != "" {
					configValue["username"] = &configInternal.ConfigValue{
						Value: clientConfig.Value.Username,
						Type:  configInternal.ValueTypeString,
					}
				}

				// 添加密码（加密形式）
				if clientConfig.Value.Password != "" {
					configValue["password"] = &configInternal.ConfigValue{
						Value: clientConfig.Value.Password,
						Type:  configInternal.ValueTypeEncrypted,
					}
				}

				// 添加TLS相关配置
				configValue["enable_tls"] = &configInternal.ConfigValue{
					Value: fmt.Sprintf("%v", clientConfig.Value.EnableTls),
					Type:  configInternal.ValueTypeBool,
				}

				if clientConfig.Value.Cert != "" {
					configValue["cert"] = &configInternal.ConfigValue{
						Value: clientConfig.Value.Cert,
						Type:  configInternal.ValueTypeString,
					}
				}

				if clientConfig.Value.Key != "" {
					configValue["key"] = &configInternal.ConfigValue{
						Value: clientConfig.Value.Key,
						Type:  configInternal.ValueTypeString,
					}
				}

				if clientConfig.Value.TrustedCAFile != "" {
					configValue["trusted_ca_file"] = &configInternal.ConfigValue{
						Value: clientConfig.Value.TrustedCAFile,
						Type:  configInternal.ValueTypeString,
					}
				}

				// 添加版本和元数据
				configValue["_version"] = &configInternal.ConfigValue{
					Value: fmt.Sprintf("%d", clientConfig.Version),
					Type:  configInternal.ValueTypeInt,
				}

				for mk, mv := range clientConfig.Metadata {
					metadata[mk] = mv
				}

				// 注册到配置中心
				err := r.app.ConfigService.Set(r.ctx, configKey, configValue, metadata)
				if err != nil {
					r.logger.Error("注册客户端配置失败", zap.String("component", component.Name()), zap.Error(err))
				} else {
					r.logger.Info("注册客户端配置成功", zap.String("component", component.Name()))
				}
			}
		}
	}
}

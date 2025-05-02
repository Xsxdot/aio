package config

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/xsxdot/aio/app/config"
	consts "github.com/xsxdot/aio/app/const"
	"github.com/xsxdot/aio/pkg/common"

	"github.com/xsxdot/aio/internal/etcd"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"
)

// 增加一个加密前缀常量，用于标识已加密的值
const (
	EncryptedPrefix = "ENC:"
)

type Config struct {
	Salt   string `json:"salt"`
	Prefix string `json:"prefix"`
}

// HistoryItem 配置历史记录项
type HistoryItem struct {
	*ConfigItem
	ModRevision int64     `json:"mod_revision"` // etcd修订版本号
	CreateTime  time.Time `json:"create_time"`  // 创建时间
}

// Service 配置中心服务
type Service struct {
	client  *etcd.EtcdClient
	logger  *zap.Logger
	watches map[string][]chan *ConfigItem
	mu      sync.RWMutex
	config  *Config
	salt    []byte
	prefix  string
	status  consts.ComponentStatus
	ctx     context.Context
	cancel  context.CancelFunc
}

func (s *Service) RegisterMetadata() (bool, int, map[string]string) {
	return false, 0, nil
}

func (s *Service) Name() string {
	return consts.ComponentConfigService
}

func (s *Service) Status() consts.ComponentStatus {
	return s.status
}

func (s *Service) Init(config *config.BaseConfig, body []byte) error {
	s.config = s.genConfig(config)
	s.salt = s.getAESKey(s.config.Salt)
	s.prefix = s.config.Prefix
	s.status = consts.StatusInitialized
	s.ctx, s.cancel = context.WithCancel(context.Background())
	return nil
}

func (s *Service) Start(ctx context.Context) error {
	// 启动全局配置监听
	go s.watchConfigs()
	s.status = consts.StatusRunning
	return nil
}

func (s *Service) Restart(ctx context.Context) error {
	err := s.Stop(ctx)
	if err != nil {
		return err
	}
	return s.Start(ctx)
}

func (s *Service) Stop(ctx context.Context) error {
	s.status = consts.StatusStopped
	if s.cancel != nil {
		s.cancel()
	}
	return nil
}

// DefaultConfig 返回组件的默认配置
func (s *Service) DefaultConfig(baseConfig *config.BaseConfig) interface{} {
	return nil
}

func (s *Service) genConfig(baseConfig *config.BaseConfig) *Config {
	return &Config{
		Prefix: "/aio/config/",
		Salt:   baseConfig.System.ConfigSalt,
	}
}

// NewService 创建配置中心服务
func NewService(client *etcd.EtcdClient) *Service {
	ctx, cancel := context.WithCancel(context.Background())
	svc := &Service{
		client:  client,
		logger:  common.GetLogger().GetZapLogger(consts.ComponentConfigService),
		watches: make(map[string][]chan *ConfigItem),
		ctx:     ctx,
		cancel:  cancel,
	}

	return svc
}

// getAESKey 获取适合AES密钥长度的key
func (s *Service) getAESKey(salt string) []byte {
	key := []byte(salt)
	keyLen := len(key)

	// AES密钥必须是16、24或32字节
	switch {
	case keyLen > 32:
		// 如果密钥太长，截断
		return key[:32]
	case keyLen > 24:
		// 使用32字节密钥
		return paddedKey(key, 32)
	case keyLen > 16:
		// 使用24字节密钥
		return paddedKey(key, 24)
	default:
		// 使用16字节密钥
		return paddedKey(key, 16)
	}
}

// paddedKey 对密钥进行填充或截断，使其达到指定长度
func paddedKey(key []byte, size int) []byte {
	paddedKey := make([]byte, size)
	// 复制原始密钥
	copy(paddedKey, key)
	// 如果密钥太短，使用相同的密钥重复填充
	if len(key) < size {
		for i := len(key); i < size; i++ {
			paddedKey[i] = key[i%len(key)]
		}
	}
	return paddedKey
}

// encryptValue 使用salt加密字符串值
func (s *Service) encryptValue(value string) (string, error) {
	// 如果值已经有加密前缀，直接返回
	if strings.HasPrefix(value, EncryptedPrefix) {
		return value, nil
	}

	if len(s.salt) == 0 {
		return "", fmt.Errorf("加密失败: salt未设置")
	}

	// 使用标准化的AES密钥
	key := s.salt

	// 使用AES-GCM加密
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("创建加密器失败: %v", err)
	}

	// 生成随机nonce
	nonce := make([]byte, 12)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("生成nonce失败: %v", err)
	}

	// 使用GCM模式
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("初始化GCM失败: %v", err)
	}

	// 加密数据
	ciphertext := aesgcm.Seal(nil, nonce, []byte(value), nil)

	// 将nonce和密文合并并base64编码
	encrypted := append(nonce, ciphertext...)
	// 添加加密前缀
	return EncryptedPrefix + base64.StdEncoding.EncodeToString(encrypted), nil
}

// decryptValue 使用salt解密字符串值
func (s *Service) decryptValue(encryptedValue string) (string, error) {
	// 检查并移除加密前缀
	if strings.HasPrefix(encryptedValue, EncryptedPrefix) {
		encryptedValue = encryptedValue[len(EncryptedPrefix):]
	}

	if len(s.salt) == 0 {
		return "", fmt.Errorf("加密失败: salt未设置")
	}

	// base64解码
	data, err := base64.StdEncoding.DecodeString(encryptedValue)
	if err != nil {
		return "", fmt.Errorf("base64解码失败: %v", err)
	}

	// 确保数据长度足够
	if len(data) < 13 { // 12字节nonce + 至少1字节密文
		return "", fmt.Errorf("加密数据格式不正确")
	}

	// 分离nonce和密文
	nonce := data[:12]
	ciphertext := data[12:]

	// 使用标准化的AES密钥
	key := s.salt

	// 创建加密器
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("创建加密器失败: %v", err)
	}

	// 使用GCM模式
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("初始化GCM失败: %v", err)
	}

	// 解密数据
	plaintext, err := aesgcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("解密失败: %v", err)
	}

	return string(plaintext), nil
}

// Set 设置配置项
func (s *Service) Set(ctx context.Context, key string, value map[string]*ConfigValue, metadata map[string]string) error {
	for _, configValue := range value {
		if configValue.Type == ValueTypeEncrypted {
			encrypted, err := s.encryptValue(configValue.Value)
			if err != nil {
				return fmt.Errorf("加密值失败: %v", err)
			}
			configValue.Value = encrypted
		}
	}
	item := &ConfigItem{
		Key:       key,
		Value:     value,
		Version:   time.Now().UnixNano(),
		UpdatedAt: time.Now(),
		Metadata:  metadata,
	}

	data, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("序列化配置项失败: %v", err)
	}

	err = s.client.Put(ctx, s.prefix+key, string(data))
	if err != nil {
		return fmt.Errorf("保存配置项失败: %v", err)
	}

	return nil
}

// Get 获取配置项
func (s *Service) Get(ctx context.Context, key string) (*ConfigItem, error) {
	value, err := s.client.Get(ctx, s.prefix+key)
	if err != nil {
		return nil, fmt.Errorf("获取配置项失败: %v", err)
	}

	if value == "" {
		return nil, fmt.Errorf("配置项不存在: %s", key)
	}

	var item ConfigItem
	if err := json.Unmarshal([]byte(value), &item); err != nil {
		return nil, fmt.Errorf("解析配置项失败: %v", err)
	}

	return &item, nil
}

// GetHistory 获取配置项的历史版本
func (s *Service) GetHistory(ctx context.Context, key string, limit int64) ([]*HistoryItem, error) {
	if limit <= 0 {
		limit = 10
	}

	// 获取etcd原始客户端
	cli := s.client.Client

	// 获取当前修订版本
	resp, err := cli.Get(ctx, s.prefix+key,
		clientv3.WithLastKey()...,
	)
	if err != nil {
		return nil, fmt.Errorf("获取历史版本失败: %v", err)
	}

	// 获取最新的修订版本
	currentRev := resp.Header.Revision

	// 从最新版本开始获取历史版本
	items := make([]*HistoryItem, 0, limit)
	for rev := currentRev; rev > 0 && int64(len(items)) < limit; rev-- {
		getResp, err := cli.Get(ctx, s.prefix+key,
			clientv3.WithRev(rev),
		)
		if err != nil {
			s.logger.Error("获取历史版本失败",
				zap.String("key", key),
				zap.Int64("revision", rev),
				zap.Error(err))
			continue
		}
		if len(getResp.Kvs) == 0 {
			continue
		}

		kv := getResp.Kvs[0]
		var item ConfigItem
		if err := json.Unmarshal(kv.Value, &item); err != nil {
			s.logger.Error("解析历史配置项失败",
				zap.String("key", key),
				zap.Int64("revision", rev),
				zap.Error(err))
			continue
		}

		historyItem := &HistoryItem{
			ConfigItem:  &item,
			ModRevision: kv.ModRevision,
			CreateTime:  time.Unix(0, kv.CreateRevision),
		}

		// 检查是否已经添加过相同的版本
		isDuplicate := false
		for _, existingItem := range items {
			if existingItem.ModRevision == historyItem.ModRevision {
				isDuplicate = true
				break
			}
		}
		if !isDuplicate {
			items = append(items, historyItem)
		}
	}

	// 如果没有找到任何版本，返回错误
	if len(items) == 0 {
		return nil, fmt.Errorf("配置项不存在: %s", key)
	}

	return items, nil
}

// GetByRevision 获取指定版本的配置项
func (s *Service) GetByRevision(ctx context.Context, key string, revision int64) (*HistoryItem, error) {
	// 获取etcd原始客户端
	cli := s.client.Client

	// 获取指定版本的配置项
	resp, err := cli.Get(ctx, s.prefix+key, clientv3.WithRev(revision))
	if err != nil {
		return nil, fmt.Errorf("获取历史版本失败: %v", err)
	}

	if len(resp.Kvs) == 0 {
		return nil, fmt.Errorf("指定版本的配置项不存在: %s@%d", key, revision)
	}

	kv := resp.Kvs[0]
	var item ConfigItem
	if err := json.Unmarshal(kv.Value, &item); err != nil {
		return nil, fmt.Errorf("解析历史配置项失败: %v", err)
	}

	historyItem := &HistoryItem{
		ConfigItem:  &item,
		ModRevision: kv.ModRevision,
		CreateTime:  time.Unix(0, kv.CreateRevision),
	}

	return historyItem, nil
}

// Delete 删除配置项
func (s *Service) Delete(ctx context.Context, key string) error {
	err := s.client.Delete(ctx, s.prefix+key)
	if err != nil {
		return fmt.Errorf("删除配置项失败: %v", err)
	}

	return nil
}

// List 列出所有配置项
func (s *Service) List(ctx context.Context) ([]*ConfigItem, error) {
	values, err := s.client.GetWithPrefix(ctx, s.prefix)
	if err != nil {
		return nil, fmt.Errorf("获取配置项列表失败: %v", err)
	}

	items := make([]*ConfigItem, 0, len(values))
	for _, value := range values {
		var item ConfigItem
		if err := json.Unmarshal([]byte(value), &item); err != nil {
			s.logger.Error("解析配置项失败", zap.Error(err))
			continue
		}
		items = append(items, &item)
	}

	return items, nil
}

// Watch 监听配置项变更
func (s *Service) Watch(key string) chan *ConfigItem {
	ch := make(chan *ConfigItem, 1)
	s.mu.Lock()
	s.watches[key] = append(s.watches[key], ch)
	s.mu.Unlock()
	return ch
}

// Unwatch 取消监听配置项变更
func (s *Service) Unwatch(key string, ch chan *ConfigItem) {
	s.mu.Lock()
	defer s.mu.Unlock()

	channels := s.watches[key]
	for i, c := range channels {
		if c == ch {
			s.watches[key] = append(channels[:i], channels[i+1:]...)
			close(ch)
			break
		}
	}

	if len(s.watches[key]) == 0 {
		delete(s.watches, key)
	}
}

// watchConfigs 监听所有配置项的变更
func (s *Service) watchConfigs() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 创建一个goroutine来处理退出清理
	go func() {
		<-s.ctx.Done() // 服务的主上下文
		cancel()       // 取消监听
	}()

	for {
		// 如果服务已停止，终止监听循环
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		// 启动监听，增加重试机制
		watchChan := s.client.WatchWithPrefix(ctx, s.prefix)
		s.logger.Info("开始监听配置变更", zap.String("prefix", s.prefix))

		// 处理监听事件
		for resp := range watchChan {
			if err := resp.Err(); err != nil {
				s.logger.Error("监听配置出错", zap.Error(err))
				break // 退出内部循环，会触发重新连接
			}

			// 批量处理变更，减少锁竞争
			changes := make(map[string][]*ConfigItem)

			for _, event := range resp.Events {
				key := string(event.Kv.Key)[len(s.prefix):]

				var item *ConfigItem
				if event.Type != clientv3.EventTypeDelete {
					item = &ConfigItem{}
					if err := json.Unmarshal(event.Kv.Value, item); err != nil {
						s.logger.Error("解析配置项失败", zap.Error(err), zap.String("key", key))
						continue
					}
				}

				changes[key] = append(changes[key], item)
			}

			// 一次性获取锁，然后处理所有变更
			for key, items := range changes {
				s.mu.RLock()
				channels := s.watches[key]
				s.mu.RUnlock()

				if len(channels) == 0 {
					continue
				}

				// 对每个配置项的最后一次变更进行通知
				latestItem := items[len(items)-1]
				s.notifyWatchers(key, channels, latestItem)
			}
		}

		// 如果监听中断，等待一段时间后重试
		select {
		case <-s.ctx.Done():
			return
		case <-time.After(3 * time.Second):
			s.logger.Info("重新连接监听配置变更")
		}
	}
}

// notifyWatchers 通知所有监听者配置变更
func (s *Service) notifyWatchers(key string, channels []chan *ConfigItem, item *ConfigItem) {
	var wg sync.WaitGroup

	// 使用固定时间的Context来避免永久阻塞
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	for _, ch := range channels {
		wg.Add(1)
		go func(ch chan *ConfigItem) {
			defer wg.Done()

			select {
			case ch <- item:
				// 通知成功
			case <-ctx.Done():
				s.logger.Warn("配置项通知超时", zap.String("key", key))
			}
		}(ch)
	}

	// 等待所有通知完成
	wg.Wait()
}

// Close 关闭服务
func (s *Service) Close() {
	if s.cancel != nil {
		s.cancel()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// 关闭所有监听channel
	for _, channels := range s.watches {
		for _, ch := range channels {
			close(ch)
		}
	}
	s.watches = make(map[string][]chan *ConfigItem)
}

// GetCompositeConfig 获取组合配置，支持引用解析
func (s *Service) GetCompositeConfig(ctx context.Context, key string) (map[string]interface{}, error) {
	if s.client == nil {
		return nil, fmt.Errorf("etcd客户端未初始化或已关闭")
	}

	item, err := s.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("获取配置项失败: %v", err)
	}

	return s.parseCompositeConfig(ctx, item)
}

// parseCompositeConfig 解析组合配置
func (s *Service) parseCompositeConfig(ctx context.Context, item *ConfigItem) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	// 处理每个配置值
	for k, v := range item.Value {
		value, err := s.parseConfigValue(ctx, v)
		if err != nil {
			return nil, fmt.Errorf("解析配置值失败 [%s]: %v", k, err)
		}
		result[k] = value
	}

	return result, nil
}

// parseConfigValue 解析单个配置值
func (s *Service) parseConfigValue(ctx context.Context, value *ConfigValue) (interface{}, error) {
	switch value.Type {
	case ValueTypeString:
		return value.Value, nil
	case ValueTypeInt:
		var i int64
		if _, err := fmt.Sscanf(value.Value, "%d", &i); err != nil {
			return nil, fmt.Errorf("解析整数类型失败: %v", err)
		}
		return i, nil
	case ValueTypeFloat:
		var f float64
		if _, err := fmt.Sscanf(value.Value, "%f", &f); err != nil {
			return nil, fmt.Errorf("解析浮点数类型失败: %v", err)
		}
		return f, nil
	case ValueTypeBool:
		switch value.Value {
		case "true", "1", "yes", "on":
			return true, nil
		default:
			return false, nil
		}
	case ValueTypeObject:
		var obj map[string]interface{}
		if err := json.Unmarshal([]byte(value.Value), &obj); err != nil {
			return nil, fmt.Errorf("解析对象类型失败: %v", err)
		}
		return obj, nil
	case ValueTypeArray:
		var arr []interface{}
		if err := json.Unmarshal([]byte(value.Value), &arr); err != nil {
			return nil, fmt.Errorf("解析数组类型失败: %v", err)
		}
		return arr, nil
	case ValueTypeRef:
		// 使用点分隔符解析引用格式：如"log.base.dev.access-key"，其中最后一个部分是属性，前面部分是键
		refStr := value.Value
		lastDotIndex := strings.LastIndex(refStr, ".")

		var ref RefValue
		if lastDotIndex > 0 {
			ref.Key = refStr[:lastDotIndex]
			ref.Property = refStr[lastDotIndex+1:]
		} else {
			// 如果没有点号，整个字符串作为键，属性为空
			ref.Key = refStr
			ref.Property = ""
		}

		return s.resolveReference(ctx, ref)
	case ValueTypeEncrypted:
		// 解密加密值
		decrypted, err := s.decryptValue(value.Value)
		if err != nil {
			return nil, fmt.Errorf("解密配置值失败: %v", err)
		}
		return decrypted, nil
	default:
		return value.Value, nil
	}
}

// resolveReference 解析引用类型的配置值
func (s *Service) resolveReference(ctx context.Context, ref RefValue) (interface{}, error) {
	refItem, err := s.Get(ctx, ref.Key)
	if err != nil {
		return nil, fmt.Errorf("获取引用配置项失败 [%s]: %v", ref.Key, err)
	}

	// 解析引用的配置项
	config, err := s.parseCompositeConfig(ctx, refItem)
	if err != nil {
		return nil, fmt.Errorf("解析引用配置项失败 [%s]: %v", ref.Key, err)
	}

	// 如果Property为空，返回整个配置项
	if ref.Property == "" {
		return config, nil
	}

	// 否则查找指定的属性
	value, exists := config[ref.Property]
	if !exists {
		return nil, fmt.Errorf("引用的属性不存在 [%s.%s]", ref.Key, ref.Property)
	}

	return value, nil
}

// resolveReferenceWithEnvironment 解析引用类型的配置值，支持环境感知
// 注意：此方法已不再适用于当前的引用格式(点分隔符格式)，请使用resolveReference方法
// 保留此方法仅为了兼容性，不建议在新代码中使用
func (s *Service) resolveReferenceWithEnvironment(ctx context.Context, ref RefValue, envConfig *EnvironmentConfig) (interface{}, error) {
	// 不再支持环境感知的引用解析，直接使用普通引用解析
	return s.resolveReference(ctx, ref)
}

// parseConfigValueWithEnvironment 解析单个配置值，支持环境感知
func (s *Service) parseConfigValueWithEnvironment(ctx context.Context, value *ConfigValue, envConfig *EnvironmentConfig) (interface{}, error) {
	// 对于引用类型，不使用环境感知的解析方法，而是使用普通解析
	if value.Type == ValueTypeRef {
		// 使用点分隔符解析引用格式：如"log.base.dev.access-key"，其中最后一个部分是属性，前面部分是键
		refStr := value.Value
		lastDotIndex := strings.LastIndex(refStr, ".")

		var ref RefValue
		if lastDotIndex > 0 {
			ref.Key = refStr[:lastDotIndex]
			ref.Property = refStr[lastDotIndex+1:]
		} else {
			// 如果没有点号，整个字符串作为键，属性为空
			ref.Key = refStr
			ref.Property = ""
		}

		// 注意：这里直接使用resolveReference，不使用环境感知解析
		return s.resolveReference(ctx, ref)
	}

	// 对于加密类型，直接使用普通解析方法（会解密）
	if value.Type == ValueTypeEncrypted {
		decrypted, err := s.decryptValue(value.Value)
		if err != nil {
			return nil, fmt.Errorf("解密配置值失败: %v", err)
		}
		return decrypted, nil
	}

	// 其他类型使用普通解析方法
	return s.parseConfigValue(ctx, value)
}

// parseCompositeConfigWithEnvironment 解析组合配置，支持环境感知
func (s *Service) parseCompositeConfigWithEnvironment(ctx context.Context, item *ConfigItem, envConfig *EnvironmentConfig) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	// 处理每个配置值
	for k, v := range item.Value {
		var value interface{}
		var err error

		if envConfig != nil {
			value, err = s.parseConfigValueWithEnvironment(ctx, v, envConfig)
		} else {
			value, err = s.parseConfigValue(ctx, v)
		}

		if err != nil {
			return nil, fmt.Errorf("解析配置值失败 [%s]: %v", k, err)
		}

		result[k] = value
	}

	return result, nil
}

// GetCompositeConfigForEnvironment 获取特定环境的组合配置
func (s *Service) GetCompositeConfigForEnvironment(ctx context.Context, key string, envConfig *EnvironmentConfig) (map[string]interface{}, error) {
	// 获取指定环境的配置
	item, err := s.GetForEnvironment(ctx, key, envConfig)
	if err != nil {
		return nil, fmt.Errorf("获取环境配置项失败 [%s@%s]: %v", key, envConfig.Environment, err)
	}

	// 如果是环境特定的配置，尝试合并默认配置
	if envConfig != nil && strings.HasSuffix(item.Key, "."+envConfig.Environment) {
		// 获取默认配置
		baseKey := strings.TrimSuffix(item.Key, "."+envConfig.Environment)
		baseItem, err := s.Get(ctx, baseKey)
		if err == nil {
			// 合并默认配置和环境配置
			mergedConfig := make(map[string]interface{})

			// 首先解析默认配置
			baseConfig, err := s.parseCompositeConfig(ctx, baseItem)
			if err != nil {
				s.logger.Warn("解析默认配置失败", zap.String("key", baseKey), zap.Error(err))
			} else {
				// 将默认配置添加到结果中
				for k, v := range baseConfig {
					mergedConfig[k] = v
				}
			}

			// 然后解析环境配置，覆盖默认值
			envValues, err := s.parseCompositeConfigWithEnvironment(ctx, item, envConfig)
			if err != nil {
				return nil, fmt.Errorf("解析环境配置失败 [%s@%s]: %v", key, envConfig.Environment, err)
			}

			// 将环境配置添加到结果中（覆盖默认值）
			for k, v := range envValues {
				mergedConfig[k] = v
			}

			return mergedConfig, nil
		}
	}

	// 如果没有默认配置或不是环境特定配置，直接解析当前配置
	return s.parseCompositeConfigWithEnvironment(ctx, item, envConfig)
}

// MergeCompositeConfigsForEnvironment 合并多个特定环境的组合配置
func (s *Service) MergeCompositeConfigsForEnvironment(ctx context.Context, keys []string, envConfig *EnvironmentConfig) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	for _, key := range keys {
		config, err := s.GetCompositeConfigForEnvironment(ctx, key, envConfig)
		if err != nil {
			return nil, fmt.Errorf("获取环境组合配置失败 [%s@%s]: %v", key, envConfig.Environment, err)
		}

		// 合并配置
		for k, v := range config {
			result[k] = v
		}
	}

	return result, nil
}

// MergeCompositeConfigs 合并多个组合配置
func (s *Service) MergeCompositeConfigs(ctx context.Context, keys []string) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	for _, key := range keys {
		config, err := s.GetCompositeConfig(ctx, key)
		if err != nil {
			return nil, fmt.Errorf("获取组合配置失败 [%s]: %v", key, err)
		}

		// 合并配置
		for k, v := range config {
			result[k] = v
		}
	}

	return result, nil
}

// ExportConfigAsJSON 将组合配置导出为JSON字符串
func (s *Service) ExportConfigAsJSON(ctx context.Context, key string) (string, error) {
	if s.client == nil {
		return "", fmt.Errorf("etcd客户端未初始化或已关闭")
	}

	config, err := s.GetCompositeConfig(ctx, key)
	if err != nil {
		return "", fmt.Errorf("获取组合配置失败: %v", err)
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return "", fmt.Errorf("序列化组合配置失败: %v", err)
	}

	return string(data), nil
}

// ExportConfigAsJSONForEnvironment 将特定环境的组合配置导出为JSON字符串
func (s *Service) ExportConfigAsJSONForEnvironment(ctx context.Context, key string, envConfig *EnvironmentConfig) (string, error) {
	config, err := s.GetCompositeConfigForEnvironment(ctx, key, envConfig)
	if err != nil {
		return "", fmt.Errorf("获取环境组合配置失败: %v", err)
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return "", fmt.Errorf("序列化环境组合配置失败: %v", err)
	}

	return string(data), nil
}

// SetForEnvironment 为特定环境设置配置项
func (s *Service) SetForEnvironment(ctx context.Context, key string, env string, value map[string]*ConfigValue, metadata map[string]string) error {
	// 确保metadata不为nil
	if metadata == nil {
		metadata = make(map[string]string)
	}

	for _, configValue := range value {
		if configValue.Type == ValueTypeEncrypted {
			encrypted, err := s.encryptValue(configValue.Value)
			if err != nil {
				return fmt.Errorf("加密值失败: %v", err)
			}
			configValue.Value = encrypted
		}
	}

	// 设置环境标识
	metadata[MetadataKeyEnvironment] = env

	// 检查key是否已经包含环境后缀，如果是则需要提取基础键
	baseKey := key
	parts := strings.Split(key, ".")
	if len(parts) >= 2 {
		// 检查最后一部分是否是环境名
		lastPart := parts[len(parts)-1]
		// 预定义的环境名列表
		envNames := []string{EnvDevelopment, EnvTesting, EnvStaging, EnvProduction}
		for _, envName := range envNames {
			if lastPart == envName {
				// 如果最后一部分是环境名，使用不带环境的部分作为基础键
				baseKey = strings.Join(parts[:len(parts)-1], ".")
				break
			}
		}
	}

	// 构造环境特定的键
	envKey := fmt.Sprintf("%s.%s", baseKey, env)

	// 保存配置项
	return s.Set(ctx, envKey, value, metadata)
}

// GetForEnvironment 获取特定环境的配置项，如果不存在，则按照回退策略查找
func (s *Service) GetForEnvironment(ctx context.Context, key string, envConfig *EnvironmentConfig) (*ConfigItem, error) {
	if envConfig == nil {
		return s.Get(ctx, key)
	}

	// 检查key是否已经包含环境后缀，如果是则需要提取基础键
	baseKey := key
	parts := strings.Split(key, ".")
	if len(parts) >= 2 {
		// 检查最后一部分是否是环境名
		lastPart := parts[len(parts)-1]
		// 预定义的环境名列表
		envNames := []string{EnvDevelopment, EnvTesting, EnvStaging, EnvProduction}
		for _, envName := range envNames {
			if lastPart == envName {
				// 如果最后一部分是环境名，使用不带环境的部分作为基础键
				baseKey = strings.Join(parts[:len(parts)-1], ".")
				break
			}
		}
	}

	// 尝试获取目标环境的配置
	envKey := fmt.Sprintf("%s.%s", baseKey, envConfig.Environment)
	item, err := s.Get(ctx, envKey)
	if err == nil {
		return item, nil
	}

	// 如果找不到目标环境的配置，尝试回退环境
	for _, fallbackEnv := range envConfig.Fallbacks {
		fallbackKey := fmt.Sprintf("%s.%s", baseKey, fallbackEnv)
		item, err = s.Get(ctx, fallbackKey)
		if err == nil {
			return item, nil
		}
	}

	// 如果所有环境都找不到，尝试获取默认配置（无环境后缀）
	return s.Get(ctx, baseKey)
}

// ListEnvironmentConfigs 列出配置项在所有环境中的版本
func (s *Service) ListEnvironmentConfigs(ctx context.Context, key string) (map[string]*ConfigItem, error) {
	result := make(map[string]*ConfigItem)

	// 首先尝试获取完全匹配的键
	exactItem, err := s.Get(ctx, key)
	if err == nil {
		// 如果配置项有环境元数据，将其归类到对应环境
		if env, ok := exactItem.Metadata[MetadataKeyEnvironment]; ok && env != "" {
			result[env] = exactItem
		} else {
			// 没有环境元数据的作为默认配置
			result["default"] = exactItem
		}
	}

	// 获取所有带环境后缀的配置项
	// 注意：如果key本身已经包含环境后缀（如test1.development），
	// 那么我们需要查找以test1为前缀的所有键，而不是test1.为前缀
	baseKey := key
	parts := strings.Split(key, ".")
	if len(parts) >= 2 {
		// 检查最后一部分是否是环境名
		lastPart := parts[len(parts)-1]
		// 预定义的环境名列表
		envNames := []string{EnvDevelopment, EnvTesting, EnvStaging, EnvProduction}
		for _, envName := range envNames {
			if lastPart == envName {
				// 如果最后一部分是环境名，那么基础键是不带环境的部分
				baseKey = strings.Join(parts[:len(parts)-1], ".")
				break
			}
		}
	}

	// 先尝试查找所有以基础键为前缀的配置项
	values, err := s.client.GetWithPrefix(ctx, s.prefix+baseKey)
	if err != nil {
		s.logger.Error("列出环境配置项失败", zap.String("key", baseKey), zap.Error(err))
		// 即使查询出错，我们也返回已经找到的配置项
		if len(result) > 0 {
			return result, nil
		}
		return nil, fmt.Errorf("列出环境配置项失败: %v", err)
	}

	// 解析每个环境的配置项
	for k, v := range values {
		// 跳过已经处理过的精确匹配键
		if k == s.prefix+key && result["default"] != nil {
			continue
		}

		var item ConfigItem
		if err := json.Unmarshal([]byte(v), &item); err != nil {
			s.logger.Warn("解析环境配置项失败", zap.String("key", k), zap.Error(err))
			continue
		}

		// 优先从metadata中获取环境信息
		if env, ok := item.Metadata[MetadataKeyEnvironment]; ok && env != "" {
			result[env] = &item
			continue
		}

		// 如果metadata中没有环境信息，则从键名中提取
		envKey := k[len(s.prefix):]
		keyParts := strings.Split(envKey, ".")
		if len(keyParts) < 2 {
			continue
		}

		// 检查键名是否与基础键匹配
		if !strings.HasPrefix(envKey, baseKey) {
			continue
		}

		env := keyParts[len(keyParts)-1]
		result[env] = &item
	}

	return result, nil
}

// GetClientConfig 实现Component接口，返回客户端配置
func (s *Service) GetClientConfig() (bool, *config.ClientConfig) {
	return false, nil
}

// ExportAllConfigs 导出所有配置
func (s *Service) ExportAllConfigs(ctx context.Context, password string) ([]byte, error) {
	// 获取所有配置项
	configItems, err := s.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取配置列表失败: %v", err)
	}

	// 遍历所有配置项，解密其中的加密类型数据
	exportData := make([]ExportConfigItem, 0, len(configItems))
	for _, item := range configItems {
		exportItem := ExportConfigItem{
			Key:       item.Key,
			Version:   item.Version,
			UpdatedAt: item.UpdatedAt,
			Metadata:  item.Metadata,
		}

		// 复制并解密值
		exportItem.Value = make(map[string]*ConfigValue)
		for k, v := range item.Value {
			exportValue := &ConfigValue{
				Type:  v.Type,
				Value: v.Value,
			}

			// 对于加密类型的值，进行解密
			if v.Type == ValueTypeEncrypted {
				decrypted, err := s.decryptValue(v.Value)
				if err != nil {
					s.logger.Warn("解密配置值失败",
						zap.String("key", item.Key),
						zap.String("field", k),
						zap.Error(err))
					// 解密失败时保留原始加密值
					exportValue.Value = v.Value
				} else {
					// 解密成功，以明文形式导出，但保留类型为encrypted
					exportValue.Value = decrypted
				}
			}

			exportItem.Value[k] = exportValue
		}

		exportData = append(exportData, exportItem)
	}

	// 将导出数据序列化为JSON
	jsonData, err := json.MarshalIndent(exportData, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("序列化配置数据失败: %v", err)
	}

	// 使用提供的密码加密整个JSON
	encrypted, err := s.encryptExportData(jsonData, password)
	if err != nil {
		return nil, fmt.Errorf("加密导出数据失败: %v", err)
	}

	return encrypted, nil
}

// ExportAllConfigsBase64 导出所有配置并以Base64格式返回
func (s *Service) ExportAllConfigsBase64(ctx context.Context, password string) (string, error) {
	data, err := s.ExportAllConfigs(ctx, password)
	if err != nil {
		return "", err
	}

	// 将二进制数据转换为Base64编码
	return base64.StdEncoding.EncodeToString(data), nil
}

// ImportAllConfigs 导入所有配置
func (s *Service) ImportAllConfigs(ctx context.Context, encryptedData []byte, password string, skipExisting bool) ([]string, error) {
	// 使用提供的密码解密数据
	s.logger.Info("开始导入配置",
		zap.Bool("skipExisting", skipExisting),
		zap.Int("数据长度", len(encryptedData)))

	decrypted, err := s.decryptExportData(encryptedData, password)
	if err != nil {
		s.logger.Error("解密导入数据失败", zap.Error(err))
		return nil, fmt.Errorf("解密导入数据失败: %v", err)
	}

	s.logger.Info("数据解密成功", zap.Int("解密后数据长度", len(decrypted)))

	// 解析JSON数据
	var importData []ExportConfigItem
	if err := json.Unmarshal(decrypted, &importData); err != nil {
		s.logger.Error("解析导入数据失败", zap.Error(err))
		return nil, fmt.Errorf("解析导入数据失败: %v", err)
	}

	s.logger.Info("解析JSON数据成功", zap.Int("配置项数量", len(importData)))

	// 记录跳过的配置
	skippedConfigs := []string{}

	// 遍历并导入每个配置项
	for _, item := range importData {
		// 检查配置是否已存在
		_, err := s.Get(ctx, item.Key)
		if err == nil && skipExisting {
			// 配置已存在且应该跳过
			s.logger.Info("配置已存在，跳过导入",
				zap.String("key", item.Key))
			skippedConfigs = append(skippedConfigs, item.Key)
			continue
		}

		// 创建配置项对象
		configItem := &ConfigItem{
			Key:       item.Key,
			Value:     make(map[string]*ConfigValue),
			Version:   time.Now().UnixNano(), // 使用当前时间作为新版本
			UpdatedAt: time.Now(),
			Metadata:  item.Metadata,
		}

		// 处理每个配置值
		for k, v := range item.Value {
			configValue := &ConfigValue{
				Type:  v.Type,
				Value: v.Value,
			}

			// 对于加密类型的值，重新加密
			if v.Type == ValueTypeEncrypted {
				encrypted, err := s.encryptValue(v.Value)
				if err != nil {
					s.logger.Warn("加密配置值失败",
						zap.String("key", item.Key),
						zap.String("field", k),
						zap.Error(err))
					// 加密失败时保留原始值
					configValue.Value = v.Value
				} else {
					// 重新加密成功
					configValue.Value = encrypted
				}
			}

			configItem.Value[k] = configValue
		}

		// 保存配置项
		data, err := json.Marshal(configItem)
		if err != nil {
			s.logger.Error("序列化配置项失败",
				zap.String("key", item.Key),
				zap.Error(err))
			continue
		}

		if err := s.client.Put(ctx, s.prefix+item.Key, string(data)); err != nil {
			s.logger.Error("保存导入的配置项失败",
				zap.String("key", item.Key),
				zap.Error(err))
		} else {
			s.logger.Info("配置项导入成功", zap.String("key", item.Key))
		}
	}

	s.logger.Info("配置导入完成",
		zap.Int("总配置数", len(importData)),
		zap.Int("跳过配置数", len(skippedConfigs)),
		zap.Strings("跳过配置列表", skippedConfigs))

	return skippedConfigs, nil
}

// ImportAllConfigsBase64 从Base64编码的字符串导入配置
func (s *Service) ImportAllConfigsBase64(ctx context.Context, base64Data string, password string, skipExisting bool) ([]string, error) {
	// 解码Base64数据
	data, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return nil, fmt.Errorf("解码Base64数据失败: %v", err)
	}

	// 调用原有导入方法
	return s.ImportAllConfigs(ctx, data, password, skipExisting)
}

// ExportConfigItem 导出配置项的结构
type ExportConfigItem struct {
	Key       string                  `json:"key"`
	Value     map[string]*ConfigValue `json:"value"`
	Version   int64                   `json:"version"`
	UpdatedAt time.Time               `json:"updated_at"`
	Metadata  map[string]string       `json:"metadata,omitempty"`
}

// encryptExportData 使用密码加密导出数据
func (s *Service) encryptExportData(data []byte, password string) ([]byte, error) {
	// 从密码生成密钥
	key := s.deriveKeyFromPassword(password)

	// 使用AES-GCM加密
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("创建加密器失败: %v", err)
	}

	// 生成随机nonce
	nonce := make([]byte, 12)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("生成nonce失败: %v", err)
	}

	// 使用GCM模式
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("初始化GCM失败: %v", err)
	}

	// 加密数据
	ciphertext := aesgcm.Seal(nil, nonce, data, nil)

	// 将nonce和密文合并
	result := append(nonce, ciphertext...)
	return result, nil
}

// decryptExportData 使用密码解密导入数据
func (s *Service) decryptExportData(data []byte, password string) ([]byte, error) {
	// 确保数据长度足够
	if len(data) < 13 { // 12字节nonce + 至少1字节密文
		return nil, fmt.Errorf("加密数据格式不正确")
	}

	// 从密码生成密钥
	key := s.deriveKeyFromPassword(password)

	// 分离nonce和密文
	nonce := data[:12]
	ciphertext := data[12:]

	// 创建加密器
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("创建加密器失败: %v", err)
	}

	// 使用GCM模式
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("初始化GCM失败: %v", err)
	}

	// 解密数据
	plaintext, err := aesgcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("解密失败: %v", err)
	}

	return plaintext, nil
}

// deriveKeyFromPassword 从密码派生密钥
func (s *Service) deriveKeyFromPassword(password string) []byte {
	// 简单实现，使用密码和salt组合
	// 在生产环境中，应使用更安全的密钥派生函数，如PBKDF2、bcrypt或Argon2
	combined := password + string(s.salt)

	// 使用与系统相同的密钥生成逻辑
	return s.getAESKey(combined)
}

package idgen

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/xsxdot/aio/pkg/distributed/common"
	"sync"
	"sync/atomic"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"
)

// IDGeneratorInfo ID生成器信息
type IDGeneratorInfo struct {
	// Name 生成器名称
	Name string `json:"name"`
	// Step 步长
	Step int64 `json:"step"`
	// CurrentID 当前ID
	CurrentID int64 `json:"currentId"`
	// LastUpdateTime 最后更新时间
	LastUpdateTime time.Time `json:"lastUpdateTime,omitempty"`
	// Status 状态
	Status common.ComponentStatus `json:"status"`
	// CreateTime 创建时间
	CreateTime string `json:"createTime"`
}

// IDGenerator ID生成器接口
type IDGenerator interface {
	// NextID 获取下一个ID
	NextID(ctx context.Context) (int64, error)
	// BatchNextID 批量获取下一组ID
	BatchNextID(ctx context.Context, count int) ([]int64, error)
	// GetInfo 获取生成器信息
	GetInfo() IDGeneratorInfo
}

// IDGenOption ID生成器配置选项函数类型
type IDGenOption func(*idGeneratorImpl)

// WithIDGeneratorStep 设置步长
func WithIDGeneratorStep(step int64) IDGenOption {
	return func(g *idGeneratorImpl) {
		g.step = step
	}
}

// WithIDGeneratorStartID 设置起始ID
func WithIDGeneratorStartID(startID int64) IDGenOption {
	return func(g *idGeneratorImpl) {
		g.currentID = startID
	}
}

// IDGeneratorService ID生成器服务接口
type IDGeneratorService interface {
	common.Component

	// Create 创建ID生成器
	Create(name string, options ...IDGenOption) (IDGenerator, error)
	// Get 获取ID生成器
	Get(name string) (IDGenerator, bool)
	// List 列出所有ID生成器
	List() []IDGeneratorInfo
	// Delete 删除ID生成器
	Delete(name string) error
}

// ID生成器服务实现
type idGeneratorServiceImpl struct {
	etcdClient *clientv3.Client
	logger     *zap.Logger
	generators map[string]IDGenerator
	mutex      sync.RWMutex
	isRunning  bool
}

// ID生成器实现
type idGeneratorImpl struct {
	name       string
	step       int64
	currentID  int64
	buffer     []int64
	bufferPos  int
	etcdClient *clientv3.Client
	logger     *zap.Logger
	mutex      sync.Mutex
	status     common.ComponentStatus
	createTime time.Time
	lastUpdate time.Time
}

// NewIDGeneratorService 创建ID生成器服务
func NewIDGeneratorService(etcdClient *clientv3.Client, logger *zap.Logger) (IDGeneratorService, error) {
	return &idGeneratorServiceImpl{
		etcdClient: etcdClient,
		logger:     logger,
		generators: make(map[string]IDGenerator),
		isRunning:  false,
	}, nil
}

// Start 启动ID生成器服务
func (s *idGeneratorServiceImpl) Start(ctx context.Context) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.isRunning {
		return nil
	}

	s.logger.Info("Starting ID generator service")

	// 从etcd恢复ID生成器配置
	err := s.restoreGeneratorsFromEtcd(ctx)
	if err != nil {
		s.logger.Error("Failed to restore ID generators from etcd", zap.Error(err))
		return err
	}

	s.isRunning = true
	return nil
}

// Stop 停止ID生成器服务
func (s *idGeneratorServiceImpl) Stop(ctx context.Context) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if !s.isRunning {
		return nil
	}

	s.logger.Info("Stopping ID generator service")

	// 保存所有ID生成器的当前状态
	for name, generator := range s.generators {
		impl, ok := generator.(*idGeneratorImpl)
		if ok {
			if err := s.saveGeneratorState(ctx, name, impl); err != nil {
				s.logger.Error("Failed to save generator state",
					zap.String("name", name),
					zap.Error(err))
			}
		}
	}

	s.isRunning = false
	return nil
}

// Create 创建ID生成器
func (s *idGeneratorServiceImpl) Create(name string, options ...IDGenOption) (IDGenerator, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// 检查是否已存在
	if generator, exists := s.generators[name]; exists {
		return generator, nil
	}

	// 创建ID生成器实现
	generator := &idGeneratorImpl{
		name:       name,
		step:       1000, // 默认步长1000
		currentID:  0,    // 默认起始ID为0
		buffer:     make([]int64, 0),
		bufferPos:  0,
		etcdClient: s.etcdClient,
		logger:     s.logger.With(zap.String("generator", name)),
		status:     common.StatusCreated,
		createTime: time.Now(),
	}

	// 应用配置选项
	for _, option := range options {
		option(generator)
	}

	s.logger.Info("Created ID generator",
		zap.String("name", name),
		zap.Int64("step", generator.step),
		zap.Int64("startID", generator.currentID))

	// 保存生成器配置到etcd
	if err := s.saveGeneratorConfig(context.Background(), name, generator); err != nil {
		s.logger.Error("Failed to save generator config", zap.Error(err))
		return nil, err
	}

	// 保存到内存
	s.generators[name] = generator

	return generator, nil
}

// Get 获取ID生成器
func (s *idGeneratorServiceImpl) Get(name string) (IDGenerator, bool) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	generator, exists := s.generators[name]
	return generator, exists
}

// List 列出所有ID生成器
func (s *idGeneratorServiceImpl) List() []IDGeneratorInfo {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	result := make([]IDGeneratorInfo, 0, len(s.generators))
	for _, generator := range s.generators {
		result = append(result, generator.GetInfo())
	}

	return result
}

// Delete 删除ID生成器
func (s *idGeneratorServiceImpl) Delete(name string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	generator, exists := s.generators[name]
	if !exists {
		return nil
	}

	// 保存最终状态
	impl, ok := generator.(*idGeneratorImpl)
	if ok {
		if err := s.saveGeneratorState(context.Background(), name, impl); err != nil {
			s.logger.Error("Failed to save final generator state",
				zap.String("name", name),
				zap.Error(err))
			// 继续删除，不返回错误
		}
	}

	// 从etcd删除生成器配置
	configKey := fmt.Sprintf("/distributed/components/idgens/%s/config", name)
	stateKey := fmt.Sprintf("/distributed/components/idgens/%s/state", name)

	// 使用事务删除配置和状态
	_, err := s.etcdClient.Txn(context.Background()).
		Then(
			clientv3.OpDelete(configKey),
			clientv3.OpDelete(stateKey),
		).
		Commit()

	if err != nil {
		s.logger.Error("Failed to delete generator config from etcd", zap.Error(err))
		return err
	}

	// 从内存中删除
	delete(s.generators, name)
	s.logger.Info("Deleted ID generator", zap.String("name", name))

	return nil
}

// 保存生成器配置到etcd
func (s *idGeneratorServiceImpl) saveGeneratorConfig(ctx context.Context, name string, generator *idGeneratorImpl) error {
	config := map[string]interface{}{
		"step":       generator.step,
		"startID":    generator.currentID,
		"createTime": generator.createTime.Format(time.RFC3339),
	}

	data, err := json.Marshal(config)
	if err != nil {
		return err
	}

	key := fmt.Sprintf("/distributed/components/idgens/%s/config", name)
	_, err = s.etcdClient.Put(ctx, key, string(data))
	if err != nil {
		return err
	}

	// 初始化状态
	return s.saveGeneratorState(ctx, name, generator)
}

// 保存生成器状态到etcd
func (s *idGeneratorServiceImpl) saveGeneratorState(ctx context.Context, name string, generator *idGeneratorImpl) error {
	state := map[string]interface{}{
		"currentID":  generator.currentID,
		"updateTime": time.Now().Format(time.RFC3339),
	}

	data, err := json.Marshal(state)
	if err != nil {
		return err
	}

	key := fmt.Sprintf("/distributed/components/idgens/%s/state", name)
	_, err = s.etcdClient.Put(ctx, key, string(data))
	return err
}

// 修改恢复ID生成器的方法，避免锁嵌套
func (s *idGeneratorServiceImpl) restoreGeneratorsFromEtcd(ctx context.Context) error {
	prefix := "/distributed/components/idgens/"
	resp, err := s.etcdClient.Get(ctx, prefix, clientv3.WithPrefix())
	if err != nil {
		return err
	}

	configMap := make(map[string]map[string]interface{})
	stateMap := make(map[string]map[string]interface{})

	// 解析所有生成器配置和状态
	for _, kv := range resp.Kvs {
		key := string(kv.Key)
		if len(key) <= len(prefix) {
			continue
		}

		// 解析生成器名称和类型
		parts := []rune(key[len(prefix):])
		nameEnd := 0
		for i, c := range parts {
			if c == '/' {
				nameEnd = i
				break
			}
		}

		if nameEnd == 0 {
			continue
		}

		name := string(parts[:nameEnd])
		configType := string(parts[nameEnd+1:])

		var data map[string]interface{}
		if err := json.Unmarshal(kv.Value, &data); err != nil {
			s.logger.Error("Failed to unmarshal generator data",
				zap.String("name", name),
				zap.String("type", configType),
				zap.Error(err))
			continue
		}

		if configType == "config" {
			configMap[name] = data
		} else if configType == "state" {
			stateMap[name] = data
		}
	}

	// 恢复生成器，使用内部方法创建生成器实例
	for name, config := range configMap {
		step, _ := config["step"].(float64)
		startID, _ := config["startID"].(float64)

		// 检查是否有状态信息
		if state, ok := stateMap[name]; ok {
			if currentID, found := state["currentID"].(float64); found {
				// 使用状态中的currentID替代配置中的startID
				startID = currentID
			}
		}

		options := []IDGenOption{
			WithIDGeneratorStep(int64(step)),
			WithIDGeneratorStartID(int64(startID)),
		}

		// 使用内部方法创建生成器，避免加锁导致的死锁
		generator := s.createGeneratorInternal(name, options...)
		s.generators[name] = generator

		s.logger.Info("Restored ID generator from etcd",
			zap.String("name", name),
			zap.Float64("currentID", startID))
	}

	return nil
}

// 添加内部方法，不获取互斥锁
func (s *idGeneratorServiceImpl) createGeneratorInternal(name string, options ...IDGenOption) *idGeneratorImpl {
	// 创建ID生成器实现
	generator := &idGeneratorImpl{
		name:       name,
		step:       1000, // 默认步长1000
		currentID:  0,    // 默认起始ID为0
		buffer:     make([]int64, 0),
		bufferPos:  0,
		etcdClient: s.etcdClient,
		logger:     s.logger.With(zap.String("generator", name)),
		status:     common.StatusCreated,
		createTime: time.Now(),
	}

	// 应用配置选项
	for _, option := range options {
		option(generator)
	}

	s.logger.Info("Internally created ID generator",
		zap.String("name", name),
		zap.Int64("step", generator.step),
		zap.Int64("startID", generator.currentID))

	// 这里不需要保存配置到etcd，因为配置已存在

	return generator
}

// NextID 获取下一个ID
func (g *idGeneratorImpl) NextID(ctx context.Context) (int64, error) {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	// 如果缓冲区中还有ID，直接使用
	if g.bufferPos < len(g.buffer) {
		id := g.buffer[g.bufferPos]
		g.bufferPos++
		return id, nil
	}

	// 缓冲区用尽，需要分配新的ID段
	if err := g.allocateNewIDRange(ctx); err != nil {
		return 0, err
	}

	// 此时缓冲区应该已经有值
	if len(g.buffer) == 0 {
		return 0, fmt.Errorf("failed to allocate new ID range")
	}

	id := g.buffer[0]
	g.bufferPos = 1
	return id, nil
}

// BatchNextID 批量获取下一组ID
func (g *idGeneratorImpl) BatchNextID(ctx context.Context, count int) ([]int64, error) {
	if count <= 0 {
		return []int64{}, nil
	}

	g.mutex.Lock()
	defer g.mutex.Unlock()

	result := make([]int64, count)
	remaining := count
	resultIndex := 0

	// 先使用缓冲区中的ID
	if g.bufferPos < len(g.buffer) {
		available := len(g.buffer) - g.bufferPos
		copyCount := remaining
		if copyCount > available {
			copyCount = available
		}

		for i := 0; i < copyCount; i++ {
			result[resultIndex] = g.buffer[g.bufferPos]
			g.bufferPos++
			resultIndex++
		}

		remaining -= copyCount
	}

	// 如果还需要更多ID，分配新的ID段
	for remaining > 0 {
		if err := g.allocateNewIDRange(ctx); err != nil {
			// 如果已经分配了一些ID，返回部分结果
			if resultIndex > 0 {
				return result[:resultIndex], nil
			}
			return nil, err
		}

		// 使用新分配的ID
		copyCount := remaining
		if copyCount > len(g.buffer) {
			copyCount = len(g.buffer)
		}

		for i := 0; i < copyCount; i++ {
			result[resultIndex] = g.buffer[i]
			resultIndex++
		}

		g.bufferPos = copyCount
		remaining -= copyCount
	}

	return result, nil
}

// 分配新的ID段
func (g *idGeneratorImpl) allocateNewIDRange(ctx context.Context) error {
	// 生成器状态的etcd键
	stateKey := fmt.Sprintf("/distributed/components/idgens/%s/state", g.name)

	// 使用事务更新currentID
	txnResp, err := g.etcdClient.Txn(ctx).
		If(clientv3.Compare(clientv3.ModRevision(stateKey), ">", 0)).
		Then(
			clientv3.OpGet(stateKey),
			clientv3.OpPut(stateKey, ""),
		).
		Else(
			clientv3.OpPut(stateKey, ""),
		).
		Commit()

	if err != nil {
		g.logger.Error("Failed to execute transaction", zap.Error(err))
		return err
	}

	var currentID int64

	// 事务成功且获取到了值
	if txnResp.Succeeded && len(txnResp.Responses) > 0 && len(txnResp.Responses[0].GetResponseRange().Kvs) > 0 {
		kv := txnResp.Responses[0].GetResponseRange().Kvs[0]
		var state map[string]interface{}

		if err := json.Unmarshal(kv.Value, &state); err == nil {
			if id, ok := state["currentID"].(float64); ok {
				currentID = int64(id)
			}
		}
	}

	if currentID < g.currentID {
		currentID = g.currentID
	}

	// 分配新的ID段
	nextID := currentID + g.step

	// 创建新的缓冲区
	g.buffer = make([]int64, g.step)
	for i := int64(0); i < g.step; i++ {
		g.buffer[i] = currentID + i
	}
	g.bufferPos = 0

	// 更新当前ID
	g.currentID = nextID
	g.lastUpdate = time.Now()

	// 保存状态到etcd
	state := map[string]interface{}{
		"currentID":  nextID,
		"updateTime": g.lastUpdate.Format(time.RFC3339),
	}

	data, err := json.Marshal(state)
	if err != nil {
		g.logger.Error("Failed to marshal state", zap.Error(err))
		return err
	}

	_, err = g.etcdClient.Put(ctx, stateKey, string(data))
	if err != nil {
		g.logger.Error("Failed to save state", zap.Error(err))
		return err
	}

	return nil
}

// GetInfo 获取生成器信息
func (g *idGeneratorImpl) GetInfo() IDGeneratorInfo {
	return IDGeneratorInfo{
		Name:           g.name,
		Step:           g.step,
		CurrentID:      atomic.LoadInt64(&g.currentID),
		LastUpdateTime: g.lastUpdate,
		Status:         g.status,
		CreateTime:     g.createTime.Format(time.RFC3339),
	}
}

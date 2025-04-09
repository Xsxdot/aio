package config

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/xsxdot/aio/pkg/common"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xsxdot/aio/internal/etcd"
	"go.uber.org/zap"
)

var (
	testDataDir = "testdata"
)

func setupTest(t *testing.T) (*zap.Logger, *etcd.EtcdClient) {
	t.Helper()

	// 创建测试数据目录
	err := os.MkdirAll(testDataDir, 0755)
	if err != nil {
		t.Fatalf("创建测试数据目录失败: %v", err)
	}

	// 创建日志记录器
	logger, err := zap.NewDevelopment()
	if err != nil {
		t.Fatalf("创建日志记录器失败: %v", err)
	}

	// 创建etcd服务器配置
	serverConfig := etcd.NewDefaultServerConfig("test-node", filepath.Join(testDataDir, "etcd"))
	serverConfig.ClientURLs = []string{"http://localhost:12379"}
	serverConfig.PeerURLs = []string{"http://localhost:12380"}
	serverConfig.InitialCluster = fmt.Sprintf("%s=http://localhost:12380", serverConfig.Name)

	// 初始化etcd服务器
	err = etcd.InitGlobalEtcdServer(serverConfig, logger)
	if err != nil {
		t.Fatalf("初始化etcd服务器失败: %v", err)
	}

	// 创建etcd客户端配置
	clientConfig := etcd.NewDefaultClientConfig()
	clientConfig.Endpoints = []string{"localhost:12379"}
	clientConfig.DialTimeout = 5 * time.Second

	// 创建etcd客户端
	client, err := etcd.NewEtcdClient(clientConfig, logger)
	if err != nil {
		t.Fatalf("创建etcd客户端失败: %v", err)
	}

	return logger, client
}

func cleanupTest(t *testing.T, logger *zap.Logger) {
	t.Helper()

	// 关闭etcd服务器和客户端
	etcd.CloseGlobalEtcdServer()

	// 清理测试数据目录
	err := os.RemoveAll(testDataDir)
	if err != nil {
		t.Errorf("清理测试数据目录失败: %v", err)
	}

	// 同步日志
	logger.Sync()
}

func TestConfigService(t *testing.T) {
	logger, client := setupTest(t)
	defer cleanupTest(t, logger)

	// 创建配置中心服务
	service := NewService(client, logger)
	defer service.Close()

	// 测试上下文
	ctx := context.Background()

	// 测试设置配置项
	testKey := "test-config"
	testValue := map[string]*ConfigValue{
		"string": {Value: "hello", Type: ValueTypeString},
		"int":    {Value: "123", Type: ValueTypeInt},
		"float":  {Value: "3.14", Type: ValueTypeFloat},
		"bool":   {Value: "true", Type: ValueTypeBool},
	}
	testMetadata := map[string]string{
		"env": "test",
	}

	err := service.Set(ctx, testKey, testValue, testMetadata)
	if err != nil {
		t.Fatalf("设置配置项失败: %v", err)
	}

	// 测试获取配置项
	item, err := service.Get(ctx, testKey)
	if err != nil {
		t.Fatalf("获取配置项失败: %v", err)
	}

	// 验证配置项内容
	if item.Key != testKey {
		t.Errorf("配置项键不匹配: 期望 %s, 得到 %s", testKey, item.Key)
	}

	if len(item.Value) != len(testValue) {
		t.Errorf("配置项值数量不匹配: 期望 %d, 得到 %d", len(testValue), len(item.Value))
	}

	if item.Metadata["env"] != testMetadata["env"] {
		t.Errorf("配置项元数据不匹配: 期望 %s, 得到 %s", testMetadata["env"], item.Metadata["env"])
	}

	// 测试列出配置项
	items, err := service.List(ctx)
	if err != nil {
		t.Fatalf("列出配置项失败: %v", err)
	}

	if len(items) != 1 {
		t.Errorf("配置项列表数量不匹配: 期望 1, 得到 %d", len(items))
	}

	// 测试删除配置项
	err = service.Delete(ctx, testKey)
	if err != nil {
		t.Fatalf("删除配置项失败: %v", err)
	}

	// 验证配置项已删除
	_, err = service.Get(ctx, testKey)
	if err == nil {
		t.Error("配置项应该已被删除")
	}
}

func TestConfigHistory(t *testing.T) {
	logger, etcdClient := setupTest(t)
	defer cleanupTest(t, logger)

	// 创建配置中心服务
	service := NewService(etcdClient, logger)
	defer service.Close()

	// 测试上下文
	ctx := context.Background()

	// 测试配置项
	testKey := "test-history-config"
	testValues := []map[string]*ConfigValue{
		{
			"version": {Value: "1.0.0", Type: ValueTypeString},
			"port":    {Value: "8080", Type: ValueTypeInt},
		},
		{
			"version": {Value: "1.0.1", Type: ValueTypeString},
			"port":    {Value: "8081", Type: ValueTypeInt},
		},
		{
			"version": {Value: "1.0.2", Type: ValueTypeString},
			"port":    {Value: "8082", Type: ValueTypeInt},
		},
	}

	// 依次设置三个不同的版本
	for _, value := range testValues {
		err := service.Set(ctx, testKey, value, nil)
		if err != nil {
			t.Fatalf("设置配置失败: %v", err)
		}
		// 等待一小段时间确保版本号不同
		time.Sleep(time.Millisecond * 10)
	}

	// 获取历史版本
	history, err := service.GetHistory(ctx, testKey, 10)
	if err != nil {
		t.Fatalf("获取历史版本失败: %v", err)
	}

	// 验证历史版本数量
	if len(history) != len(testValues) {
		t.Errorf("历史版本数量不匹配: 期望 %d, 得到 %d", len(testValues), len(history))
	}

	// 验证最新的版本是最后设置的值
	latest := history[0]
	expectedLatest := testValues[len(testValues)-1]
	if latest.Value["port"].Value != expectedLatest["port"].Value {
		t.Errorf("最新版本不匹配: 期望 %s, 得到 %s", expectedLatest["port"].Value, latest.Value["port"].Value)
	}

	// 获取第一个版本
	firstVersion := history[len(history)-1].ModRevision
	firstItem, err := service.GetByRevision(ctx, testKey, firstVersion)
	if err != nil {
		t.Fatalf("获取指定版本失败: %v", err)
	}

	// 验证第一个版本的值
	expectedFirst := testValues[0]
	if firstItem.Value["port"].Value != expectedFirst["port"].Value {
		t.Errorf("第一个版本不匹配: 期望 %s, 得到 %s", expectedFirst["port"].Value, firstItem.Value["port"].Value)
	}

	// 删除配置后获取历史版本
	err = service.Delete(ctx, testKey)
	if err != nil {
		t.Fatalf("删除配置失败: %v", err)
	}

	// 获取已删除配置的历史版本
	history, err = service.GetHistory(ctx, testKey, 10)
	if err != nil {
		t.Fatalf("获取已删除配置的历史失败: %v", err)
	}

	// 验证历史版本数量不变
	if len(history) != len(testValues) {
		t.Errorf("删除后历史版本数量不匹配: 期望 %d, 得到 %d", len(testValues), len(history))
	}
}

// TestCompositeConfig 测试组合配置功能
func TestCompositeConfig(t *testing.T) {
	logger, client := setupTest(t)
	defer cleanupTest(t, logger)

	// 创建配置中心服务
	service := NewService(client, logger)
	defer service.Close()

	ctx := context.Background()

	// 创建基础配置项
	baseKey := "base-config"
	baseValue := map[string]*ConfigValue{
		"app_name": {Value: "测试应用", Type: ValueTypeString},
		"version":  {Value: "1.0.0", Type: ValueTypeString},
		"debug":    {Value: "true", Type: ValueTypeBool},
	}
	err := service.Set(ctx, baseKey, baseValue, nil)
	if err != nil {
		t.Fatalf("设置基础配置项失败: %v", err)
	}

	// 创建数据库配置
	dbKey := "db-config"
	dbValue := map[string]*ConfigValue{
		"host":     {Value: "localhost", Type: ValueTypeString},
		"port":     {Value: "3306", Type: ValueTypeInt},
		"username": {Value: "root", Type: ValueTypeString},
		"password": {Value: "password", Type: ValueTypeString},
		"params": {
			Value: `{"charset":"utf8mb4","parseTime":"true"}`,
			Type:  ValueTypeObject,
		},
	}
	err = service.Set(ctx, dbKey, dbValue, nil)
	if err != nil {
		t.Fatalf("设置数据库配置项失败: %v", err)
	}

	// 创建引用配置项
	compositeKey := "composite-config"
	refBaseValue, _ := json.Marshal(RefValue{Key: baseKey, Property: ""})
	refDbValue, _ := json.Marshal(RefValue{Key: dbKey, Property: ""})
	refAppNameValue, _ := json.Marshal(RefValue{Key: baseKey, Property: "app_name"})

	compositeValue := map[string]*ConfigValue{
		"base":        {Value: string(refBaseValue), Type: ValueTypeRef},
		"database":    {Value: string(refDbValue), Type: ValueTypeRef},
		"environment": {Value: "production", Type: ValueTypeString},
		"app_title":   {Value: string(refAppNameValue), Type: ValueTypeRef},
		"servers": {
			Value: `["server1","server2","server3"]`,
			Type:  ValueTypeArray,
		},
	}
	err = service.Set(ctx, compositeKey, compositeValue, nil)
	if err != nil {
		t.Fatalf("设置组合配置项失败: %v", err)
	}

	// 测试获取组合配置
	config, err := service.GetCompositeConfig(ctx, compositeKey)
	if err != nil {
		t.Fatalf("获取组合配置失败: %v", err)
	}

	// 验证组合配置内容
	baseConfig, ok := config["base"].(map[string]interface{})
	if !ok {
		t.Error("base配置应该是对象类型")
	} else {
		if baseConfig["app_name"] != "测试应用" {
			t.Errorf("base.app_name不匹配: 期望 %s, 得到 %v", "测试应用", baseConfig["app_name"])
		}
	}

	dbConfig, ok := config["database"].(map[string]interface{})
	if !ok {
		t.Error("database配置应该是对象类型")
	} else {
		if dbConfig["host"] != "localhost" {
			t.Errorf("database.host不匹配: 期望 %s, 得到 %v", "localhost", dbConfig["host"])
		}

		if port, ok := dbConfig["port"].(int64); !ok || port != 3306 {
			t.Errorf("database.port不匹配: 期望 %d, 得到 %v", 3306, dbConfig["port"])
		}

		params, ok := dbConfig["params"].(map[string]interface{})
		if !ok {
			t.Error("database.params应该是对象类型")
		} else {
			if params["charset"] != "utf8mb4" {
				t.Errorf("database.params.charset不匹配: 期望 %s, 得到 %v", "utf8mb4", params["charset"])
			}
		}
	}

	if config["app_title"] != "测试应用" {
		t.Errorf("app_title不匹配: 期望 %s, 得到 %v", "测试应用", config["app_title"])
	}

	if config["environment"] != "production" {
		t.Errorf("environment不匹配: 期望 %s, 得到 %v", "production", config["environment"])
	}

	servers, ok := config["servers"].([]interface{})
	if !ok {
		t.Error("servers应该是数组类型")
	} else {
		if len(servers) != 3 {
			t.Errorf("servers数组长度不匹配: 期望 %d, 得到 %d", 3, len(servers))
		}
	}

	// 测试导出JSON
	jsonStr, err := service.ExportConfigAsJSON(ctx, compositeKey)
	if err != nil {
		t.Fatalf("导出JSON失败: %v", err)
	}

	// 验证JSON是否可以解析回相同的结构
	var jsonConfig map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &jsonConfig); err != nil {
		t.Fatalf("解析导出的JSON失败: %v", err)
	}

	// 测试合并配置
	mergedConfig, err := service.MergeCompositeConfigs(ctx, []string{baseKey, dbKey})
	if err != nil {
		t.Fatalf("合并配置失败: %v", err)
	}

	if mergedConfig["app_name"] != "测试应用" {
		t.Errorf("合并配置的app_name不匹配: 期望 %s, 得到 %v", "测试应用", mergedConfig["app_name"])
	}

	if mergedConfig["host"] != "localhost" {
		t.Errorf("合并配置的host不匹配: 期望 %s, 得到 %v", "localhost", mergedConfig["host"])
	}
}

func TestMultiEnvironmentConfig(t *testing.T) {
	logger, client := setupTest(t)
	defer cleanupTest(t, logger)

	// 创建配置中心服务
	svc := NewService(client, logger)
	defer svc.Close()

	ctx := context.Background()

	// 1. 设置基础配置（默认配置）
	baseConfig := map[string]*ConfigValue{
		"app_name": {
			Value: "MyApp",
			Type:  ValueTypeString,
		},
		"max_connections": {
			Value: "100",
			Type:  ValueTypeInt,
		},
		"debug": {
			Value: "false",
			Type:  ValueTypeBool,
		},
	}
	err := svc.Set(ctx, "app.config", baseConfig, nil)
	require.NoError(t, err, "设置基础配置失败")

	// 2. 设置开发环境特定配置
	devConfig := map[string]*ConfigValue{
		"debug": {
			Value: "true",
			Type:  ValueTypeBool,
		},
		"dev_tools_enabled": {
			Value: "true",
			Type:  ValueTypeBool,
		},
	}
	err = svc.SetForEnvironment(ctx, "app.config", EnvDevelopment, devConfig, nil)
	require.NoError(t, err, "设置开发环境配置失败")

	// 3. 设置生产环境特定配置
	prodConfig := map[string]*ConfigValue{
		"max_connections": {
			Value: "1000",
			Type:  ValueTypeInt,
		},
	}
	err = svc.SetForEnvironment(ctx, "app.config", EnvProduction, prodConfig, nil)
	require.NoError(t, err, "设置生产环境配置失败")

	// 4. 测试环境配置获取
	t.Run("获取开发环境配置", func(t *testing.T) {
		envConfig := NewEnvironmentConfig(EnvDevelopment)
		config, err := svc.GetCompositeConfigForEnvironment(ctx, "app.config", envConfig)
		require.NoError(t, err, "获取开发环境配置失败")

		// 验证配置值
		assert.Equal(t, "MyApp", config["app_name"], "app_name应该继承自基础配置")
		assert.Equal(t, int64(100), config["max_connections"], "max_connections应该继承自基础配置")
		assert.Equal(t, true, config["debug"], "debug应该被开发环境覆盖为true")
		assert.Equal(t, true, config["dev_tools_enabled"], "dev_tools_enabled应该存在于开发环境")

		jsonStr, err := svc.ExportConfigAsJSONForEnvironment(ctx, "app.config", envConfig)
		require.NoError(t, err, "导出开发环境配置失败")
		assert.Contains(t, jsonStr, "MyApp", "导出的JSON应该包含app_name")
		assert.Contains(t, jsonStr, "true", "导出的JSON应该包含debug=true")
		common.GetLogger().Info(jsonStr)
	})

	t.Run("获取生产环境配置", func(t *testing.T) {
		envConfig := NewEnvironmentConfig(EnvProduction)
		config, err := svc.GetCompositeConfigForEnvironment(ctx, "app.config", envConfig)
		require.NoError(t, err, "获取生产环境配置失败")

		// 验证配置值
		assert.Equal(t, "MyApp", config["app_name"], "app_name应该继承自基础配置")
		assert.Equal(t, int64(1000), config["max_connections"], "max_connections应该被生产环境覆盖")
		assert.Equal(t, false, config["debug"], "debug应该继承自基础配置")
		assert.NotContains(t, config, "dev_tools_enabled", "dev_tools_enabled不应该存在于生产环境")
	})

	// 5. 测试配置引用
	t.Run("测试配置引用", func(t *testing.T) {
		// 设置一个包含引用的配置
		dbConfig := map[string]*ConfigValue{
			"host": {
				Value: "localhost",
				Type:  ValueTypeString,
			},
			"port": {
				Value: "5432",
				Type:  ValueTypeInt,
			},
		}
		err := svc.Set(ctx, "db.config", dbConfig, nil)
		require.NoError(t, err, "设置数据库配置失败")

		// 设置引用配置
		appConfigWithRef := map[string]*ConfigValue{
			"database": {
				Value: `{"key":"db.config"}`,
				Type:  ValueTypeRef,
			},
		}
		err = svc.Set(ctx, "app.full_config", appConfigWithRef, nil)
		require.NoError(t, err, "设置引用配置失败")

		// 获取并验证引用配置
		config, err := svc.GetCompositeConfig(ctx, "app.full_config")
		require.NoError(t, err, "获取引用配置失败")

		dbSettings, ok := config["database"].(map[string]interface{})
		require.True(t, ok, "database应该是一个map")
		assert.Equal(t, "localhost", dbSettings["host"], "数据库主机配置不正确")
		assert.Equal(t, int64(5432), dbSettings["port"], "数据库端口配置不正确")
	})

	// 6. 测试环境配置列表
	t.Run("测试环境配置列表", func(t *testing.T) {
		configs, err := svc.ListEnvironmentConfigs(ctx, "app.config")
		require.NoError(t, err, "获取环境配置列表失败")

		// 验证是否包含所有环境的配置
		assert.Contains(t, configs, "default", "应该包含默认配置")
		assert.Contains(t, configs, EnvDevelopment, "应该包含开发环境配置")
		assert.Contains(t, configs, EnvProduction, "应该包含生产环境配置")

		// 验证每个环境的配置内容
		assert.Equal(t, "MyApp", configs["default"].Value["app_name"].Value, "默认配置的app_name不正确")
		assert.Equal(t, "true", configs[EnvDevelopment].Value["debug"].Value, "开发环境的debug配置不正确")
		assert.Equal(t, "1000", configs[EnvProduction].Value["max_connections"].Value, "生产环境的max_connections配置不正确")
	})

	// 7. 测试配置导出
	t.Run("测试配置导出", func(t *testing.T) {
		envConfig := NewEnvironmentConfig(EnvDevelopment)
		jsonConfig, err := svc.ExportConfigAsJSONForEnvironment(ctx, "app.config", envConfig)
		require.NoError(t, err, "导出开发环境配置失败")
		assert.Contains(t, jsonConfig, "MyApp", "导出的JSON应该包含app_name")
		assert.Contains(t, jsonConfig, "true", "导出的JSON应该包含debug=true")
	})
}

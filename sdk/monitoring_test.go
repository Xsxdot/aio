package sdk

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMetricsCollector 测试指标收集器
func TestMetricsCollector(t *testing.T) {
	client, err := createTestClient()
	require.NoError(t, err, "创建客户端失败")

	// 首先连接
	err = client.Connect()
	require.NoError(t, err, "连接服务器失败")
	defer client.Close()

	// 创建指标收集选项
	options := &MetricsCollectorOptions{
		ServiceName:           "test-service",
		StatusCollectInterval: 3 * time.Second,
		AutoCollectStatus:     true,
		DisableMetrics:        false,
	}

	// 启动指标收集
	_, err = client.StartMetrics(options)
	if err != nil {
		t.Logf("启动指标收集失败: %v", err)
	} else {
		t.Log("成功启动指标收集")

		// 等待收集一些指标
		time.Sleep(5 * time.Second)

		// 验证指标收集器已启动
		assert.NotNil(t, client.Metrics, "指标收集器不应为空")

		// 停止指标收集
		err = client.StopMetrics()
		assert.NoError(t, err, "停止指标收集失败")
	}
}

// TestMonitoringClient 测试监控客户端
func TestMonitoringClient(t *testing.T) {
	client, err := createTestClient()
	require.NoError(t, err, "创建客户端失败")

	// 首先连接
	err = client.Connect()
	require.NoError(t, err, "连接服务器失败")
	defer client.Close()

	// 延迟运行，确保有足够的时间连接服务器
	time.Sleep(1 * time.Second)

	// 测试获取监控客户端
	monitoringClient := NewMonitoringClient(client)
	assert.NotNil(t, monitoringClient, "监控客户端不应为空")

	// 测试获取系统状态
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	systemStatus, err := monitoringClient.GetSystemStatus(ctx)
	if err != nil {
		t.Logf("获取系统状态失败: %v", err)
	} else {
		t.Logf("系统状态: CPU使用率=%.2f%%, 内存使用率=%.2f%%",
			systemStatus.CpuUsage, systemStatus.MemoryUsage)
		assert.GreaterOrEqual(t, systemStatus.CpuUsage, 0.0, "CPU使用率应该>=0")
		assert.GreaterOrEqual(t, systemStatus.MemoryUsage, 0.0, "内存使用率应该>=0")
	}

	// 测试获取节点状态
	nodeStatus, err := monitoringClient.GetNodesStatus(ctx)
	if err != nil {
		t.Logf("获取节点状态失败: %v", err)
	} else {
		t.Logf("获取到 %d 个节点状态", len(nodeStatus))
		for id, status := range nodeStatus {
			t.Logf("节点 %s: 状态=%s, 已运行=%s",
				id, status.State, status.Uptime)
		}
	}

	// 测试获取服务状态
	servicesStatus, err := monitoringClient.GetServicesStatus(ctx)
	if err != nil {
		t.Logf("获取服务状态失败: %v", err)
	} else {
		t.Logf("获取到 %d 个服务状态", len(servicesStatus))
		for name, status := range servicesStatus {
			t.Logf("服务 %s: 实例数=%d, 可用性=%.2f%%",
				name, status.InstanceCount, status.Availability*100)
		}
	}
}

// TestMonitoringMetrics 测试监控指标
func TestMonitoringMetrics(t *testing.T) {
	client, err := createTestClient()
	require.NoError(t, err, "创建客户端失败")

	// 首先连接
	err = client.Connect()
	require.NoError(t, err, "连接服务器失败")
	defer client.Close()

	// 测试获取监控客户端
	monitoringClient := NewMonitoringClient(client)
	assert.NotNil(t, monitoringClient, "监控客户端不应为空")

	// 测试获取CPU指标
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cpuMetrics, err := monitoringClient.GetCpuMetrics(ctx, "1h")
	if err != nil {
		t.Logf("获取CPU指标失败: %v", err)
	} else {
		t.Logf("获取到 %d 个CPU指标点", len(cpuMetrics.Points))
		if len(cpuMetrics.Points) > 0 {
			firstPoint := cpuMetrics.Points[0]
			t.Logf("CPU指标: 时间=%s, 值=%.2f%%",
				firstPoint.Timestamp.Format(time.RFC3339), firstPoint.Value)
		}
	}

	// 测试获取内存指标
	memoryMetrics, err := monitoringClient.GetMemoryMetrics(ctx, "1h")
	if err != nil {
		t.Logf("获取内存指标失败: %v", err)
	} else {
		t.Logf("获取到 %d 个内存指标点", len(memoryMetrics.Points))
		if len(memoryMetrics.Points) > 0 {
			firstPoint := memoryMetrics.Points[0]
			t.Logf("内存指标: 时间=%s, 值=%.2f%%",
				firstPoint.Timestamp.Format(time.RFC3339), firstPoint.Value)
		}
	}

	// 测试获取网络指标
	networkMetrics, err := monitoringClient.GetNetworkMetrics(ctx, "1h")
	if err != nil {
		t.Logf("获取网络指标失败: %v", err)
	} else {
		t.Logf("获取到 %d 个网络指标点", len(networkMetrics.Points))
		if len(networkMetrics.Points) > 0 {
			firstPoint := networkMetrics.Points[0]
			t.Logf("网络指标: 时间=%s, 值=%.2f",
				firstPoint.Timestamp.Format(time.RFC3339), firstPoint.Value)
		}
	}
}

// TestAlertManagement 测试告警管理
func TestAlertManagement(t *testing.T) {
	client, err := createTestClient()
	require.NoError(t, err, "创建客户端失败")

	// 首先连接
	err = client.Connect()
	require.NoError(t, err, "连接服务器失败")
	defer client.Close()

	// 测试获取监控客户端
	monitoringClient := NewMonitoringClient(client)
	assert.NotNil(t, monitoringClient, "监控客户端不应为空")

	// 测试获取告警
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	alerts, err := monitoringClient.GetAlerts(ctx)
	if err != nil {
		t.Logf("获取告警失败: %v", err)
	} else {
		t.Logf("获取到 %d 个告警", len(alerts))
		for _, alert := range alerts {
			t.Logf("告警: ID=%s, 级别=%s, 消息=%s",
				alert.ID, alert.Level, alert.Message)
		}
	}

	// 测试创建告警规则
	rule := AlertRule{
		Name:      "test-rule",
		Metric:    "cpu_usage",
		Threshold: 90.0,
		Operator:  ">",
		Duration:  "5m",
		Level:     "warning",
		Message:   "CPU使用率超过90%",
	}

	ruleID, err := monitoringClient.CreateAlertRule(ctx, rule)
	if err != nil {
		t.Logf("创建告警规则失败: %v", err)
	} else {
		t.Logf("成功创建告警规则，ID=%s", ruleID)

		// 测试删除告警规则
		err = monitoringClient.DeleteAlertRule(ctx, ruleID)
		if err != nil {
			t.Logf("删除告警规则失败: %v", err)
		} else {
			t.Log("成功删除告警规则")
		}
	}
}

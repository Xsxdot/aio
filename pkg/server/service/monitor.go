package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/xsxdot/aio/internal/etcd"
	"github.com/xsxdot/aio/pkg/common"
	"github.com/xsxdot/aio/pkg/monitoring/collector"
	"github.com/xsxdot/aio/pkg/registry"
	"github.com/xsxdot/aio/pkg/scheduler"
	"github.com/xsxdot/aio/pkg/server"

	"go.uber.org/zap"
)

var aioServiceName = "aio-service"

// MonitorAssignment 监控分配信息
type MonitorAssignment struct {
	ServerID     string    `json:"server_id"`
	ServerName   string    `json:"server_name"`
	AssignedNode string    `json:"assigned_node"`
	AssignTime   time.Time `json:"assign_time"`
}

// MonitorManager 监控管理器
type MonitorManager struct {
	service          server.Service
	executor         server.Executor
	registry         registry.Registry
	etcdClient       *etcd.EtcdClient
	collector        *collector.ServerCollector
	schedulerManager *scheduler.Scheduler
	logger           *zap.Logger

	// 监控分配相关
	ctx    context.Context
	cancel context.CancelFunc
	nodeID string // 当前节点ID
	taskId string // 定时任务ID
}

func NewMonitorManager(service server.Service, executor server.Executor, registry registry.Registry, etcdClient *etcd.EtcdClient, collector *collector.ServerCollector, schedulerManager *scheduler.Scheduler) *MonitorManager {
	ctx, cancel := context.WithCancel(context.Background())

	hostname, _ := os.Hostname()
	nodeID := fmt.Sprintf("%s-%d", hostname, os.Getpid())

	logger := common.GetLogger().GetZapLogger("monitor-server")

	m := &MonitorManager{
		service:          service,
		executor:         executor,
		registry:         registry,
		etcdClient:       etcdClient,
		collector:        collector,
		schedulerManager: schedulerManager,
		logger:           logger,
		ctx:              ctx,
		cancel:           cancel,
		nodeID:           nodeID,
	}

	schedulerManager.AddTask(scheduler.NewIntervalTask(
		"monitor-server",
		time.Now().Add(time.Second*30),
		time.Second*30,
		scheduler.TaskExecuteModeLocal,
		time.Second*30,
		m.monitorServer,
	))
	return m
}

func (m *MonitorManager) AddServer(server *server.Server) {
	if server.InstallAIO {
		m.logger.Debug("跳过已安装AIO的服务器", zap.String("server_id", server.ID))
		return
	}

	// 发现所有AIO服务节点
	services, err := m.registry.Discover(m.ctx, aioServiceName)
	if err != nil {
		m.logger.Error("发现AIO服务失败", zap.Error(err))
		return
	}

	if len(services) == 0 {
		m.logger.Warn("没有找到可用的AIO节点")
		return
	}

	// 检查是否已经有监控分配
	assignmentKey := m.getAssignmentKey(server.ID)
	existingData, err := m.etcdClient.Get(m.ctx, assignmentKey)
	if err == nil && existingData != "" {
		m.logger.Debug("服务器已有监控分配", zap.String("server_id", server.ID))
		return
	}

	// 找到负载最小的节点
	selectedNode := m.selectLeastLoadedNode(services)
	if selectedNode == nil {
		m.logger.Error("无法选择合适的监控节点")
		return
	}

	// 创建监控分配记录
	assignment := MonitorAssignment{
		ServerID:     server.ID,
		ServerName:   server.Name,
		AssignedNode: selectedNode.ID,
		AssignTime:   time.Now(),
	}

	// 保存到etcd
	assignmentData, err := json.Marshal(assignment)
	if err != nil {
		m.logger.Error("序列化监控分配失败", zap.Error(err))
		return
	}

	if err := m.etcdClient.Put(m.ctx, assignmentKey, string(assignmentData)); err != nil {
		m.logger.Error("保存监控分配到etcd失败", zap.Error(err))
		return
	}

	m.logger.Info("添加服务器监控分配成功",
		zap.String("server_id", server.ID),
		zap.String("server_name", server.Name),
		zap.String("assigned_node", selectedNode.ID))
}

func (m *MonitorManager) RemoveServer(server *server.Server) {
	// 从etcd中删除监控分配
	assignmentKey := m.getAssignmentKey(server.ID)

	// 先检查是否存在
	existingData, err := m.etcdClient.Get(m.ctx, assignmentKey)
	if err != nil || existingData == "" {
		m.logger.Debug("服务器监控分配不存在", zap.String("server_id", server.ID))
		return
	}

	// 删除分配记录
	if err := m.etcdClient.Delete(m.ctx, assignmentKey); err != nil {
		m.logger.Error("删除监控分配失败",
			zap.String("server_id", server.ID),
			zap.Error(err))
		return
	}

	m.logger.Info("删除服务器监控分配成功",
		zap.String("server_id", server.ID),
		zap.String("server_name", server.Name))
}

func (m *MonitorManager) EditServer(server *server.Server) {
	// 检查是否已经有监控分配
	assignmentKey := m.getAssignmentKey(server.ID)
	existingData, err := m.etcdClient.Get(m.ctx, assignmentKey)

	// 服务器没有监控分配
	if err != nil || existingData == "" {
		m.logger.Debug("服务器没有监控分配", zap.String("server_id", server.ID))
		// 没有监控分配，调用AddServer
		m.AddServer(server)
		return
	}

	// 服务器已有监控分配
	m.logger.Debug("服务器已有监控分配", zap.String("server_id", server.ID))

	if server.InstallAIO {
		// 如果InstallAIO为true，删除这个节点的监控分配
		m.logger.Info("服务器已安装AIO，删除监控分配",
			zap.String("server_id", server.ID),
			zap.String("server_name", server.Name))
		m.RemoveServer(server)
	} else {
		// 如果InstallAIO为false，不做任何操作
		m.logger.Debug("服务器未安装AIO，保持现有监控分配",
			zap.String("server_id", server.ID),
			zap.String("server_name", server.Name))
	}
}

func (m *MonitorManager) monitorServer(ctx context.Context) error {
	// 从etcd中找到分配给自己的监控列表
	assignments, err := m.getMyAssignments()
	if err != nil {
		m.logger.Error("获取监控分配列表失败", zap.Error(err))
		return err
	}

	if len(assignments) == 0 {
		m.logger.Debug("当前节点没有分配的监控任务")
		return nil
	}

	m.logger.Debug("开始执行监控任务", zap.Int("server_count", len(assignments)))

	// 为每个分配的服务器收集监控数据
	for _, assignment := range assignments {
		if err := m.monitorSingleServer(ctx, assignment.ServerID); err != nil {
			m.logger.Error("监控单个服务器失败",
				zap.String("server_id", assignment.ServerID),
				zap.Error(err))
			// 继续监控其他服务器，不要因为一个失败而中断
		}
	}

	return nil
}

// monitorSingleServer 监控单个服务器
func (m *MonitorManager) monitorSingleServer(ctx context.Context, serverID string) error {
	// 使用server构建监控命令
	cmd := server.NewBatchCommand("服务器监控").
		Parallel().
		ContinueOnFailed().
		Try(
			// CPU监控
			server.NewCommand("CPU使用率", "top -b -n1 | grep 'Cpu(s)' | awk '{print $2}' | awk -F'%' '{print $1}'").Build(),
			server.NewCommand("CPU负载", "uptime | awk -F'load average:' '{print $2}' | awk '{print $1,$2,$3}'").Build(),

			// 内存监控
			server.NewCommand("内存信息", "free -m | grep Mem | awk '{printf \"%.2f,%.2f,%.2f,%.2f\", $3/$2*100, $2, $3, $4}'").Build(),

			// 磁盘监控
			server.NewCommand("磁盘使用率", "df -h / | tail -1 | awk '{print $5}' | sed 's/%//'").Build(),
			server.NewCommand("磁盘详细信息", "df -BG / | tail -1 | awk '{printf \"%.2f,%.2f,%.2f\", $2, $3, $4}' | sed 's/G//g'").Build(),

			// 网络监控
			server.NewCommand("网络接口统计", "cat /proc/net/dev | grep -E '(eth|ens|enp)' | head -1 | awk '{printf \"%.0f,%.0f,%.0f,%.0f\", $2, $10, $3, $11}'").Build(),
		).
		Build()

	// 执行监控命令
	result, err := m.executor.Execute(ctx, &server.ExecuteRequest{
		ServerID:     serverID,
		Type:         server.CommandTypeBatch,
		BatchCommand: cmd,
	})
	if err != nil {
		return fmt.Errorf("执行监控命令失败: %w", err)
	}

	// 解析结果并存储
	metrics, err := m.parseMonitoringResult(result.BatchResult, serverID)
	if err != nil {
		m.logger.Error("解析监控结果失败", zap.Error(err))
		return err
	}

	// 使用collector存储结果
	if err := m.collector.StoreExternalServerMetrics(metrics); err != nil {
		return fmt.Errorf("存储监控指标失败: %w", err)
	}

	m.logger.Debug("服务器监控数据收集完成", zap.String("server_id", serverID))
	return nil
}

// parseMonitoringResult 解析监控命令结果
func (m *MonitorManager) parseMonitoringResult(result *server.BatchResult, serverID string) (*collector.ServerMetrics, error) {
	if result == nil || result.TryResults == nil {
		return nil, fmt.Errorf("监控结果为空")
	}

	// 获取服务器信息用作hostname
	serverEntity, err := m.service.GetServer(m.ctx, serverID)
	if err != nil {
		m.logger.Warn("获取服务器信息失败，使用服务器ID作为hostname", zap.Error(err))
	}

	hostname := serverID
	if serverEntity != nil {
		hostname = fmt.Sprintf("%s(%s)", serverEntity.Name, serverEntity.Host)
	}

	metrics := collector.NewExternalServerMetrics(hostname, serverEntity.Host, time.Now())

	// 解析每个命令的结果
	for _, cmdResult := range result.TryResults {
		if cmdResult.Status != server.CommandStatusSuccess {
			m.logger.Warn("监控命令执行失败",
				zap.String("command", cmdResult.CommandName),
				zap.String("error", cmdResult.Error))
			continue
		}

		output := strings.TrimSpace(cmdResult.Stdout)
		if output == "" {
			continue
		}

		switch cmdResult.CommandName {
		case "CPU使用率":
			if value, err := parseFloat(output); err == nil {
				metrics.SetMetric(collector.MetricCPUUsage, value)
			}

		case "CPU负载":
			loads := strings.Fields(strings.ReplaceAll(output, ",", ""))
			if len(loads) >= 3 {
				if load1, err := parseFloat(loads[0]); err == nil {
					metrics.SetMetric(collector.MetricCPULoad1, load1)
				}
				if load5, err := parseFloat(loads[1]); err == nil {
					metrics.SetMetric(collector.MetricCPULoad5, load5)
				}
				if load15, err := parseFloat(loads[2]); err == nil {
					metrics.SetMetric(collector.MetricCPULoad15, load15)
				}
			}

		case "内存信息":
			// 格式: used_percent,total_mb,used_mb,free_mb
			parts := strings.Split(output, ",")
			if len(parts) >= 4 {
				if usedPercent, err := parseFloat(parts[0]); err == nil {
					metrics.SetMetric(collector.MetricMemoryUsedPercent, usedPercent)
				}
				if totalMB, err := parseFloat(parts[1]); err == nil {
					metrics.SetMetric(collector.MetricMemoryTotal, totalMB)
				}
				if usedMB, err := parseFloat(parts[2]); err == nil {
					metrics.SetMetric(collector.MetricMemoryUsed, usedMB)
				}
				if freeMB, err := parseFloat(parts[3]); err == nil {
					metrics.SetMetric(collector.MetricMemoryFree, freeMB)
				}
			}

		case "磁盘使用率":
			if value, err := parseFloat(output); err == nil {
				metrics.SetMetric(collector.MetricDiskUsedPercent, value)
			}

		case "磁盘详细信息":
			// 格式: total_gb,used_gb,free_gb
			parts := strings.Split(output, ",")
			if len(parts) >= 3 {
				if totalGB, err := parseFloat(parts[0]); err == nil {
					metrics.SetMetric(collector.MetricDiskTotal, totalGB)
				}
				if usedGB, err := parseFloat(parts[1]); err == nil {
					metrics.SetMetric(collector.MetricDiskUsed, usedGB)
				}
				if freeGB, err := parseFloat(parts[2]); err == nil {
					metrics.SetMetric(collector.MetricDiskFree, freeGB)
				}
			}

		case "网络接口统计":
			// 格式: rx_bytes,tx_bytes,rx_packets,tx_packets
			parts := strings.Split(output, ",")
			if len(parts) >= 4 {
				if rxBytes, err := parseFloat(parts[0]); err == nil {
					metrics.SetMetric(collector.MetricNetworkIn, rxBytes)
				}
				if txBytes, err := parseFloat(parts[1]); err == nil {
					metrics.SetMetric(collector.MetricNetworkOut, txBytes)
				}
				if rxPackets, err := parseFloat(parts[2]); err == nil {
					metrics.SetMetric(collector.MetricNetworkInPackets, rxPackets)
				}
				if txPackets, err := parseFloat(parts[3]); err == nil {
					metrics.SetMetric(collector.MetricNetworkOutPackets, txPackets)
				}
			}
		}
	}

	return metrics, nil
}

// getMyAssignments 获取分配给当前节点的监控任务
func (m *MonitorManager) getMyAssignments() ([]MonitorAssignment, error) {
	prefix := "/monitor/assignments/"
	kvs, err := m.etcdClient.GetWithPrefix(m.ctx, prefix)
	if err != nil {
		return nil, fmt.Errorf("从etcd获取监控分配失败: %w", err)
	}

	var assignments []MonitorAssignment
	for _, value := range kvs {
		var assignment MonitorAssignment
		if err := json.Unmarshal([]byte(value), &assignment); err != nil {
			m.logger.Error("解析监控分配数据失败", zap.Error(err))
			continue
		}

		// 只返回分配给当前节点的任务
		if assignment.AssignedNode == m.nodeID {
			assignments = append(assignments, assignment)
		}
	}

	return assignments, nil
}

// selectLeastLoadedNode 选择负载最小的节点
func (m *MonitorManager) selectLeastLoadedNode(services []*registry.ServiceInstance) *registry.ServiceInstance {
	if len(services) == 0 {
		return nil
	}

	// 统计每个节点的监控任务数量
	nodeCounts := make(map[string]int)
	prefix := "/monitor/assignments/"
	kvs, err := m.etcdClient.GetWithPrefix(m.ctx, prefix)
	if err != nil {
		m.logger.Error("获取现有监控分配失败", zap.Error(err))
		// 如果获取失败，返回第一个节点
		return services[0]
	}

	// 统计每个节点的分配数量
	for _, value := range kvs {
		var assignment MonitorAssignment
		if err := json.Unmarshal([]byte(value), &assignment); err == nil {
			nodeCounts[assignment.AssignedNode]++
		}
	}

	// 找到负载最小的节点
	var selectedNode *registry.ServiceInstance
	minCount := int(^uint(0) >> 1) // 最大int值

	for _, service := range services {
		if service.Env != registry.EnvAll {
			continue
		}
		count := nodeCounts[service.ID]
		if count < minCount {
			minCount = count
			selectedNode = service
		}
	}

	m.logger.Debug("选择监控节点",
		zap.String("selected_node", selectedNode.ID),
		zap.Int("current_load", minCount))

	return selectedNode
}

// getAssignmentKey 获取监控分配的etcd键
func (m *MonitorManager) getAssignmentKey(serverID string) string {
	return path.Join("/monitor/assignments", serverID)
}

// GetMonitorNodeIP 根据服务器ID获取负责监控该服务器的节点IP
func (m *MonitorManager) GetMonitorNodeIP(serverID string) (string, string, error) {
	// 首先获取服务器详情
	server, err := m.service.GetServer(m.ctx, serverID)
	if err != nil {
		return "", "", fmt.Errorf("获取服务器详情失败: %w", err)
	}

	// 如果服务器安装了AIO，直接通过服务注册中心查找节点信息
	if server.InstallAIO {
		m.logger.Debug("服务器已安装AIO，直接查找服务注册信息", zap.String("server_id", serverID))

		// 通过服务注册中心查找节点信息
		services, err := m.registry.Discover(m.ctx, aioServiceName)
		if err != nil {
			return "", "", fmt.Errorf("发现AIO服务失败: %w", err)
		}

		// 根据服务器的Host查找对应的AIO服务节点
		for _, service := range services {
			// 检查服务地址是否匹配服务器Host
			serviceIP := strings.Split(service.Address, ":")[0]
			if serviceIP == server.Host {
				return m.getIpAndPort(serverID, service, server)
			}
		}

		return "", "", fmt.Errorf("未找到服务器 %s 对应的AIO服务节点", server.Host)
	}

	// 服务器未安装AIO，通过监控分配查找
	m.logger.Debug("服务器未安装AIO，查找监控分配", zap.String("server_id", serverID))

	assignmentKey := m.getAssignmentKey(serverID)
	assignmentData, err := m.etcdClient.Get(m.ctx, assignmentKey)
	if err != nil {
		return "", "", fmt.Errorf("获取监控分配信息失败: %w", err)
	}

	if assignmentData == "" {
		return "", "", fmt.Errorf("服务器 %s 没有监控分配", serverID)
	}

	// 解析监控分配信息
	var assignment MonitorAssignment
	if err := json.Unmarshal([]byte(assignmentData), &assignment); err != nil {
		return "", "", fmt.Errorf("解析监控分配信息失败: %w", err)
	}

	// 通过服务注册中心查找节点信息
	services, err := m.registry.Discover(m.ctx, aioServiceName)
	if err != nil {
		return "", "", fmt.Errorf("发现AIO服务失败: %w", err)
	}

	// 查找分配的节点
	for _, service := range services {
		if service.ID == assignment.AssignedNode {
			return m.getIpAndPort(serverID, service, server)
		}
	}

	return "", "", fmt.Errorf("未找到分配的监控节点 %s", assignment.AssignedNode)
}

func (m *MonitorManager) getIpAndPort(serverID string, service *registry.ServiceInstance, server *server.Server) (string, string, error) {
	m.logger.Debug("找到AIO服务节点",
		zap.String("server_id", serverID),
		zap.String("server_host", server.Host),
		zap.String("service_address", service.Address))

	ip := strings.Split(service.Address, ":")[0]
	port := "9999"
	for key, value := range service.Metadata {
		if key == "http_port" {
			port = value
			break
		}
	}
	return ip, port, nil
}

// parseFloat 安全地解析浮点数
func parseFloat(s string) (float64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("空字符串")
	}
	return strconv.ParseFloat(s, 64)
}

// GetMonitorAssignment 根据serverID查询分配的节点信息
func (m *MonitorManager) GetMonitorAssignment(serverID string) (*MonitorAssignment, error) {
	assignmentKey := m.getAssignmentKey(serverID)
	assignmentData, err := m.etcdClient.Get(m.ctx, assignmentKey)
	if err != nil {
		return nil, fmt.Errorf("获取监控分配信息失败: %w", err)
	}

	if assignmentData == "" {
		return nil, fmt.Errorf("服务器 %s 没有监控分配", serverID)
	}

	// 解析监控分配信息
	var assignment MonitorAssignment
	if err := json.Unmarshal([]byte(assignmentData), &assignment); err != nil {
		return nil, fmt.Errorf("解析监控分配信息失败: %w", err)
	}

	return &assignment, nil
}

// ReassignMonitorNode 重新分配监控节点
func (m *MonitorManager) ReassignMonitorNode(serverID, nodeID string) error {
	// 获取服务器信息
	server, err := m.service.GetServer(m.ctx, serverID)
	if err != nil {
		return fmt.Errorf("获取服务器信息失败: %w", err)
	}

	// 验证nodeID是否有效
	services, err := m.registry.Discover(m.ctx, aioServiceName)
	if err != nil {
		return fmt.Errorf("发现AIO服务失败: %w", err)
	}

	var targetNode *registry.ServiceInstance
	for _, service := range services {
		if service.ID == nodeID {
			targetNode = service
			break
		}
	}

	if targetNode == nil {
		return fmt.Errorf("节点 %s 不存在或不可用", nodeID)
	}

	// 创建新的监控分配记录
	assignment := MonitorAssignment{
		ServerID:     serverID,
		ServerName:   server.Name,
		AssignedNode: nodeID,
		AssignTime:   time.Now(),
	}

	// 保存到etcd
	assignmentKey := m.getAssignmentKey(serverID)
	assignmentData, err := json.Marshal(assignment)
	if err != nil {
		return fmt.Errorf("序列化监控分配失败: %w", err)
	}

	if err := m.etcdClient.Put(m.ctx, assignmentKey, string(assignmentData)); err != nil {
		return fmt.Errorf("保存监控分配到etcd失败: %w", err)
	}

	m.logger.Info("重新分配服务器监控节点成功",
		zap.String("server_id", serverID),
		zap.String("server_name", server.Name),
		zap.String("new_assigned_node", nodeID))

	return nil
}

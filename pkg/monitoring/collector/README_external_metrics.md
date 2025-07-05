# 外部服务器指标采集

本文档说明如何向监控系统推送外部服务器的指标数据，以及如何通过API查询这些数据。

## 功能概述

监控系统现在支持接收来自外部服务器的指标数据，每个服务器通过其IP地址和主机名进行唯一标识。这使得可以在一个中央监控系统中管理多台服务器的指标。

## 服务器指标结构

### ServerMetrics 结构体

```go
type ServerMetrics struct {
    Timestamp time.Time                    `json:"timestamp"`
    Hostname  string                       `json:"hostname"`
    IP        string                       `json:"ip"`
    Metrics   map[ServerMetricName]float64 `json:"metrics"`
}
```

### 支持的指标类型

#### CPU相关指标
- `cpu.usage` - CPU总使用率（百分比）
- `cpu.usage.user` - 用户态CPU使用率
- `cpu.usage.system` - 系统态CPU使用率
- `cpu.usage.idle` - CPU空闲率
- `cpu.usage.iowait` - IO等待时间百分比
- `cpu.load1` - 1分钟负载平均值
- `cpu.load5` - 5分钟负载平均值
- `cpu.load15` - 15分钟负载平均值

#### 内存相关指标
- `memory.total` - 总内存（MB）
- `memory.used` - 已使用内存（MB）
- `memory.free` - 空闲内存（MB）
- `memory.buffers` - 缓冲区内存（MB）
- `memory.cache` - 缓存内存（MB）
- `memory.used_percent` - 内存使用率（百分比）

#### 磁盘相关指标
- `disk.total` - 总磁盘空间（GB）
- `disk.used` - 已使用磁盘空间（GB）
- `disk.free` - 空闲磁盘空间（GB）
- `disk.used_percent` - 磁盘使用率（百分比）
- `disk.io.read` - 磁盘读取次数
- `disk.io.write` - 磁盘写入次数
- `disk.io.read_bytes` - 磁盘读取字节数（MB）
- `disk.io.write_bytes` - 磁盘写入字节数（MB）

#### 网络相关指标
- `network.in` - 网络入流量（字节）
- `network.out` - 网络出流量（字节）
- `network.in_packets` - 网络入包数
- `network.out_packets` - 网络出包数

## API接口

### 1. 单个指标推送

**接口**: `POST /monitoring/server/metrics`

**请求体示例**:
```json
{
    "timestamp": "2024-01-15T10:30:00Z",
    "hostname": "web-server-01",
    "ip": "192.168.1.100",
    "metrics": {
        "cpu.usage": 75.5,
        "memory.used_percent": 68.2,
        "disk.used_percent": 45.8,
        "network.in": 1048576,
        "network.out": 2097152
    }
}
```

**响应示例**:
```json
{
    "success": true,
    "data": {
        "message": "指标数据接收成功",
        "hostname": "web-server-01",
        "ip": "192.168.1.100",
        "timestamp": "2024-01-15T10:30:00Z",
        "metrics_count": 5
    }
}
```

### 2. 批量指标推送

**接口**: `POST /monitoring/server/metrics/batch`

**请求体示例**:
```json
[
    {
        "timestamp": "2024-01-15T10:30:00Z",
        "hostname": "web-server-01",
        "ip": "192.168.1.100",
        "metrics": {
            "cpu.usage": 75.5,
            "memory.used_percent": 68.2
        }
    },
    {
        "timestamp": "2024-01-15T10:30:00Z",
        "hostname": "db-server-01",
        "ip": "192.168.1.101",
        "metrics": {
            "cpu.usage": 45.2,
            "memory.used_percent": 82.1
        }
    }
]
```

### 3. 获取服务器列表

**接口**: `GET /monitoring/servers`

**查询参数**:
- `timeRange`: 时间范围（默认24h）

**响应示例**:
```json
{
    "success": true,
    "data": [
        {
            "hostname": "web-server-01",
            "ip": "192.168.1.100",
            "last_seen": "2024-01-15T10:30:00Z"
        },
        {
            "hostname": "db-server-01",
            "ip": "192.168.1.101",
            "last_seen": "2024-01-15T10:25:00Z"
        }
    ]
}
```

### 4. 按IP查询指标

**系统概览**: `GET /monitoring/system/overview?ip=192.168.1.100`

**指标查询**: `GET /monitoring/metrics/cpu.usage?ip=192.168.1.100&start=1642248000&end=1642251600`

## 使用示例

### Go客户端示例

```go
package main

import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
    "time"
    
    "github.com/xsxdot/aio/pkg/monitoring/collector"
)

func main() {
    // 创建服务器指标
    metrics := collector.NewExternalServerMetrics("web-server-01", "192.168.1.100", time.Now())
    
    // 设置指标值
    metrics.SetMetric(collector.MetricCPUUsage, 75.5)
    metrics.SetMetric(collector.MetricMemoryUsedPercent, 68.2)
    metrics.SetMetric(collector.MetricDiskUsedPercent, 45.8)
    
    // 发送到监控系统
    if err := sendMetrics("http://monitoring-server:8080", metrics); err != nil {
        fmt.Printf("发送指标失败: %v\n", err)
        return
    }
    
    fmt.Println("指标发送成功")
}

func sendMetrics(baseURL string, metrics *collector.ServerMetrics) error {
    data, err := json.Marshal(metrics)
    if err != nil {
        return err
    }
    
    resp, err := http.Post(
        baseURL+"/monitoring/server/metrics",
        "application/json",
        bytes.NewBuffer(data),
    )
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("HTTP错误: %d", resp.StatusCode)
    }
    
    return nil
}
```

### Python客户端示例

```python
import requests
import json
from datetime import datetime
import time

def send_server_metrics(base_url, hostname, ip, metrics_data):
    """发送服务器指标到监控系统"""
    
    payload = {
        "timestamp": datetime.utcnow().isoformat() + "Z",
        "hostname": hostname,
        "ip": ip,
        "metrics": metrics_data
    }
    
    try:
        response = requests.post(
            f"{base_url}/monitoring/server/metrics",
            json=payload,
            headers={"Content-Type": "application/json"}
        )
        response.raise_for_status()
        return response.json()
    except requests.exceptions.RequestException as e:
        print(f"发送指标失败: {e}")
        return None

# 使用示例
if __name__ == "__main__":
    base_url = "http://monitoring-server:8080"
    hostname = "web-server-01"
    ip = "192.168.1.100"
    
    # 模拟指标数据
    metrics = {
        "cpu.usage": 75.5,
        "memory.used_percent": 68.2,
        "disk.used_percent": 45.8,
        "network.in": 1048576,
        "network.out": 2097152
    }
    
    result = send_server_metrics(base_url, hostname, ip, metrics)
    if result:
        print("指标发送成功:", result)
```

### Shell脚本示例

```bash
#!/bin/bash

# 配置
MONITORING_URL="http://monitoring-server:8080"
HOSTNAME=$(hostname)
IP=$(ip route get 8.8.8.8 | awk 'NR==1 {print $7}')

# 获取CPU使用率
CPU_USAGE=$(top -bn1 | grep "Cpu(s)" | awk '{print $2}' | sed 's/%us,//')

# 获取内存使用率
MEMORY_USAGE=$(free | grep Mem | awk '{printf("%.1f", $3/$2 * 100.0)}')

# 获取磁盘使用率
DISK_USAGE=$(df / | tail -1 | awk '{print $5}' | sed 's/%//')

# 构建JSON数据
JSON_DATA=$(cat <<EOF
{
    "timestamp": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
    "hostname": "$HOSTNAME",
    "ip": "$IP",
    "metrics": {
        "cpu.usage": $CPU_USAGE,
        "memory.used_percent": $MEMORY_USAGE,
        "disk.used_percent": $DISK_USAGE
    }
}
EOF
)

# 发送数据
curl -X POST \
    -H "Content-Type: application/json" \
    -d "$JSON_DATA" \
    "$MONITORING_URL/monitoring/server/metrics"

echo "指标已发送"
```

## 定时采集建议

建议设置定时任务（如cron）定期发送指标数据：

```bash
# 每分钟采集一次指标
* * * * * /path/to/send_metrics.sh

# 每5分钟采集一次指标
*/5 * * * * /path/to/send_metrics.sh
```

## 错误处理

### 常见错误代码

- `400`: 请求格式错误，检查JSON格式和必要字段
- `500`: 服务器内部错误，检查监控系统状态

### 重试机制

建议在客户端实现重试机制：

```go
func sendMetricsWithRetry(url string, metrics *collector.ServerMetrics, maxRetries int) error {
    for i := 0; i < maxRetries; i++ {
        if err := sendMetrics(url, metrics); err == nil {
            return nil
        }
        time.Sleep(time.Duration(i+1) * time.Second)
    }
    return fmt.Errorf("发送失败，已重试%d次", maxRetries)
}
```

## 注意事项

1. **时间戳**: 如果不提供timestamp字段，系统会使用接收时间
2. **IP地址**: IP地址用作服务器的唯一标识，确保准确性
3. **指标单位**: 注意各指标的单位要求（如内存使用MB，磁盘使用GB）
4. **网络**: 确保客户端能够访问监控系统的API接口
5. **认证**: 根据系统配置可能需要提供认证信息 
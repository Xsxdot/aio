package bootstrap

import (
	"net"
	"strings"
)

// NetworkType 网络类型常量
const (
	NetworkLocal    = "local"
	NetworkInternal = "internal"
	NetworkTailscale = "tailscale"
)

// DetectedEndpoint 检测到的端点信息
type DetectedEndpoint struct {
	Host     string // 主机地址
	Network  string // 网络类型
	Priority int    // 优先级
}

// IPDetector IP 自动检测器
type IPDetector struct{}

// NewIPDetector 创建 IP 检测器
func NewIPDetector() *IPDetector {
	return &IPDetector{}
}

// DetectEndpoints 根据配置的网络类型列表自动检测端点
// networks: 要检测的网络类型列表，如 ["local", "internal", "tailscale"]
func (d *IPDetector) DetectEndpoints(networks []string) []DetectedEndpoint {
	result := make([]DetectedEndpoint, 0, len(networks))

	for i, network := range networks {
		network = strings.ToLower(strings.TrimSpace(network))
		host := d.detectIP(network)
		if host != "" {
			result = append(result, DetectedEndpoint{
				Host:     host,
				Network:  network,
				Priority: i + 1, // 按配置顺序分配优先级
			})
		}
	}

	return result
}

// detectIP 检测指定网络类型的 IP
func (d *IPDetector) detectIP(network string) string {
	switch network {
	case NetworkLocal:
		return "127.0.0.1"
	case NetworkInternal:
		return d.detectInternalIP()
	case NetworkTailscale:
		return d.detectTailscaleIP()
	default:
		return ""
	}
}

// detectInternalIP 检测内网 IP
// 支持：192.168.x.x, 10.x.x.x, 172.16.x.x - 172.31.x.x
func (d *IPDetector) detectInternalIP() string {
	interfaces, err := net.Interfaces()
	if err != nil {
		return ""
	}

	for _, iface := range interfaces {
		// 跳过回环接口和未启用的接口
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			// 解析 IP 地址
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			// 只处理 IPv4
			if ip == nil || ip.IsLoopback() || ip.To4() == nil {
				continue
			}

			// 检查是否是内网 IP
			if d.isInternalIP(ip) {
				return ip.String()
			}
		}
	}

	return ""
}

// detectTailscaleIP 检测 Tailscale IP
// Tailscale IP 范围：100.64.0.0/10 (100.64.0.0 - 100.127.255.255)
func (d *IPDetector) detectTailscaleIP() string {
	interfaces, err := net.Interfaces()
	if err != nil {
		return ""
	}

	for _, iface := range interfaces {
		// 跳过回环接口和未启用的接口
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}

		// Tailscale 通常使用名为 "tailscale" 或 "Tailscale" 的接口
		// 但也可以检测 IP 范围
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			// 只处理 IPv4
			if ip == nil || ip.IsLoopback() || ip.To4() == nil {
				continue
			}

			// 检查是否是 Tailscale IP（100.64.0.0/10）
			if d.isTailscaleIP(ip) {
				return ip.String()
			}
		}
	}

	return ""
}

// isInternalIP 检查是否是内网 IP
// 内网范围：
// - 10.0.0.0/8 (10.x.x.x)
// - 172.16.0.0/12 (172.16.x.x - 172.31.x.x)
// - 192.168.0.0/16 (192.168.x.x)
func (d *IPDetector) isInternalIP(ip net.IP) bool {
	// 10.0.0.0/8
	if ip[0] == 10 {
		return true
	}

	// 172.16.0.0/12 (172.16 - 172.31)
	if ip[0] == 172 && ip[1] >= 16 && ip[1] <= 31 {
		return true
	}

	// 192.168.0.0/16
	if ip[0] == 192 && ip[1] == 168 {
		return true
	}

	return false
}

// isTailscaleIP 检查是否是 Tailscale IP
// Tailscale 范围：100.64.0.0/10 (100.64.0.0 - 100.127.255.255)
func (d *IPDetector) isTailscaleIP(ip net.IP) bool {
	// 100.64.0.0/10: 第一字节 100，第二字节 64-127
	if ip[0] == 100 && ip[1] >= 64 && ip[1] <= 127 {
		return true
	}
	return false
}
package sdk

import (
	"strings"

	"google.golang.org/grpc/resolver"
)

// init 注册 static resolver（用于 registry 集群地址解析）
func init() {
	resolver.Register(&staticResolverBuilder{})
}

// staticResolverBuilder 实现 resolver.Builder，用于解析 static:/// scheme
type staticResolverBuilder struct{}

// Scheme 返回 resolver scheme
func (*staticResolverBuilder) Scheme() string {
	return "static"
}

// Build 构建 resolver 实例
func (*staticResolverBuilder) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOptions) (resolver.Resolver, error) {
	// target.Endpoint() 格式：host1:port1,host2:port2,...
	addrs := parseRegistryAddrs(target.Endpoint())

	// 将地址转为 resolver.Address
	resolverAddrs := make([]resolver.Address, 0, len(addrs))
	for _, addr := range addrs {
		resolverAddrs = append(resolverAddrs, resolver.Address{
			Addr: addr,
		})
	}

	// 更新 ClientConn 状态
	err := cc.UpdateState(resolver.State{
		Addresses: resolverAddrs,
	})
	if err != nil {
		return nil, err
	}

	// 返回 noop resolver（地址是静态的，不需要动态更新）
	return &staticResolver{}, nil
}

// staticResolver 静态地址 resolver（不会动态更新）
type staticResolver struct{}

// ResolveNow 不需要实现（静态地址）
func (*staticResolver) ResolveNow(resolver.ResolveNowOptions) {}

// Close 关闭 resolver
func (*staticResolver) Close() {}

// parseRegistryAddrs 解析逗号分隔的地址列表
// 示例：
//   "host:port" -> ["host:port"]
//   "host1:port1,host2:port2" -> ["host1:port1", "host2:port2"]
//   "host1:port1, host2:port2 , " -> ["host1:port1", "host2:port2"]
func parseRegistryAddrs(addrsStr string) []string {
	// 按逗号分割
	parts := strings.Split(addrsStr, ",")

	addrs := make([]string, 0, len(parts))
	for _, part := range parts {
		// 去除空格
		addr := strings.TrimSpace(part)
		// 跳过空字符串
		if addr == "" {
			continue
		}
		addrs = append(addrs, addr)
	}

	return addrs
}

// buildRegistryDialTarget 根据 RegistryAddr 构建 gRPC Dial target
// - 空地址：返回空字符串
// - 单地址：直接返回地址（向后兼容）
// - 多地址：返回 static:///addr1,addr2 格式
func buildRegistryDialTarget(registryAddr string) string {
	addrs := parseRegistryAddrs(registryAddr)

	// 空地址列表：返回空字符串
	if len(addrs) == 0 {
		return ""
	}

	// 单地址：直接返回（向后兼容）
	if len(addrs) == 1 {
		return addrs[0]
	}

	// 多地址：使用 static scheme
	return "static:///" + strings.Join(addrs, ",")
}


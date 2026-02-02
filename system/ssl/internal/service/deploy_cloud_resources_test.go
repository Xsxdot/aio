package service

import (
	"testing"

	cas "github.com/alibabacloud-go/cas-20200407/v2/client"
	"github.com/alibabacloud-go/tea/tea"
	"github.com/xsxdot/aio/pkg/core/logger"
)

// TestResourceDomainMatchesTarget 测试资源域名匹配（支持通配符）
func TestResourceDomainMatchesTarget(t *testing.T) {
	s := &DeployService{}

	tests := []struct {
		name           string
		resourceDomain string
		searchDomain   string
		want           bool
	}{
		{
			name:           "精确匹配 - 相同域名",
			resourceDomain: "api.shanzilai.com",
			searchDomain:   "api.shanzilai.com",
			want:           true,
		},
		{
			name:           "精确匹配 - 不同域名",
			resourceDomain: "cdn.shanzilai.com",
			searchDomain:   "api.shanzilai.com",
			want:           false,
		},
		{
			name:           "通配符匹配 - 单层子域名匹配",
			resourceDomain: "api.shanzilai.com",
			searchDomain:   "*.shanzilai.com",
			want:           true,
		},
		{
			name:           "通配符匹配 - 另一个单层子域名匹配",
			resourceDomain: "hzc-api.shanzilai.com",
			searchDomain:   "*.shanzilai.com",
			want:           true,
		},
		{
			name:           "通配符不匹配 - 多层子域名",
			resourceDomain: "api.oss.shanzilai.com",
			searchDomain:   "*.shanzilai.com",
			want:           false,
		},
		{
			name:           "通配符不匹配 - 根域名",
			resourceDomain: "shanzilai.com",
			searchDomain:   "*.shanzilai.com",
			want:           false,
		},
		{
			name:           "通配符不匹配 - 不同基础域名",
			resourceDomain: "api.example.com",
			searchDomain:   "*.shanzilai.com",
			want:           false,
		},
		{
			name:           "精确域名搜索 - 不匹配通配符资源",
			resourceDomain: "cdn.shanzilai.com",
			searchDomain:   "*.shanzilai.com",
			want:           true,
		},
		{
			name:           "空域名",
			resourceDomain: "",
			searchDomain:   "*.shanzilai.com",
			want:           false,
		},
		{
			name:           "单字符子域名",
			resourceDomain: "a.shanzilai.com",
			searchDomain:   "*.shanzilai.com",
			want:           true,
		},
		{
			name:           "包含连字符的子域名",
			resourceDomain: "api-test.shanzilai.com",
			searchDomain:   "*.shanzilai.com",
			want:           true,
		},
		{
			name:           "包含数字的子域名",
			resourceDomain: "api2.shanzilai.com",
			searchDomain:   "*.shanzilai.com",
			want:           true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.resourceDomainMatchesTarget(tt.resourceDomain, tt.searchDomain)
			if got != tt.want {
				t.Errorf("resourceDomainMatchesTarget(%q, %q) = %v, want %v",
					tt.resourceDomain, tt.searchDomain, got, tt.want)
			}
		})
	}
}

// TestGroupCloudResourcesByProduct 测试云资源分组
func TestGroupCloudResourcesByProduct(t *testing.T) {
	// 创建带有 logger 的 DeployService 实例
	log := logger.GetLogger().WithEntryName("DeployServiceTest")
	s := &DeployService{log: log}

	// 模拟云资源数据（来自 ListCloudResources 的响应）
	allResources := []*cas.ListCloudResourcesResponseBodyData{
		// CDN 资源 - 匹配
		{
			Id:           tea.Int64(1001),
			Domain:       tea.String("api.shanzilai.com"),
			CloudProduct: tea.String("CDN"),
		},
		// DCDN 资源 - 匹配
		{
			Id:           tea.Int64(1002),
			Domain:       tea.String("hzc-api.shanzilai.com"),
			CloudProduct: tea.String("DCDN"),
		},
		// OSS 资源 - 匹配，有完整信息
		{
			Id:           tea.Int64(1003),
			Domain:       tea.String("cdn.shanzilai.com"),
			CloudProduct: tea.String("OSS"),
			InstanceId:   tea.String("aio-workflow"),
			RegionId:     tea.String("cn-qingdao"),
		},
		// OSS 资源 - 匹配，但缺失 RegionId
		{
			Id:           tea.Int64(1004),
			Domain:       tea.String("oss.shanzilai.com"),
			CloudProduct: tea.String("OSS"),
			InstanceId:   tea.String("acan1"),
			RegionId:     tea.String(""), // 缺失 RegionId
		},
		// CDN 资源 - 不匹配（多层子域名）
		{
			Id:           tea.Int64(1005),
			Domain:       tea.String("api.sub.shanzilai.com"),
			CloudProduct: tea.String("CDN"),
		},
		// CDN 资源 - 不匹配（根域名）
		{
			Id:           tea.Int64(1006),
			Domain:       tea.String("shanzilai.com"),
			CloudProduct: tea.String("CDN"),
		},
		// ECS 资源 - 不匹配（非目标云产品）
		{
			Id:           tea.Int64(1007),
			Domain:       tea.String("api.shanzilai.com"),
			CloudProduct: tea.String("ECS"),
		},
		// DCDN 资源 - 不匹配（不同域名）
		{
			Id:           tea.Int64(1008),
			Domain:       tea.String("api.example.com"),
			CloudProduct: tea.String("DCDN"),
		},
	}

	tests := []struct {
		name         string
		searchDomain string
		wantCDN      int
		wantDCDN     int
		wantOSS      int
	}{
		{
			name:         "通配符域名匹配",
			searchDomain: "*.shanzilai.com",
			wantCDN:      1, // api.shanzilai.com
			wantDCDN:     1, // hzc-api.shanzilai.com
			wantOSS:      2, // cdn.shanzilai.com, oss.shanzilai.com (包括缺失 RegionId 的)
		},
		{
			name:         "精确域名匹配",
			searchDomain: "cdn.shanzilai.com",
			wantCDN:      0,
			wantDCDN:     0,
			wantOSS:      1, // cdn.shanzilai.com
		},
		{
			name:         "无匹配的域名",
			searchDomain: "nonexistent.com",
			wantCDN:      0,
			wantDCDN:     0,
			wantOSS:      0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			group := s.groupCloudResourcesByProduct(allResources, tt.searchDomain)

			if len(group.CDN) != tt.wantCDN {
				t.Errorf("CDN count = %d, want %d", len(group.CDN), tt.wantCDN)
			}

			if len(group.DCDN) != tt.wantDCDN {
				t.Errorf("DCDN count = %d, want %d", len(group.DCDN), tt.wantDCDN)
			}

			if len(group.OSS) != tt.wantOSS {
				t.Errorf("OSS count = %d, want %d", len(group.OSS), tt.wantOSS)
			}

			// 验证 OSS 资源包含 RegionId（即使为空字符串）
			if tt.name == "通配符域名匹配" && len(group.OSS) > 0 {
				hasEmptyRegion := false
				hasValidRegion := false
				for _, item := range group.OSS {
					if item.RegionID == "" {
						hasEmptyRegion = true
					} else {
						hasValidRegion = true
					}
				}
				if !hasEmptyRegion || !hasValidRegion {
					t.Error("Expected OSS group to contain both resources with and without RegionId")
				}
			}
		})
	}
}

// TestGroupCloudResourcesByProduct_EdgeCases 测试边界情况
func TestGroupCloudResourcesByProduct_EdgeCases(t *testing.T) {
	// 创建带有 logger 的 DeployService 实例
	log := logger.GetLogger().WithEntryName("DeployServiceTest")
	s := &DeployService{log: log}

	tests := []struct {
		name         string
		resources    []*cas.ListCloudResourcesResponseBodyData
		searchDomain string
		wantCDN      int
		wantDCDN     int
		wantOSS      int
	}{
		{
			name:         "空资源列表",
			resources:    []*cas.ListCloudResourcesResponseBodyData{},
			searchDomain: "*.shanzilai.com",
			wantCDN:      0,
			wantDCDN:     0,
			wantOSS:      0,
		},
		{
			name: "资源缺失 Domain",
			resources: []*cas.ListCloudResourcesResponseBodyData{
				{
					Id:           tea.Int64(2001),
					Domain:       nil, // 缺失 Domain
					CloudProduct: tea.String("CDN"),
				},
			},
			searchDomain: "*.shanzilai.com",
			wantCDN:      0,
			wantDCDN:     0,
			wantOSS:      0,
		},
		{
			name: "资源缺失 CloudProduct",
			resources: []*cas.ListCloudResourcesResponseBodyData{
				{
					Id:           tea.Int64(2002),
					Domain:       tea.String("api.shanzilai.com"),
					CloudProduct: nil, // 缺失 CloudProduct
				},
			},
			searchDomain: "*.shanzilai.com",
			wantCDN:      0,
			wantDCDN:     0,
			wantOSS:      0,
		},
		{
			name: "OSS 资源缺失 InstanceId",
			resources: []*cas.ListCloudResourcesResponseBodyData{
				{
					Id:           tea.Int64(2003),
					Domain:       tea.String("oss.shanzilai.com"),
					CloudProduct: tea.String("OSS"),
					InstanceId:   nil, // 缺失 InstanceId
					RegionId:     tea.String("cn-qingdao"),
				},
			},
			searchDomain: "*.shanzilai.com",
			wantCDN:      0,
			wantDCDN:     0,
			wantOSS:      1, // 仍然会收集，但 InstanceID 为空字符串
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			group := s.groupCloudResourcesByProduct(tt.resources, tt.searchDomain)

			if len(group.CDN) != tt.wantCDN {
				t.Errorf("CDN count = %d, want %d", len(group.CDN), tt.wantCDN)
			}

			if len(group.DCDN) != tt.wantDCDN {
				t.Errorf("DCDN count = %d, want %d", len(group.DCDN), tt.wantDCDN)
			}

			if len(group.OSS) != tt.wantOSS {
				t.Errorf("OSS count = %d, want %d", len(group.OSS), tt.wantOSS)
			}
		})
	}
}


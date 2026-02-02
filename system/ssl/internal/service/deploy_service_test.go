package service

import (
	"testing"
)

// TestDomainMatchesCert 测试域名匹配逻辑
func TestDomainMatchesCert(t *testing.T) {
	tests := []struct {
		name        string
		domain      string
		certDomains []string
		want        bool
	}{
		{
			name:        "精确匹配 - 相同域名",
			domain:      "oss.shanzilai.com",
			certDomains: []string{"oss.shanzilai.com"},
			want:        true,
		},
		{
			name:        "精确匹配 - 不同域名",
			domain:      "cdn.shanzilai.com",
			certDomains: []string{"oss.shanzilai.com"},
			want:        false,
		},
		{
			name:        "通配符匹配 - 域名匹配通配符",
			domain:      "oss.shanzilai.com",
			certDomains: []string{"*.shanzilai.com"},
			want:        true,
		},
		{
			name:        "通配符匹配 - 另一个子域名匹配通配符",
			domain:      "cdn.shanzilai.com",
			certDomains: []string{"*.shanzilai.com"},
			want:        true,
		},
		{
			name:        "通配符不匹配 - 不同基础域名",
			domain:      "oss.example.com",
			certDomains: []string{"*.shanzilai.com"},
			want:        false,
		},
		{
			name:        "通配符不匹配 - 多层子域名",
			domain:      "api.oss.shanzilai.com",
			certDomains: []string{"*.shanzilai.com"},
			want:        false,
		},
		{
			name:        "通配符不匹配 - 根域名",
			domain:      "shanzilai.com",
			certDomains: []string{"*.shanzilai.com"},
			want:        false,
		},
		{
			name:        "多个证书域名 - 匹配第一个",
			domain:      "oss.shanzilai.com",
			certDomains: []string{"oss.shanzilai.com", "cdn.shanzilai.com"},
			want:        true,
		},
		{
			name:        "多个证书域名 - 匹配第二个",
			domain:      "cdn.shanzilai.com",
			certDomains: []string{"oss.shanzilai.com", "cdn.shanzilai.com"},
			want:        true,
		},
		{
			name:        "多个证书域名 - 都不匹配",
			domain:      "api.shanzilai.com",
			certDomains: []string{"oss.shanzilai.com", "cdn.shanzilai.com"},
			want:        false,
		},
		{
			name:        "混合匹配 - 精确域名在通配符域名列表中",
			domain:      "oss.shanzilai.com",
			certDomains: []string{"*.shanzilai.com", "example.com"},
			want:        true,
		},
		{
			name:        "空证书域名列表",
			domain:      "oss.shanzilai.com",
			certDomains: []string{},
			want:        false,
		},
	}

	// 创建一个 DeployService 实例来调用私有方法
	// 注意：这里我们不需要完整的依赖，因为 domainMatchesCert 不依赖任何外部资源
	s := &DeployService{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.domainMatchesCert(tt.domain, tt.certDomains)
			if got != tt.want {
				t.Errorf("domainMatchesCert(%q, %v) = %v, want %v", 
					tt.domain, tt.certDomains, got, tt.want)
			}
		})
	}
}

// TestDomainMatchesCert_EdgeCases 测试边界情况
func TestDomainMatchesCert_EdgeCases(t *testing.T) {
	s := &DeployService{}

	tests := []struct {
		name        string
		domain      string
		certDomains []string
		want        bool
	}{
		{
			name:        "空域名",
			domain:      "",
			certDomains: []string{"*.shanzilai.com"},
			want:        false,
		},
		{
			name:        "通配符前缀为空的情况",
			domain:      "shanzilai.com",
			certDomains: []string{"*.shanzilai.com"},
			want:        false,
		},
		{
			name:        "通配符匹配 - 精确的通配符域名",
			domain:      "*.shanzilai.com",
			certDomains: []string{"*.shanzilai.com"},
			want:        true,
		},
		{
			name:        "单字符子域名",
			domain:      "a.shanzilai.com",
			certDomains: []string{"*.shanzilai.com"},
			want:        true,
		},
		{
			name:        "包含连字符的子域名",
			domain:      "api-test.shanzilai.com",
			certDomains: []string{"*.shanzilai.com"},
			want:        true,
		},
		{
			name:        "包含数字的子域名",
			domain:      "api2.shanzilai.com",
			certDomains: []string{"*.shanzilai.com"},
			want:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.domainMatchesCert(tt.domain, tt.certDomains)
			if got != tt.want {
				t.Errorf("domainMatchesCert(%q, %v) = %v, want %v",
					tt.domain, tt.certDomains, got, tt.want)
			}
		})
	}
}


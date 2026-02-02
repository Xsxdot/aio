package sdk

import (
	"reflect"
	"testing"
)

// TestParseRegistryAddrs 测试地址解析
func TestParseRegistryAddrs(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "single address",
			input:    "localhost:50051",
			expected: []string{"localhost:50051"},
		},
		{
			name:     "two addresses",
			input:    "host1:50051,host2:50051",
			expected: []string{"host1:50051", "host2:50051"},
		},
		{
			name:     "three addresses",
			input:    "host1:50051,host2:50051,host3:50051",
			expected: []string{"host1:50051", "host2:50051", "host3:50051"},
		},
		{
			name:     "addresses with spaces",
			input:    "host1:50051, host2:50051 , host3:50051",
			expected: []string{"host1:50051", "host2:50051", "host3:50051"},
		},
		{
			name:     "trailing comma",
			input:    "host1:50051,host2:50051,",
			expected: []string{"host1:50051", "host2:50051"},
		},
		{
			name:     "empty parts",
			input:    "host1:50051,,host2:50051",
			expected: []string{"host1:50051", "host2:50051"},
		},
		{
			name:     "empty string",
			input:    "",
			expected: []string{},
		},
		{
			name:     "only commas",
			input:    ",,,",
			expected: []string{},
		},
		{
			name:     "IPv4 addresses",
			input:    "10.0.0.1:50051,10.0.0.2:50051",
			expected: []string{"10.0.0.1:50051", "10.0.0.2:50051"},
		},
		{
			name:     "mixed hostname and IP",
			input:    "registry.example.com:50051,10.0.0.1:50051",
			expected: []string{"registry.example.com:50051", "10.0.0.1:50051"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseRegistryAddrs(tt.input)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("parseRegistryAddrs(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

// TestBuildRegistryDialTarget 测试 dial target 构建
func TestBuildRegistryDialTarget(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "single address - passthrough",
			input:    "localhost:50051",
			expected: "localhost:50051",
		},
		{
			name:     "two addresses - static scheme",
			input:    "host1:50051,host2:50051",
			expected: "static:///host1:50051,host2:50051",
		},
		{
			name:     "three addresses - static scheme",
			input:    "host1:50051,host2:50051,host3:50051",
			expected: "static:///host1:50051,host2:50051,host3:50051",
		},
		{
			name:     "addresses with spaces - normalized",
			input:    "host1:50051, host2:50051 , host3:50051",
			expected: "static:///host1:50051,host2:50051,host3:50051",
		},
		{
			name:     "trailing comma - filtered",
			input:    "host1:50051,host2:50051,",
			expected: "static:///host1:50051,host2:50051",
		},
		{
			name:     "IPv4 addresses",
			input:    "10.0.0.1:50051,10.0.0.2:50051",
			expected: "static:///10.0.0.1:50051,10.0.0.2:50051",
		},
		{
			name:     "empty string - empty result",
			input:    "",
			expected: "",
		},
		{
			name:     "single with spaces - passthrough",
			input:    " localhost:50051 ",
			expected: "localhost:50051",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildRegistryDialTarget(tt.input)
			if result != tt.expected {
				t.Errorf("buildRegistryDialTarget(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestBuildRegistryDialTarget_EmptyHandling 测试空地址处理
func TestBuildRegistryDialTarget_EmptyHandling(t *testing.T) {
	// 空字符串
	result := buildRegistryDialTarget("")
	if result != "" {
		t.Errorf("buildRegistryDialTarget(\"\") = %q, want empty string", result)
	}

	// 只有逗号和空格
	result = buildRegistryDialTarget(", , ,")
	if result != "" {
		t.Errorf("buildRegistryDialTarget(\", , ,\") = %q, want empty string", result)
	}
}

// TestBuildRegistryDialTarget_BackwardCompatibility 测试向后兼容性
func TestBuildRegistryDialTarget_BackwardCompatibility(t *testing.T) {
	// 单地址应该直接返回，不包装成 static:/// scheme
	singleAddresses := []string{
		"localhost:50051",
		"registry.example.com:50051",
		"10.0.0.1:50051",
		"[::1]:50051", // IPv6
	}

	for _, addr := range singleAddresses {
		result := buildRegistryDialTarget(addr)
		if result != addr {
			t.Errorf("buildRegistryDialTarget(%q) = %q, want %q (backward compatibility)", addr, result, addr)
		}
	}
}

// TestParseRegistryAddrs_EdgeCases 测试边界情况
func TestParseRegistryAddrs_EdgeCases(t *testing.T) {
	// 大量空白
	result := parseRegistryAddrs("   host1:50051   ,   host2:50051   ")
	expected := []string{"host1:50051", "host2:50051"}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("parseRegistryAddrs with extra spaces = %v, want %v", result, expected)
	}

	// 连续逗号
	result = parseRegistryAddrs("host1:50051,,,host2:50051")
	expected = []string{"host1:50051", "host2:50051"}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("parseRegistryAddrs with multiple commas = %v, want %v", result, expected)
	}

	// 非常长的地址列表
	longInput := "h1:1,h2:2,h3:3,h4:4,h5:5,h6:6,h7:7,h8:8,h9:9,h10:10"
	result = parseRegistryAddrs(longInput)
	if len(result) != 10 {
		t.Errorf("parseRegistryAddrs long list returned %d addresses, want 10", len(result))
	}
}


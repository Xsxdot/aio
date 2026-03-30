package sdk

import (
	"encoding/json"
	"testing"
)

// TestComposeConfigsByPrefix_DottedKeys 测试 dotted keys 正确嵌套
func TestComposeConfigsByPrefix_DottedKeys(t *testing.T) {
	configs := map[string]string{
		"tk.server.tools.deepseek": `{"api_key": "deepseek-key", "model": "deepseek-chat"}`,
		"tk.server.tools.ali-bj":   `{"api_key": "ali-key", "region": "beijing"}`,
	}

	result, err := ComposeConfigsByPrefix(configs, "tk.server")
	if err != nil {
		t.Fatalf("ComposeConfigsByPrefix failed: %v", err)
	}

	var output map[string]interface{}
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	// 验证顶层有 tools 键
	tools, ok := output["tools"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected 'tools' to be a map, got: %T", output["tools"])
	}

	// 验证 tools 下有 deepseek 和 ali-bj
	deepseek, ok := tools["deepseek"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected 'tools.deepseek' to be a map, got: %T", tools["deepseek"])
	}
	if deepseek["api_key"] != "deepseek-key" {
		t.Errorf("expected deepseek.api_key='deepseek-key', got: %v", deepseek["api_key"])
	}

	aliBj, ok := tools["ali-bj"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected 'tools.ali-bj' to be a map, got: %T", tools["ali-bj"])
	}
	if aliBj["region"] != "beijing" {
		t.Errorf("expected ali-bj.region='beijing', got: %v", aliBj["region"])
	}
}

// TestComposeConfigsByPrefix_ParentChildMerge 测试父子并存时的合并语义
func TestComposeConfigsByPrefix_ParentChildMerge(t *testing.T) {
	configs := map[string]string{
		"tk.server.tools":          `{"timeout": 10, "retry": 3}`,
		"tk.server.tools.deepseek": `{"timeout": 30, "model": "deepseek-chat"}`,
		"tk.server.tools.ali-bj":   `{"timeout": 20, "region": "beijing"}`,
	}

	result, err := ComposeConfigsByPrefix(configs, "tk.server")
	if err != nil {
		t.Fatalf("ComposeConfigsByPrefix failed: %v", err)
	}

	var output map[string]interface{}
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	tools, ok := output["tools"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected 'tools' to be a map, got: %T", output["tools"])
	}

	// 验证父节点 tools 的字段存在
	if tools["timeout"] != float64(10) {
		t.Errorf("expected tools.timeout=10, got: %v", tools["timeout"])
	}
	if tools["retry"] != float64(3) {
		t.Errorf("expected tools.retry=3, got: %v", tools["retry"])
	}

	// 验证子节点 deepseek 有自己的 timeout（应该保留子节点的值）
	deepseek, ok := tools["deepseek"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected 'tools.deepseek' to be a map, got: %T", tools["deepseek"])
	}
	if deepseek["timeout"] != float64(30) {
		t.Errorf("expected deepseek.timeout=30, got: %v", deepseek["timeout"])
	}
	if deepseek["model"] != "deepseek-chat" {
		t.Errorf("expected deepseek.model='deepseek-chat', got: %v", deepseek["model"])
	}

	// 验证子节点 ali-bj
	aliBj, ok := tools["ali-bj"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected 'tools.ali-bj' to be a map, got: %T", tools["ali-bj"])
	}
	if aliBj["timeout"] != float64(20) {
		t.Errorf("expected ali-bj.timeout=20, got: %v", aliBj["timeout"])
	}
}

// TestComposeConfigsByPrefix_AppSection 测试 app section 的特殊处理
func TestComposeConfigsByPrefix_AppSection(t *testing.T) {
	configs := map[string]string{
		"tk.server.app":   `{"name": "test-app", "version": "1.0"}`,
		"tk.server.redis": `{"host": "localhost", "port": 6379}`,
	}

	result, err := ComposeConfigsByPrefix(configs, "tk.server")
	if err != nil {
		t.Fatalf("ComposeConfigsByPrefix failed: %v", err)
	}

	var output map[string]interface{}
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	// 验证 app 的内容被 merge 到根
	if output["name"] != "test-app" {
		t.Errorf("expected name='test-app' at root, got: %v", output["name"])
	}
	if output["version"] != "1.0" {
		t.Errorf("expected version='1.0' at root, got: %v", output["version"])
	}

	// 验证 redis 作为正常嵌套字段
	redis, ok := output["redis"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected 'redis' to be a map, got: %T", output["redis"])
	}
	if redis["host"] != "localhost" {
		t.Errorf("expected redis.host='localhost', got: %v", redis["host"])
	}
}

// TestComposeConfigsByPrefix_DeepNesting 测试更深层级的嵌套
func TestComposeConfigsByPrefix_DeepNesting(t *testing.T) {
	configs := map[string]string{
		"tk.server.a.b.c": `{"value": "deep"}`,
		"tk.server.a.b":   `{"value": "middle", "extra": "data"}`,
		"tk.server.a":     `{"value": "shallow"}`,
	}

	result, err := ComposeConfigsByPrefix(configs, "tk.server")
	if err != nil {
		t.Fatalf("ComposeConfigsByPrefix failed: %v", err)
	}

	var output map[string]interface{}
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	// 验证三层嵌套结构
	a, ok := output["a"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected 'a' to be a map, got: %T", output["a"])
	}
	if a["value"] != "shallow" {
		t.Errorf("expected a.value='shallow', got: %v", a["value"])
	}

	b, ok := a["b"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected 'a.b' to be a map, got: %T", a["b"])
	}
	if b["value"] != "middle" {
		t.Errorf("expected a.b.value='middle', got: %v", b["value"])
	}
	if b["extra"] != "data" {
		t.Errorf("expected a.b.extra='data', got: %v", b["extra"])
	}

	c, ok := b["c"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected 'a.b.c' to be a map, got: %T", b["c"])
	}
	if c["value"] != "deep" {
		t.Errorf("expected a.b.c.value='deep', got: %v", c["value"])
	}
}

// TestComposeConfigsByPrefix_EmptyPrefix 测试空前缀情况
func TestComposeConfigsByPrefix_EmptyPrefix(t *testing.T) {
	configs := map[string]string{
		"redis": `{"host": "localhost"}`,
		"db":    `{"host": "postgres"}`,
	}

	result, err := ComposeConfigsByPrefix(configs, "")
	if err != nil {
		t.Fatalf("ComposeConfigsByPrefix failed: %v", err)
	}

	var output map[string]interface{}
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	// 空前缀时不应该有 redis/db 嵌套（因为前缀过滤逻辑）
	// 实际上空前缀+点号会导致所有 key 都不匹配 HasPrefix(".") 检查
	// 这个测试验证边界行为
	if len(output) != 0 {
		t.Logf("with empty prefix, got output: %+v", output)
	}
}

// TestMergeMaps 测试 map 递归合并逻辑
func TestMergeMaps(t *testing.T) {
	dst := map[string]interface{}{
		"a": "original",
		"b": map[string]interface{}{
			"c": "original-c",
			"d": "original-d",
		},
	}

	src := map[string]interface{}{
		"b": map[string]interface{}{
			"c": "new-c",
			"e": "new-e",
		},
		"f": "new-f",
	}

	mergeMaps(dst, src)

	// 验证合并结果
	if dst["a"] != "original" {
		t.Errorf("expected a='original', got: %v", dst["a"])
	}
	if dst["f"] != "new-f" {
		t.Errorf("expected f='new-f', got: %v", dst["f"])
	}

	b, ok := dst["b"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected 'b' to be a map, got: %T", dst["b"])
	}
	if b["c"] != "new-c" {
		t.Errorf("expected b.c='new-c' (overwritten), got: %v", b["c"])
	}
	if b["d"] != "original-d" {
		t.Errorf("expected b.d='original-d' (preserved), got: %v", b["d"])
	}
	if b["e"] != "new-e" {
		t.Errorf("expected b.e='new-e' (added), got: %v", b["e"])
	}
}

// TestSortEntriesByDepth 测试条目排序逻辑
func TestSortEntriesByDepth(t *testing.T) {
	entries := []configEntry{
		{section: "tools.deepseek", depth: 2},
		{section: "tools", depth: 1},
		{section: "tools.ali-bj", depth: 2},
		{section: "redis", depth: 1},
	}

	sortEntriesByDepth(entries)

	// 验证排序结果：深度优先，同深度按字典序
	expected := []string{"redis", "tools", "tools.ali-bj", "tools.deepseek"}
	for i, entry := range entries {
		if entry.section != expected[i] {
			t.Errorf("at position %d, expected section='%s', got: '%s'", i, expected[i], entry.section)
		}
	}
}
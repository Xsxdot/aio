package common

import (
	"encoding/json"
	"testing"
	"time"
)

func TestFlexTime_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "RFC3339格式",
			input:   `{"time":"2025-11-09T17:44:55+08:00"}`,
			wantErr: false,
		},
		{
			name:    "标准格式带空格",
			input:   `{"time":"2025-11-09 17:44:55"}`,
			wantErr: false,
		},
		{
			name:    "ISO 8601格式",
			input:   `{"time":"2025-11-09T17:44:55"}`,
			wantErr: false,
		},
		{
			name:    "日期格式",
			input:   `{"time":"2025-11-09"}`,
			wantErr: false,
		},
		{
			name:    "null值",
			input:   `{"time":null}`,
			wantErr: false,
		},
		{
			name:    "空字符串",
			input:   `{"time":""}`,
			wantErr: false,
		},
	}

	type testStruct struct {
		Time *FlexTime `json:"time"`
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result testStruct
			err := json.Unmarshal([]byte(tt.input), &result)
			
			if (err != nil) != tt.wantErr {
				t.Errorf("UnmarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			
			if !tt.wantErr && err == nil {
				t.Logf("成功解析时间: %v", result.Time)
			}
		})
	}
}

func TestFlexTime_MarshalJSON(t *testing.T) {
	now := time.Now()
	flexTime := NewFlexTime(now)
	
	type testStruct struct {
		Time *FlexTime `json:"time"`
	}
	
	test := testStruct{Time: flexTime}
	data, err := json.Marshal(test)
	if err != nil {
		t.Errorf("MarshalJSON() error = %v", err)
		return
	}
	
	t.Logf("序列化结果: %s", string(data))
	
	// 测试反序列化
	var result testStruct
	err = json.Unmarshal(data, &result)
	if err != nil {
		t.Errorf("UnmarshalJSON() error = %v", err)
		return
	}
	
	// 比较时间（忽略纳秒差异）
	if !result.Time.Time.Truncate(time.Second).Equal(now.Truncate(time.Second)) {
		t.Errorf("时间不匹配: got %v, want %v", result.Time.Time, now)
	}
}

func TestFlexTime_ZeroValue(t *testing.T) {
	var flexTime FlexTime
	
	if !flexTime.IsZero() {
		t.Error("零值应该返回 true")
	}
	
	data, err := json.Marshal(flexTime)
	if err != nil {
		t.Errorf("MarshalJSON() error = %v", err)
		return
	}
	
	if string(data) != "null" {
		t.Errorf("零值序列化应该是 null，实际是: %s", string(data))
	}
}

func TestFlexTime_RealWorldExample(t *testing.T) {
	// 模拟实际的API请求数据
	requestJSON := `{
		"name": "测试商品",
		"showStartTime": "2025-11-09 17:44:55",
		"showEndTime": "2025-12-31 23:59:59",
		"saleStartTime": null
	}`
	
	type Product struct {
		Name          string    `json:"name"`
		ShowStartTime *FlexTime `json:"showStartTime"`
		ShowEndTime   *FlexTime `json:"showEndTime"`
		SaleStartTime *FlexTime `json:"saleStartTime"`
	}
	
	var product Product
	err := json.Unmarshal([]byte(requestJSON), &product)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	
	if product.Name != "测试商品" {
		t.Errorf("商品名称错误: %s", product.Name)
	}
	
	if product.ShowStartTime == nil {
		t.Error("ShowStartTime 不应该为 nil")
	} else if product.ShowStartTime.IsZero() {
		t.Error("ShowStartTime 不应该是零值")
	} else {
		t.Logf("ShowStartTime: %v", product.ShowStartTime.Time)
	}
	
	if product.ShowEndTime == nil {
		t.Error("ShowEndTime 不应该为 nil")
	} else if product.ShowEndTime.IsZero() {
		t.Error("ShowEndTime 不应该是零值")
	} else {
		t.Logf("ShowEndTime: %v", product.ShowEndTime.Time)
	}
	
	if product.SaleStartTime != nil && !product.SaleStartTime.IsZero() {
		t.Error("SaleStartTime 应该为 nil 或零值")
	}
	
	// 测试序列化回JSON
	responseJSON, err := json.Marshal(product)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	
	t.Logf("响应JSON: %s", string(responseJSON))
}






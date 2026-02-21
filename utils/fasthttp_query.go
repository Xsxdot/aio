package utils

import (
	"fmt"
	"net/url"
	"strings"

	json "github.com/json-iterator/go"
)

// mergeQueryIntoURL 将 query 参数合并到 URL 中
// rawQuery: 原始查询字符串（如 "a=1&b=2"）
// obj: 对象（map[string]any 或 struct）
func mergeQueryIntoURL(urlStr string, rawQuery string, obj interface{}) (string, error) {
	// 解析现有 URL
	u, err := url.Parse(urlStr)
	if err != nil {
		return "", fmt.Errorf("解析URL失败: %w", err)
	}
	
	// 获取现有的 query 参数
	values := u.Query()
	
	// 合并 rawQuery
	if rawQuery != "" {
		rawValues, err := url.ParseQuery(rawQuery)
		if err != nil {
			return "", fmt.Errorf("解析rawQuery失败: %w", err)
		}
		for k, v := range rawValues {
			for _, vv := range v {
				values.Add(k, vv)
			}
		}
	}
	
	// 合并 obj
	if obj != nil {
		objValues, err := objToQueryValues(obj)
		if err != nil {
			return "", err
		}
		for k, v := range objValues {
			for _, vv := range v {
				values.Add(k, vv)
			}
		}
	}
	
	// 设置新的 query
	u.RawQuery = values.Encode()
	
	return u.String(), nil
}

// objToQueryValues 将对象转换为 url.Values
// 支持 map[string]string、map[string]any 和任意 struct
func objToQueryValues(obj interface{}) (url.Values, error) {
	if obj == nil {
		return url.Values{}, nil
	}
	
	values := url.Values{}
	
	// 处理 map[string]string
	if m, ok := obj.(map[string]string); ok {
		for k, v := range m {
			values.Add(k, v)
		}
		return values, nil
	}
	
	// 处理 map[string]interface{}
	if m, ok := obj.(map[string]interface{}); ok {
		for k, v := range m {
			values.Add(k, fmt.Sprint(v))
		}
		return values, nil
	}
	
	// 其他类型通过 JSON marshal/unmarshal 转换
	data, err := json.Marshal(obj)
	if err != nil {
		return nil, fmt.Errorf("序列化对象失败: %w", err)
	}
	
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("反序列化对象失败: %w", err)
	}
	
	for k, v := range m {
		values.Add(k, fmt.Sprint(v))
	}
	
	return values, nil
}

// isGetOrHead 判断是否为 GET 或 HEAD 方法
func isGetOrHead(method string) bool {
	m := strings.ToUpper(method)
	return m == "GET" || m == "HEAD"
}


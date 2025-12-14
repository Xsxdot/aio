package utils

import (
	"encoding/json"
	"math/rand"
	"strconv"

	"go.etcd.io/etcd/pkg/v3/stringutil"
)

func RandomString(length uint) string {
	return stringutil.RandString(length)
}

func RandomInt64(max int) int {
	return rand.Intn(max)
}

// ParseInt64 将字符串转换为int64类型，如果转换失败则返回默认值
func ParseInt64(s string, defaultVal int64) int64 {
	if s == "" {
		return defaultVal
	}

	val, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return defaultVal
	}

	return val
}

// ParseJSON 将 JSON 字符串解析为对象
func ParseJSON(jsonStr string, v interface{}) error {
	return json.Unmarshal([]byte(jsonStr), v)
}

// ToJSON 将对象序列化为 JSON 字符串
func ToJSON(v interface{}) (string, error) {
	bytes, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

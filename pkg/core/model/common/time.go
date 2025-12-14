package common

import (
	"database/sql/driver"
	"fmt"
	"strings"
	"time"
)

// FlexTime 灵活的时间类型，支持多种格式的JSON解析
type FlexTime struct {
	time.Time
}

// 支持的时间格式列表
var timeFormats = []string{
	time.RFC3339,                 // "2006-01-02T15:04:05Z07:00"
	"2006-01-02T15:04:05",        // "2006-01-02T15:04:05"
	"2006-01-02 15:04:05",        // "2006-01-02 15:04:05"
	"2006-01-02T15:04:05.999Z",   // "2006-01-02T15:04:05.999Z"
	"2006-01-02T15:04:05.999",    // "2006-01-02T15:04:05.999"
	"2006-01-02 15:04:05.999",    // "2006-01-02 15:04:05.999"
	"2006-01-02",                 // "2006-01-02"
	time.RFC3339Nano,             // "2006-01-02T15:04:05.999999999Z07:00"
}

// UnmarshalJSON 自定义JSON反序列化，支持多种时间格式
func (t *FlexTime) UnmarshalJSON(data []byte) error {
	// 去除引号
	str := strings.Trim(string(data), "\"")
	
	// 空字符串或null返回零值
	if str == "" || str == "null" {
		t.Time = time.Time{}
		return nil
	}

	// 尝试各种格式
	var parseErr error
	for _, format := range timeFormats {
		parsed, err := time.Parse(format, str)
		if err == nil {
			t.Time = parsed
			return nil
		}
		parseErr = err
	}

	// 所有格式都失败，返回最后一个错误
	return fmt.Errorf("无法解析时间格式: %s, 错误: %v", str, parseErr)
}

// MarshalJSON 自定义JSON序列化，使用RFC3339格式
func (t FlexTime) MarshalJSON() ([]byte, error) {
	if t.Time.IsZero() {
		return []byte("null"), nil
	}
	return []byte(fmt.Sprintf("\"%s\"", t.Time.Format(time.RFC3339))), nil
}

// Value 实现driver.Valuer接口，用于数据库写入
func (t FlexTime) Value() (driver.Value, error) {
	if t.Time.IsZero() {
		return nil, nil
	}
	return t.Time, nil
}

// Scan 实现sql.Scanner接口，用于数据库读取
func (t *FlexTime) Scan(value interface{}) error {
	if value == nil {
		t.Time = time.Time{}
		return nil
	}
	
	switch v := value.(type) {
	case time.Time:
		t.Time = v
		return nil
	default:
		return fmt.Errorf("无法将 %T 转换为 FlexTime", value)
	}
}

// IsZero 判断是否为零值
func (t FlexTime) IsZero() bool {
	return t.Time.IsZero()
}

// String 返回字符串表示
func (t FlexTime) String() string {
	if t.Time.IsZero() {
		return ""
	}
	return t.Time.Format("2006-01-02 15:04:05")
}

// NewFlexTime 创建FlexTime实例
func NewFlexTime(t time.Time) *FlexTime {
	return &FlexTime{Time: t}
}

// ToTime 转换为标准time.Time指针
func (t *FlexTime) ToTime() *time.Time {
	if t == nil || t.Time.IsZero() {
		return nil
	}
	return &t.Time
}

// FromTime 从标准time.Time指针创建FlexTime
func FromTime(t *time.Time) *FlexTime {
	if t == nil {
		return nil
	}
	return &FlexTime{Time: *t}
}






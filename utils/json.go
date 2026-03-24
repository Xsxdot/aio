package utils

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/xsxdot/aio/pkg/core/model/common"
)

type NormalJson struct {
	Values []JsonValue `json:"values"` //字段数组
}

type JsonValue struct {
	Key     string      `json:"key"`                                                //jsonKey
	Name    string      `json:"name"`                                               //字段名称
	Value   string      `json:"value" gorm:"type:text" `                            //字段值，根据type来string化的结果，无论是数字还是任何类型，最终保存成json，解析成json的时候，再根据实际类型转换
	Type    ValueType   `json:"type"`                                               //字段类型
	Must    common.Flag `json:"must,omitempty"`                                     //字段是否必填
	IsArray common.Flag `json:"isArray,omitempty"`                                  //字段是否为数组
	Options []string    `json:"options,omitempty" gorm:"type:jsonb;serializer:json"` //值的快速选项
	Entity  *NormalJson `json:"entity,omitempty" gorm:"type:jsonb;serializer:json"`  //字段为inner是，下面还有一层
}

type ValueType string

const (
	ValueTypeString ValueType = "string"
	ValueTypeOssKey ValueType = "oss"
	ValueTypeUrl    ValueType = "url"
	ValueTypeInt    ValueType = "int"
	ValueTypeFloat  ValueType = "float"
	ValueTypeBool   ValueType = "bool"
	ValueTypeInner  ValueType = "inner" // 包含一层NormalJson类型
)

// parseRawByType 将任意 raw（字符串或 JSON 解析出的类型）按 Type 解析为对应类型，供 GetInterfaceValue 与数组元素解析复用
func (j *JsonValue) parseRawByType(raw any) (any, error) {
	if raw == nil {
		return nil, nil
	}
	str, ok := raw.(string)
	if !ok {
		switch j.Type {
		case ValueTypeInt:
			switch v := raw.(type) {
			case float64:
				return int64(v), nil
			case int:
				return int64(v), nil
			case int64:
				return v, nil
			}
		case ValueTypeFloat:
			switch v := raw.(type) {
			case float64:
				return v, nil
			case int:
				return float64(v), nil
			case int64:
				return float64(v), nil
			}
		case ValueTypeBool:
			if v, ok := raw.(bool); ok {
				return v, nil
			}
		}
		str = fmt.Sprint(raw)
	}
	switch j.Type {
	case ValueTypeString, ValueTypeOssKey, ValueTypeUrl:
		return str, nil
	case ValueTypeInt:
		return strconv.ParseInt(str, 10, 64)
	case ValueTypeFloat:
		return strconv.ParseFloat(str, 64)
	case ValueTypeBool:
		return strconv.ParseBool(str)
	}
	return str, nil
}

func (j *JsonValue) GetInterfaceValue() (any, error) {
	if j.Type == ValueTypeInner {
		return j.Entity.GetInterfaceValueMap()
	}
	return j.parseRawByType(j.Value)
}

// parseElementByType 将数组中的单个元素按 ValueType 解析为对应类型
func (j *JsonValue) parseElementByType(elem any) (any, error) {
	return j.parseRawByType(elem)
}

func (j *NormalJson) GetInterfaceValueMap() (map[string]any, error) {
	result := make(map[string]any)
	for _, value := range j.Values {
		if value.Type == ValueTypeInner {
			innerMap, err := value.Entity.GetInterfaceValueMap()
			if err == nil {
				result[value.Key] = innerMap
			}
			continue
		}

		if value.IsArray.True() {
			var raw []any
			if err := json.Unmarshal([]byte(value.Value), &raw); err == nil {
				typed := make([]any, 0, len(raw))
				for _, elem := range raw {
					if v, err := value.parseElementByType(elem); err == nil {
						typed = append(typed, v)
					} else {
						typed = append(typed, elem)
					}
				}
				result[value.Key] = typed
			}
			continue
		}

		v, err := value.GetInterfaceValue()
		if err == nil {
			result[value.Key] = v
		}
	}
	return result, nil
}

func (j *NormalJson) GetJson() ([]byte, error) {
	values, err := j.GetInterfaceValueMap()
	if err != nil {
		return nil, err
	}
	return json.Marshal(values)
}

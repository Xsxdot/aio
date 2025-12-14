package common

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
)

type JSON map[string]interface{}

// 实现 sql.Scanner 接口，Scan 将 value 扫描至 Jsonb
func (j *JSON) Scan(value interface{}) error {
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New(fmt.Sprint("Failed to unmarshal JSONB value:", value))
	}

	result := make(map[string]interface{})
	err := json.Unmarshal(bytes, &result)
	*j = result
	return err
}

// 实现 driver.Valuer 接口，Value 返回 json value
func (j JSON) Value() (driver.Value, error) {

	return json.Marshal(j)
}

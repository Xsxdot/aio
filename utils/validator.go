package utils

import (
	"sync"

	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
)

var (
	validate     *validator.Validate
	translator   ut.Translator
	validateOnce sync.Once
)

// GetValidator 获取全局验证器实例
func GetValidator() (*validator.Validate, ut.Translator) {
	validateOnce.Do(func() {
		validate, translator = NewValidator()
	})
	return validate, translator
}

// Validate 验证结构体并返回中文错误信息
func Validate(data interface{}) (string, error) {
	v, trans := GetValidator()
	return ValidateStruct(v, trans, data)
}

// ValidationError 获取第一个验证错误的中文描述
func ValidationError(err error) string {
	_, trans := GetValidator()
	return GetValidationError(err, trans)
}

// IsValid 检查结构体是否有效，返回是否有效及错误信息
func IsValid(data interface{}) (bool, string) {
	errMsg, err := Validate(data)
	if err != nil {
		return false, errMsg
	}
	return true, ""
}

package utils

import (
	"reflect"
	"strings"

	"github.com/go-playground/locales/zh"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	zh_translations "github.com/go-playground/validator/v10/translations/zh"
)

// 常见中文错误信息映射
var customErrorMessages = map[string]string{
	"required": "不能为空",
	"email":    "必须是有效的电子邮件地址",
	"min":      "长度必须至少为%s",
	"max":      "长度不能超过%s",
	"oneof":    "必须是[%s]中的一个", // 这个会在registerCustomTranslation中特殊处理
	"len":      "长度必须是%s",
	"eq":       "必须等于%s",
	"ne":       "不能等于%s",
	"gt":       "必须大于%s",
	"gte":      "必须大于或等于%s",
	"lt":       "必须小于%s",
	"lte":      "必须小于或等于%s",
	"numeric":  "必须是有效的数值",
	"datetime": "必须是有效的日期时间格式",
	"alpha":    "只能包含字母",
	"alphanum": "只能包含字母和数字",
	"url":      "必须是有效的URL",
	"json":     "必须是有效的JSON格式",
}

// NewValidator 创建一个支持中文错误信息的验证器
func NewValidator() (*validator.Validate, ut.Translator) {
	// 创建验证器实例
	validate := validator.New()

	// 注册函数，获取struct字段中的中文标签
	validate.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("comment"), ",", 2)[0]
		if name == "" {
			name = strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		}
		if name == "-" {
			return fld.Name
		}
		return name
	})

	// 创建中文翻译器
	zhTrans := zh.New()
	uni := ut.New(zhTrans, zhTrans)
	trans, _ := uni.GetTranslator("zh")

	// 注册默认的中文翻译器
	zh_translations.RegisterDefaultTranslations(validate, trans)

	// 注册自定义的错误信息
	for tag, msg := range customErrorMessages {
		registerCustomTranslation(validate, trans, tag, msg)
	}

	return validate, trans
}

// 注册自定义翻译
func registerCustomTranslation(validate *validator.Validate, trans ut.Translator, tag string, message string) {
	validate.RegisterTranslation(tag, trans, func(ut ut.Translator) error {
		return ut.Add(tag, message, true)
	}, func(ut ut.Translator, fe validator.FieldError) string {
		// 根据不同的验证规则处理参数
		switch tag {
		case "oneof":
			return fe.Field() + "必须是[" + fe.Param() + "]中的一个"
		case "min", "max", "len", "eq", "ne", "gt", "gte", "lt", "lte":
			// 这些规则需要参数替换
			t, _ := ut.T(fe.Tag(), fe.Field(), fe.Param())
			return t
		default:
			// 其他规则直接使用字段名
			return fe.Field() + message
		}
	})
}

// ValidateStruct 验证结构体并返回中文错误信息
func ValidateStruct(validate *validator.Validate, trans ut.Translator, s interface{}) (string, error) {
	err := validate.Struct(s)
	if err == nil {
		return "", nil
	}

	errs := err.(validator.ValidationErrors)
	var errMessages []string
	for _, e := range errs {
		errMessages = append(errMessages, e.Translate(trans))
	}

	return strings.Join(errMessages, "; "), err
}

// GetValidationError 从错误中提取第一个验证错误的中文描述
func GetValidationError(err error, trans ut.Translator) string {
	if err == nil {
		return ""
	}

	validationErrors, ok := err.(validator.ValidationErrors)
	if !ok || len(validationErrors) == 0 {
		return err.Error()
	}

	return validationErrors[0].Translate(trans)
}

// ValidateBOMOperation 验证BOM工序的业务规则
func ValidateBOMOperation(operation interface{}) (string, error) {
	v, trans := GetValidator()

	// 先进行基础验证
	if errMsg, err := ValidateStruct(v, trans, operation); err != nil {
		return errMsg, err
	}

	// 获取反射对象用于访问字段
	value := reflect.ValueOf(operation)
	if value.Kind() == reflect.Ptr {
		value = value.Elem()
	}

	// 获取TimingMode字段
	timingModeField := value.FieldByName("TimingMode")
	if !timingModeField.IsValid() {
		return "", nil
	}

	timingMode := timingModeField.String()

	// 获取RecommendedWorkers字段
	recommendedWorkersField := value.FieldByName("RecommendedWorkers")
	if !recommendedWorkersField.IsValid() {
		return "", nil
	}

	recommendedWorkers := int(recommendedWorkersField.Int())

	// 如果是BATCH模式，建议人数必须大于0
	if timingMode == "BATCH" && recommendedWorkers <= 0 {
		return "批次模式下建议人数必须大于0", nil
	}

	// 如果是PROPORTIONAL模式且建议人数为0，设置默认值1
	if timingMode == "PROPORTIONAL" && recommendedWorkers <= 0 {
		if recommendedWorkersField.CanSet() {
			recommendedWorkersField.SetInt(1)
		}
	}

	return "", nil
}

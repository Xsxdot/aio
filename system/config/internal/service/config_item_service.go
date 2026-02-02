package service

import (
	"context"
	"encoding/json"
	"strings"
	errorc "github.com/xsxdot/aio/pkg/core/err"
	"github.com/xsxdot/aio/pkg/core/logger"
	"github.com/xsxdot/aio/pkg/core/mvc"
	"github.com/xsxdot/aio/system/config/internal/dao"
	"github.com/xsxdot/aio/system/config/internal/model"

	"gorm.io/gorm"
)

// ConfigItemService 配置项服务
type ConfigItemService struct {
	mvc.IBaseService[model.ConfigItemModel]
	dao               *dao.ConfigItemDao
	encryptionService *EncryptionService
	log               *logger.Log
	err               *errorc.ErrorBuilder
}

// NewConfigItemService 创建配置项服务实例
func NewConfigItemService(dao *dao.ConfigItemDao, encryptionService *EncryptionService, log *logger.Log) *ConfigItemService {
	return &ConfigItemService{
		IBaseService:      mvc.NewBaseService[model.ConfigItemModel](dao.IBaseDao),
		dao:               dao,
		encryptionService: encryptionService,
		log:               log,
		err:               errorc.NewErrorBuilder("ConfigItemService"),
	}
}

// FindByKey 根据配置键查询
func (s *ConfigItemService) FindByKey(ctx context.Context, key string) (*model.ConfigItemModel, error) {
	return s.dao.FindByKey(ctx, key)
}

// FindByKeyWithDecrypt 根据配置键查询并解密
func (s *ConfigItemService) FindByKeyWithDecrypt(ctx context.Context, key string) (*model.ConfigItem, error) {
	item, err := s.dao.FindByKey(ctx, key)
	if err != nil {
		return nil, err
	}

	return s.ConvertAndDecrypt(item)
}

// ConvertAndDecrypt 转换数据库模型为业务模型并解密
func (s *ConfigItemService) ConvertAndDecrypt(dbModel *model.ConfigItemModel) (*model.ConfigItem, error) {
	// 解析 Value JSON
	var values map[string]*model.ConfigValue
	if err := json.Unmarshal([]byte(dbModel.Value), &values); err != nil {
		return nil, s.err.New("解析配置值失败", err)
	}

	// 解密敏感字段
	if err := s.encryptionService.DecryptConfigValues(values); err != nil {
		return nil, err
	}

	// 解析 Metadata JSON
	var metadata map[string]string
	if dbModel.Metadata != "" {
		if err := json.Unmarshal([]byte(dbModel.Metadata), &metadata); err != nil {
			return nil, s.err.New("解析元数据失败", err)
		}
	}

	return &model.ConfigItem{
		Key:      dbModel.Key,
		Value:    values,
		Version:  dbModel.Version,
		Metadata: metadata,
	}, nil
}

// GetConfigValueByEnv 根据环境获取配置值（已废弃，保留用于兼容）
// Deprecated: 使用 GetConfigAsPlainObject 替代
func (s *ConfigItemService) GetConfigValueByEnv(ctx context.Context, key string, env string) (*model.ConfigValue, error) {
	configItem, err := s.FindByKeyWithDecrypt(ctx, key)
	if err != nil {
		return nil, err
	}

	// 查找指定环境的配置
	if value, ok := configItem.Value[env]; ok {
		return value, nil
	}

	// 如果没有找到，尝试查找全局配置
	if value, ok := configItem.Value["global"]; ok {
		return value, nil
	}

	return nil, s.err.New("指定环境的配置不存在", nil).WithCode(errorc.ErrorCodeNotFound)
}

// GetConfigAsPlainObject 根据完整配置键获取配置，返回为纯对象（map[属性名]原始值）
func (s *ConfigItemService) GetConfigAsPlainObject(ctx context.Context, fullKey string) (map[string]interface{}, error) {
	return s.getConfigAsPlainObjectWithRefChain(ctx, fullKey, make(map[string]bool))
}

// getConfigAsPlainObjectWithRefChain 递归解析配置引用（带循环检测）
func (s *ConfigItemService) getConfigAsPlainObjectWithRefChain(ctx context.Context, fullKey string, refChain map[string]bool) (map[string]interface{}, error) {
	// 检测循环引用
	if refChain[fullKey] {
		return nil, s.err.New("检测到循环引用: "+fullKey, nil).ValidWithCtx()
	}
	refChain[fullKey] = true

	configItem, err := s.FindByKeyWithDecrypt(ctx, fullKey)
	if err != nil {
		return nil, err
	}

	result, err := ConvertConfigValuesToPlanObject(configItem.Value)
	if err != nil {
		return nil, err
	}

	// 处理引用类型，递归解析
	for field := range result {
		// 检查原始 ConfigValue 的类型
		if configValue, ok := configItem.Value[field]; ok && configValue.Type == model.ValueTypeRef {
			// ref 类型的 value 格式为 "配置键.字段名"，例如 "app.cert.a.12"
			refValue := configValue.Value

			// 查找最后一个 "." 的位置
			lastDotIndex := strings.LastIndex(refValue, ".")
			if lastDotIndex == -1 {
				// 如果没有 "."，说明引用的是整个配置对象
				// 为每个引用路径创建独立的 refChain 副本，避免误判循环引用
				newRefChain := copyRefChain(refChain)
				refConfig, err := s.getConfigAsPlainObjectWithRefChain(ctx, refValue, newRefChain)
				if err != nil {
					return nil, s.err.New("获取引用配置失败: "+refValue, err)
				}
				result[field] = refConfig
			} else {
				// 分离配置键和字段名
				refKey := refValue[:lastDotIndex]         // "app.cert.a"
				refFieldName := refValue[lastDotIndex+1:] // "12"

				// 为每个引用路径创建独立的 refChain 副本，避免误判循环引用
				newRefChain := copyRefChain(refChain)
				// 递归获取引用的配置
				refConfig, err := s.getConfigAsPlainObjectWithRefChain(ctx, refKey, newRefChain)
				if err != nil {
					return nil, s.err.New("获取引用配置失败: "+refKey, err)
				}

				// 获取指定字段的值
				if refFieldValue, ok := refConfig[refFieldName]; ok {
					result[field] = refFieldValue
				} else {
					return nil, s.err.New("引用的字段不存在: "+refFieldName+" in "+refKey, nil).ValidWithCtx()
				}
			}
		}
	}

	return result, nil
}

// copyRefChain 复制引用链 map，避免不同引用路径互相影响
func copyRefChain(original map[string]bool) map[string]bool {
	copied := make(map[string]bool, len(original))
	for k, v := range original {
		copied[k] = v
	}
	return copied
}

// ConvertConfigValuesToPlanObject 将 map[属性名]*ConfigValue 转换为 map[string]interface{}
// 按 type 字段解码为原始值类型
// 兼容旧数据：如果检测到 values 的 key 全部是环境名（dev/test/prod/staging/global），
// 则视为旧形态，将其转为 {"value": <解码后的值>} 返回
func ConvertConfigValuesToPlanObject(values map[string]*model.ConfigValue) (map[string]interface{}, error) {
	// 检测是否为旧数据形态（所有 key 都是环境名）
	if isLegacyEnvShape(values) {
		// 旧形态：取第一个环境的值作为默认值，映射到 "value" 属性
		for _, cv := range values {
			decoded, err := decodeConfigValue(cv, "value")
			if err != nil {
				return nil, err
			}
			return map[string]interface{}{"value": decoded}, nil
		}
	}

	result := make(map[string]interface{}, len(values))

	for field, cv := range values {
		decoded, err := decodeConfigValue(cv, field)
		if err != nil {
			return nil, err
		}
		result[field] = decoded
	}

	return result, nil
}

// isLegacyEnvShape 检测是否为旧数据形态（所有 key 都是环境名）
func isLegacyEnvShape(values map[string]*model.ConfigValue) bool {
	if len(values) == 0 {
		return false
	}

	validEnvs := map[string]bool{
		"dev":     true,
		"test":    true,
		"prod":    true,
		"staging": true,
		"global":  true,
	}

	// 检查所有 key 是否都是有效的环境名
	for key := range values {
		if !validEnvs[key] {
			return false
		}
	}

	return true
}

// decodeConfigValue 根据 type 解码 ConfigValue 为原始值
func decodeConfigValue(cv *model.ConfigValue, field string) (interface{}, error) {
	var decoded interface{}
	var err error

	switch cv.Type {
	case model.ValueTypeString, model.ValueTypeEncrypted:
		// 字符串和加密类型直接返回字符串
		decoded = cv.Value

	case model.ValueTypeInt:
		// 解析为整数
		var intVal int64
		if err = json.Unmarshal([]byte(cv.Value), &intVal); err != nil {
			return nil, errorc.New("解析整数配置失败: "+field, err)
		}
		decoded = intVal

	case model.ValueTypeFloat:
		// 解析为浮点数
		var floatVal float64
		if err = json.Unmarshal([]byte(cv.Value), &floatVal); err != nil {
			return nil, errorc.New("解析浮点数配置失败: "+field, err)
		}
		decoded = floatVal

	case model.ValueTypeBool:
		// 解析为布尔值
		var boolVal bool
		if err = json.Unmarshal([]byte(cv.Value), &boolVal); err != nil {
			return nil, errorc.New("解析布尔配置失败: "+field, err)
		}
		decoded = boolVal

	case model.ValueTypeObject:
		// 解析为对象
		var objVal map[string]interface{}
		if err = json.Unmarshal([]byte(cv.Value), &objVal); err != nil {
			return nil, errorc.New("解析对象配置失败: "+field, err)
		}
		decoded = objVal

	case model.ValueTypeArray:
		// 解析为数组
		var arrVal []interface{}
		if err = json.Unmarshal([]byte(cv.Value), &arrVal); err != nil {
			return nil, errorc.New("解析数组配置失败: "+field, err)
		}
		decoded = arrVal

	case model.ValueTypeRef:
		// 引用类型保持为字符串（后续由调用方解析）
		decoded = cv.Value

	default:
		// 默认作为字符串
		decoded = cv.Value
	}

	return decoded, nil
}

// CreateWithEncrypt 创建配置（自动加密）
func (s *ConfigItemService) CreateWithEncrypt(ctx context.Context, configItem *model.ConfigItem) error {
	// 验证配置键
	if err := ValidateConfigKey(configItem.Key); err != nil {
		return err
	}

	// 检查是否已存在
	exists, err := s.dao.ExistsByKey(ctx, configItem.Key)
	if err != nil {
		return err
	}
	if exists {
		return s.err.New("配置键已存在", nil).ValidWithCtx()
	}

	// 加密敏感字段
	if err := s.encryptionService.EncryptConfigValues(configItem.Value); err != nil {
		return err
	}

	// 序列化为 JSON
	valueJSON, err := json.Marshal(configItem.Value)
	if err != nil {
		return s.err.New("序列化配置值失败", err)
	}

	var metadataJSON []byte
	if configItem.Metadata != nil {
		metadataJSON, err = json.Marshal(configItem.Metadata)
		if err != nil {
			return s.err.New("序列化元数据失败", err)
		}
	}

	// 创建数据库模型
	dbModel := &model.ConfigItemModel{
		Key:      configItem.Key,
		Value:    string(valueJSON),
		Version:  1,
		Metadata: string(metadataJSON),
	}

	return s.dao.Create(ctx, dbModel)
}

// UpdateWithEncrypt 更新配置（自动加密）
func (s *ConfigItemService) UpdateWithEncrypt(ctx context.Context, id int64, configItem *model.ConfigItem) error {
	// 加密敏感字段
	if err := s.encryptionService.EncryptConfigValues(configItem.Value); err != nil {
		return err
	}

	// 序列化为 JSON
	valueJSON, err := json.Marshal(configItem.Value)
	if err != nil {
		return s.err.New("序列化配置值失败", err)
	}

	var metadataJSON []byte
	if configItem.Metadata != nil {
		metadataJSON, err = json.Marshal(configItem.Metadata)
		if err != nil {
			return s.err.New("序列化元数据失败", err)
		}
	}

	// 更新数据库模型
	dbModel := &model.ConfigItemModel{
		Value:    string(valueJSON),
		Metadata: string(metadataJSON),
	}

	_, err = s.dao.UpdateById(ctx, id, dbModel)
	return err
}

// FindByKeyLike 根据配置键模糊查询
func (s *ConfigItemService) FindByKeyLike(ctx context.Context, keyPattern string) ([]*model.ConfigItemModel, error) {
	return s.dao.FindByKeyLike(ctx, keyPattern)
}

// FindByKeyPrefix 根据配置键前缀查询
func (s *ConfigItemService) FindByKeyPrefix(ctx context.Context, prefix string) ([]*model.ConfigItemModel, error) {
	return s.dao.FindByKeyPrefix(ctx, prefix)
}

// FindPageByKeyLike 根据配置键模糊查询（分页）
func (s *ConfigItemService) FindPageByKeyLike(ctx context.Context, page *mvc.Page, keyPattern string) ([]*model.ConfigItemModel, int64, error) {
	return s.dao.FindPageByKeyLike(ctx, page, keyPattern)
}

// FindAll 查询所有配置项
func (s *ConfigItemService) FindAll(ctx context.Context) ([]*model.ConfigItemModel, error) {
	return s.dao.FindAll(ctx)
}

// FindByKeys 根据多个配置键查询
func (s *ConfigItemService) FindByKeys(ctx context.Context, keys []string) ([]*model.ConfigItemModel, error) {
	return s.dao.FindByKeys(ctx, keys)
}

// IncrementVersion 增加版本号
func (s *ConfigItemService) IncrementVersion(ctx context.Context, id int64) error {
	return s.dao.IncrementVersion(ctx, id)
}

// WithTx 使用事务
func (s *ConfigItemService) WithTx(tx *gorm.DB) *ConfigItemService {
	return &ConfigItemService{
		IBaseService:      s.IBaseService.WithTx(tx),
		dao:               s.dao.WithTx(tx),
		encryptionService: s.encryptionService,
		log:               s.log,
		err:               s.err,
	}
}

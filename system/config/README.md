# 配置中心使用文档

## 概述

配置中心是一个完整的配置管理系统，支持多环境配置、加密存储、历史版本、缓存优化、导入导出等功能。

## 功能特性

- ✅ **多环境配置**：支持 dev/test/prod/staging 等多环境隔离
- ✅ **加密存储**：敏感配置使用 AES-256-GCM 加密，加密值带 `ENC:` 前缀
- ✅ **历史版本**：每次更新自动记录历史版本，支持版本回滚
- ✅ **对外 API**：提供 JSON 字符串或对象反序列化接口
- ✅ **缓存优化**：使用 Redis 缓存提升查询性能（TTL: 5分钟）
- ✅ **导入导出**：支持配置的批量导入导出，可指定加密 salt

## 目录结构

```
system/config/
├── internal/              # 内部实现（不对外暴露）
│   ├── model/            # 数据模型
│   │   ├── types.go      # 业务类型定义
│   │   ├── config_item.go         # 配置项数据库模型
│   │   ├── config_history.go      # 配置历史数据库模型
│   │   └── dto/          # 内部传输对象
│   ├── dao/              # 数据访问层
│   │   ├── config_item_dao.go
│   │   └── config_history_dao.go
│   ├── service/          # 业务逻辑层
│   │   ├── config_item_service.go
│   │   ├── config_history_service.go
│   │   └── encryption_service.go
│   └── app/              # 应用编排层
│       ├── app.go
│       ├── config_manage.go       # 配置管理
│       └── config_export.go       # 导入导出
├── api/                  # 对外接口
│   ├── dto/              # 对外 DTO
│   │   └── config_api_dto.go
│   └── client/           # 对外客户端
│       └── config_client.go
├── external/             # 外部适配层
│   └── http/
│       └── controller/
│           ├── config_controller.go       # 后台管理接口
│           └── config_api_controller.go   # 对外查询接口
├── app_facade.go         # 应用门面（对外暴露）
├── migrate.go            # 数据库迁移
└── README.md             # 本文档
```

## 配置 Key 设计

### 格式规范

配置键格式：`<module>.<submodule>.<env>`

- 示例：`app.cert.dev`、`payment.wechat.prod`
- **环境后缀可选**：允许 `app.global` 这样的全局配置（不带环境）

### 验证规则

- Key 由 2-4 部分组成，用 `.` 分隔
- 如果最后一部分是合法环境名（dev/test/prod/staging），则识别为环境配置
- 否则视为全局配置（不区分环境）

### 示例

```
app.cert.dev              # 开发环境证书配置
app.cert.prod             # 生产环境证书配置
payment.wechat.test       # 测试环境微信支付配置
system.global             # 全局系统配置（不分环境）
```

## API 接口

### 后台管理接口（需要管理员权限）

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/admin/configs` | 创建配置 |
| PUT | `/admin/configs/:id` | 更新配置 |
| DELETE | `/admin/configs/:id` | 删除配置 |
| GET | `/admin/configs` | 分页查询配置 |
| GET | `/admin/configs/:id` | 查询单个配置 |
| GET | `/admin/configs/:id/history` | 查询历史版本 |
| POST | `/admin/configs/:id/rollback/:version` | 回滚到指定版本 |
| POST | `/admin/configs/export` | 导出配置 |
| POST | `/admin/configs/import` | 导入配置 |

### 对外查询接口（供其他组件调用）

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/configs/get` | 获取单个配置 |
| POST | `/api/configs/batch` | 批量获取配置 |

## 使用示例

### 1. 创建配置

```bash
POST /admin/configs
Content-Type: application/json
Authorization: Bearer <admin_token>

{
  "key": "app.cert.dev",
  "value": {
    "dev": {
      "value": "cert_content_here",
      "type": "encrypted"
    }
  },
  "metadata": {
    "owner": "backend-team",
    "updateTime": "2024-01-01"
  },
  "description": "应用证书配置",
  "changeNote": "初始化配置"
}
```

### 2. 更新配置

```bash
PUT /admin/configs/1
Content-Type: application/json
Authorization: Bearer <admin_token>

{
  "value": {
    "dev": {
      "value": "new_cert_content",
      "type": "encrypted"
    }
  },
  "changeNote": "更新证书"
}
```

### 3. 查询配置（对外接口）

```bash
POST /api/configs/get
Content-Type: application/json

{
  "key": "app.cert",
  "env": "dev"
}
```

响应：

```json
{
  "status": 200,
  "data": {
    "value": "decrypted_cert_content",
    "type": "encrypted"
  }
}
```

### 4. 在代码中使用配置客户端

```go
package main

import (
    "context"
    "xiaozhizhang/system/config/api/client"
)

func example(configClient *client.ConfigClient) {
    ctx := context.Background()
    
    // 获取配置（JSON 字符串）
    jsonStr, err := configClient.GetConfigJSON(ctx, "app.cert", "dev")
    if err != nil {
        // 处理错误
    }
    
    // 获取配置（反序列化到结构体）
    var certConfig CertConfig
    err = configClient.GetConfig(ctx, "app.cert", "dev", &certConfig)
    if err != nil {
        // 处理错误
    }
    
    // 批量获取配置
    keys := []string{"app.cert", "payment.wechat"}
    configs, err := configClient.GetConfigs(ctx, keys, "dev")
    if err != nil {
        // 处理错误
    }
}
```

### 5. 导出配置

```bash
POST /admin/configs/export
Content-Type: application/json
Authorization: Bearer <admin_token>

{
  "keys": ["app.cert.dev", "payment.wechat.dev"],
  "environment": "dev",
  "targetSalt": ""  // 为空则使用当前系统盐值
}
```

### 6. 导入配置

```bash
POST /admin/configs/import
Content-Type: application/json
Authorization: Bearer <admin_token>

{
  "sourceSalt": "old_salt_value",  // 源文件的盐值，为空则认为与当前系统相同
  "overWrite": true,  // 是否覆盖已存在的配置
  "configs": [
    {
      "key": "app.cert.dev",
      "value": {
        "dev": {
          "value": "ENC:xxx",
          "type": "encrypted"
        }
      },
      "metadata": {},
      "description": "应用证书",
      "version": 1
    }
  ]
}
```

### 7. 回滚配置

```bash
POST /admin/configs/1/rollback/3
Authorization: Bearer <admin_token>
```

## 配置值类型

系统支持以下配置值类型：

| 类型 | 说明 |
|------|------|
| `string` | 普通字符串 |
| `int` | 整数 |
| `float` | 浮点数 |
| `bool` | 布尔值 |
| `encrypted` | 加密类型（自动加解密） |
| `object` | 对象类型 |
| `array` | 数组类型 |
| `ref` | 引用其他配置项 |

## 加密说明

### 加密算法

- 使用 AES-256-GCM 算法
- 密钥由配置文件中的 `encryption-salt` 生成（SHA-256）
- 加密后的字符串带 `ENC:` 前缀

### 配置加密盐值

在 `resources/dev.yaml` 中配置：

```yaml
config:
  encryption-salt: 'xiaozhizhang-config-center-encryption-salt-2024'
```

### 自动加解密

- **创建/更新配置**：当 `type = "encrypted"` 时，系统自动加密 `value` 字段
- **查询配置**：系统自动解密返回明文
- **导入导出**：支持使用不同的盐值重新加密

## 缓存策略

- 使用 Redis 缓存查询结果
- 缓存键格式：`config:<key>:<env>`
- TTL：5 分钟
- 更新/删除配置时自动清除相关缓存

## 数据库表结构

### config_items（配置主表）

| 字段 | 类型 | 说明 |
|------|------|------|
| id | bigint | 主键 |
| key | varchar(255) | 配置键（唯一索引） |
| value | json | 配置值 |
| version | bigint | 当前版本号 |
| metadata | json | 元数据 |
| description | varchar(500) | 配置描述 |
| created_at | timestamp | 创建时间 |
| updated_at | timestamp | 更新时间 |
| deleted_at | timestamp | 删除时间（软删除） |

### config_history（历史版本表）

| 字段 | 类型 | 说明 |
|------|------|------|
| id | bigint | 主键 |
| config_key | varchar(255) | 关联的配置键（索引） |
| version | bigint | 版本号 |
| value | json | 该版本的配置值 |
| metadata | json | 该版本的元数据 |
| operator | varchar(100) | 操作人账号 |
| operator_id | bigint | 操作人ID |
| change_note | varchar(500) | 变更说明 |
| created_at | timestamp | 创建时间 |

## 注意事项

1. **权限控制**：所有后台管理接口都需要管理员权限
2. **环境隔离**：生产环境配置请使用 `.prod` 后缀
3. **敏感信息**：密码、密钥等敏感信息务必使用 `encrypted` 类型
4. **版本管理**：重要配置更新前建议记录详细的变更说明
5. **导入导出**：跨环境迁移配置时注意盐值的转换
6. **缓存时效**：配置更新后最多有 5 分钟的缓存延迟（手动清除缓存除外）

## 故障排查

### 配置查询返回 404

- 检查配置键是否正确
- 检查环境参数是否正确
- 确认配置是否已创建

### 解密失败

- 检查加密盐值是否正确
- 确认配置值的 `type` 是否为 `encrypted`
- 检查配置值是否有 `ENC:` 前缀

### 缓存不生效

- 检查 Redis 连接是否正常
- 查看日志确认缓存写入是否成功
- 确认缓存键格式是否正确

## 开发计划

- [ ] 支持配置订阅（实时推送配置变更）
- [ ] 支持配置灰度发布
- [ ] 支持配置审计日志
- [ ] 支持配置模板功能
- [ ] 支持配置依赖关系图

## 许可证

本项目遵循项目主许可证。

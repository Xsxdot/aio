# MySQL 到 PostgreSQL 迁移成本分析报告

> 分析日期：2026-03-11

## 一、项目概览

| 模块 | Model数量 | DAO数量 |
|------|----------|--------|
| SSL | 5 | 4 |
| Server | 3 | 3 |
| User | 2 | 2 |
| Registry | 2 | 2 |
| Config | 2 | 2 |
| ShortURL | 4 | 4 |
| Executor | 2 | 2 |
| **总计** | **20** | **19** |

---

## 二、需要修改的文件清单

### 1. 数据库配置层（低成本）

| 文件 | 修改内容 |
|------|---------|
| `pkg/core/config/database.go` | 已有 `InitPg` 函数，需调整配置读取逻辑，将 `EnableMysql()` 切换为 PostgreSQL |

### 2. Model 层修改（中等成本）

**MySQL 特定类型需要替换：**

| MySQL类型 | 出现次数 | PostgreSQL替代 | 涉及文件 |
|-----------|---------|----------------|---------|
| `type:varchar` | ~25处 | `size` 标签即可，GORM会自动处理 | 多个model文件 |
| `type:text` | 12处 | 直接移除或保留，GORM自动映射 | executor_job.go, certificate.go等 |
| `type:longtext` | 2处 | PostgreSQL使用 `text` 类型 | `system/ssl/internal/model/certificate.go:20-21` |
| `type:tinyint(1)` | 6处 | PostgreSQL使用 `boolean` | server.go, short_link.go等 |
| `type:datetime` | 6处 | PostgreSQL使用 `timestamptz` | server_status.go, short_visit.go等 |
| `type:decimal` | 3处 | PostgreSQL兼容 | server_status.go |
| `type:bigint` | 10处 | PostgreSQL兼容 | 多个model文件 |
| `type:json` | 8处 | PostgreSQL原生JSON支持更好 | 多个model文件 |

**建议：** 大部分 `type:xxx` 标签可以直接移除，让GORM根据字段类型自动推断。PostgreSQL对JSON支持更原生。

### 3. DAO 层修改（高成本）

**MySQL 特定函数需要替换：**

| 函数 | 出现位置 | 替代方案 |
|------|---------|---------|
| `TIMESTAMPDIFF(DAY, ?, expires_at)` | `system/ssl/internal/dao/certificate_dao.go:46` | `expires_at - ? <= interval '1 day' * renew_before_days` 或使用 Go 计算 |
| `JSON_CONTAINS(tags, ?)` | `system/server/internal/dao/server_dao.go:75` | `tags::jsonb @> ?::jsonb` |
| `NOW()` | `system/user/internal/dao/client_credential_dao.go:82` | `CURRENT_TIMESTAMP` 或 `NOW()` (PostgreSQL也支持) |
| `DATE(created_at)` | visit_dao.go, success_event_dao.go | `DATE(created_at)` (PostgreSQL兼容) |

### 4. 复杂 SQL 查询修改（高成本）

**`system/executor/internal/dao/executor_job_dao.go`** 包含最复杂的原生SQL：

- `AcquireJob` 方法（73-92行）：包含复杂的事务和子查询
- 需要仔细测试 `NOT EXISTS` 子查询在 PostgreSQL 中的行为
- 表名引用 `aio_executor_jobs` 需要检查 PostgreSQL 的标识符大小写敏感性

**`system/registry/internal/dao/registry_instance_dao.go`**

- `Upsert` 方法使用 `clause.OnConflict`（77-89行）
- PostgreSQL 的 `ON CONFLICT` 语法与 MySQL 的 `ON DUPLICATE KEY UPDATE` 不同
- 需要修改 `OnConflict` 的 `Columns` 配置，PostgreSQL需要唯一约束名或索引名

---

## 三、各模块迁移详细分析

### 1. SSL 模块 ⚠️ 中等风险

| 文件 | 问题 | 解决方案 |
|------|------|---------|
| `certificate_dao.go:46` | `TIMESTAMPDIFF` MySQL函数 | 改用 Go 代码计算或 PostgreSQL 日期运算 |
| `certificate.go:20-21` | `longtext` 类型 | 改用 `text` |

### 2. Server 模块 ⚠️ 中等风险

| 文件 | 问题 | 解决方案 |
|------|------|---------|
| `server_dao.go:75` | `JSON_CONTAINS` MySQL函数 | 改用 PostgreSQL JSON操作符 `@>` |
| `server_status.go` | `decimal`, `datetime` 类型 | GORM自动处理或移除type标签 |

### 3. User 模块 ✅ 低风险

| 文件 | 问题 | 解决方案 |
|------|------|---------|
| `client_credential_dao.go:82` | `NOW()` 函数 | PostgreSQL兼容 |

### 4. Registry 模块 ⚠️ 高风险

| 文件 | 问题 | 解决方案 |
|------|------|---------|
| `registry_instance_dao.go:77-89` | `OnConflict` 语法 | 需要调整唯一约束的配置方式 |

### 5. ShortURL 模块 ✅ 低风险

| 文件 | 问题 | 解决方案 |
|------|------|---------|
| visit_dao.go, success_event_dao.go | `DATE()` 函数 | PostgreSQL兼容 |

### 6. Executor 模块 ⚠️ 高风险

| 文件 | 问题 | 解决方案 |
|------|------|---------|
| `executor_job_dao.go` | 复杂原生SQL、表名引用 | 需要全面重写和测试 |

### 7. Config 模块 ✅ 低风险

无MySQL特定语法，基本兼容。

---

## 四、工作量估算

| 工作项 | 文件数 | 预估工时 | 优先级 |
|--------|--------|---------|--------|
| 数据库配置切换 | 2 | 0.5天 | P0 |
| Model类型标签清理 | 10+ | 1天 | P0 |
| DAO函数替换 (TIMESTAMPDIFF, JSON_CONTAINS) | 3 | 1天 | P0 |
| OnConflict语法调整 | 1 | 0.5天 | P0 |
| 复杂SQL重写 (executor_job_dao) | 1 | 1-2天 | P1 |
| 单元测试更新 | 全部 | 2天 | P1 |
| 集成测试 | 全部 | 2天 | P1 |
| 数据迁移脚本 | - | 1-3天 | P2 |
| **总计** | - | **8-12天** | - |

---

## 五、迁移建议

### 1. 好消息

- 项目已使用 GORM ORM，大部分操作是数据库无关的
- `pkg/core/config/database.go` 已经包含了 `InitPg` 函数，说明项目已经考虑过PostgreSQL支持
- PostgreSQL 对 JSON/JSONB 支持更好，`type:json` 字段可以获得更好的性能

### 2. 迁移步骤建议

1. **创建迁移分支**，保留MySQL版本
2. **修改配置层**，切换到PostgreSQL连接
3. **清理Model标签**，移除MySQL特定类型
4. **修改DAO层SQL**，替换MySQL特定函数
5. **调整OnConflict**，适配PostgreSQL语法
6. **编写数据迁移脚本**，迁移现有数据
7. **全面测试**，特别是Executor模块的复杂查询

### 3. 风险点

- **Executor模块**：复杂的任务领取逻辑使用了大量原生SQL，需要仔细测试
- **JSON查询**：`JSON_CONTAINS` 需要改用 PostgreSQL 的 JSON 操作符
- **日期函数**：`TIMESTAMPDIFF` 需要重写

---

## 六、需要修改的具体代码位置

### 6.1 TIMESTAMPDIFF 替换

**文件**: `system/ssl/internal/dao/certificate_dao.go:46`

```go
// MySQL 原代码
Where("TIMESTAMPDIFF(DAY, ?, expires_at) <= renew_before_days", now)

// PostgreSQL 替代方案1：使用 Go 计算
now := time.Now()
err := d.db.WithContext(ctx).
    Where("auto_renew = ?", 1).
    Where("status = ?", model.CertificateStatusActive).
    Where("expires_at IS NOT NULL").
    Where("expires_at <= ?", now.AddDate(0, 0, 30)). // 假设默认30天，需动态计算
    Find(&certificates).Error

// PostgreSQL 替代方案2：使用 PostgreSQL 日期运算
Where("expires_at - ? <= INTERVAL '1 day' * renew_before_days", now)
```

### 6.2 JSON_CONTAINS 替换

**文件**: `system/server/internal/dao/server_dao.go:75`

```go
// MySQL 原代码
query = query.Where("JSON_CONTAINS(tags, ?)", "\""+tag+"\"")

// PostgreSQL 替代方案
query = query.Where("tags::jsonb @> ?::jsonb", fmt.Sprintf(`["%s"]`, tag))
```

### 6.3 OnConflict 调整

**文件**: `system/registry/internal/dao/registry_instance_dao.go:77-89`

```go
// 当前代码（需要确认唯一约束名）
return d.db.WithContext(ctx).
    Clauses(clause.OnConflict{
        Columns: []clause.Column{{Name: "service_id"}, {Name: "instance_key"}},
        DoUpdates: clause.AssignmentColumns([]string{...}),
    }).
    Create(inst).Error

// PostgreSQL 需要确保有对应的唯一约束
// 可以在迁移中添加：
// CREATE UNIQUE INDEX idx_service_instance ON registry_instance(service_id, instance_key);
```

### 6.4 Model 类型标签清理示例

**文件**: `system/ssl/internal/model/certificate.go`

```go
// 修改前
FullchainPem string `gorm:"type:longtext" json:"fullchain_pem"`
PrivkeyPem   string `gorm:"type:longtext" json:"privkey_pem"`

// 修改后（PostgreSQL text 类型足够大）
FullchainPem string `gorm:"type:text" json:"fullchain_pem"`
PrivkeyPem   string `gorm:"type:text" json:"privkey_pem"`
```

**文件**: `system/server/internal/model/server.go`

```go
// 修改前
Enabled bool `gorm:"type:tinyint(1);not null;default:1" json:"enabled"`

// 修改后（PostgreSQL 原生 boolean）
Enabled bool `gorm:"not null;default:true" json:"enabled"`
```

---

## 七、结论

**迁移难度：中等**

项目整体迁移工作量可控，约需 **8-12个工作日**。主要风险集中在：

1. `executor_job_dao.go` 的复杂SQL查询
2. `JSON_CONTAINS` 和 `TIMESTAMPDIFF` 函数替换
3. `OnConflict` 语法的差异

建议先在测试环境完成迁移验证，再进行生产环境切换。

---

## 附录：文件清单

### Model 文件

| 文件路径 | 表名 |
|---------|------|
| `system/ssl/internal/model/certificate.go` | ssl_certificates |
| `system/ssl/internal/model/deploy_target.go` | ssl_deploy_targets |
| `system/ssl/internal/model/deploy_history.go` | ssl_deploy_histories |
| `system/ssl/internal/model/dns_credential.go` | ssl_dns_credentials |
| `system/ssl/internal/model/types.go` | (枚举定义) |
| `system/server/internal/model/server.go` | server_servers |
| `system/server/internal/model/server_ssh_credential.go` | server_ssh_credentials |
| `system/server/internal/model/server_status.go` | server_status |
| `system/user/internal/model/admin.go` | user_admin |
| `system/user/internal/model/client_credential.go` | user_client_credential |
| `system/registry/internal/model/registry_service.go` | registry_service |
| `system/registry/internal/model/registry_instance.go` | registry_instance |
| `system/config/internal/model/config_item.go` | config_items |
| `system/config/internal/model/config_history.go` | config_history |
| `system/shorturl/internal/model/short_link.go` | shorturl_links |
| `system/shorturl/internal/model/short_domain.go` | shorturl_domains |
| `system/shorturl/internal/model/short_visit.go` | shorturl_visits |
| `system/shorturl/internal/model/short_success_event.go` | shorturl_success_events |
| `system/executor/internal/model/executor_job.go` | aio_executor_jobs, executor_job_attempts |

### DAO 文件

| 文件路径 |
|---------|
| `system/ssl/internal/dao/certificate_dao.go` |
| `system/ssl/internal/dao/deploy_history_dao.go` |
| `system/ssl/internal/dao/dns_credential_dao.go` |
| `system/ssl/internal/dao/deploy_target_dao.go` |
| `system/server/internal/dao/server_dao.go` |
| `system/server/internal/dao/server_ssh_credential_dao.go` |
| `system/server/internal/dao/server_status_dao.go` |
| `system/user/internal/dao/admin_dao.go` |
| `system/user/internal/dao/client_credential_dao.go` |
| `system/registry/internal/dao/registry_service_dao.go` |
| `system/registry/internal/dao/registry_instance_dao.go` |
| `system/config/internal/dao/config_item_dao.go` |
| `system/config/internal/dao/config_history_dao.go` |
| `system/shorturl/internal/dao/domain_dao.go` |
| `system/shorturl/internal/dao/link_dao.go` |
| `system/shorturl/internal/dao/visit_dao.go` |
| `system/shorturl/internal/dao/success_event_dao.go` |
| `system/executor/internal/dao/executor_job_dao.go` |
| `system/executor/internal/dao/executor_job_attempt_dao.go` |
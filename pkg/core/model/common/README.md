# Common 模型包

本包提供了项目中通用的模型定义和类型。

## 目录结构

- `model.go` - 基础模型定义 (Model, ModelString)
- `time.go` - 灵活的时间类型 (FlexTime)
- `json.go` - JSON 相关类型
- `redis-json.go` - Redis JSON 类型
- `normal-status.go` - 状态相关类型
- `page-return.go` - 分页相关类型

## FlexTime 灵活时间类型

### 功能说明

`FlexTime` 是一个灵活的时间类型，支持多种时间格式的 JSON 解析和序列化，解决了前后端时间格式不统一的问题。

### 支持的时间格式

- RFC3339: `"2006-01-02T15:04:05Z07:00"`
- ISO 8601: `"2006-01-02T15:04:05"`
- 标准格式（带空格）: `"2006-01-02 15:04:05"`
- 带毫秒: `"2006-01-02T15:04:05.999Z"` 或 `"2006-01-02 15:04:05.999"`
- 纯日期: `"2006-01-02"`
- RFC3339Nano: `"2006-01-02T15:04:05.999999999Z07:00"`
- null 或空字符串

### 使用场景

当需要在 JSON 中处理时间字段时，使用 `*common.FlexTime` 代替 `*time.Time`，可以自动处理多种时间格式。

### 基本用法

#### 在 DTO 中使用

```go
package dto

import "xiaozhizhang/pkg/core/model/common"

type CreateProductRequest struct {
    Name          string           `json:"name"`
    ShowStartTime *common.FlexTime `json:"showStartTime" comment:"显示开始时间"`
    ShowEndTime   *common.FlexTime `json:"showEndTime" comment:"显示结束时间"`
}
```

#### 在 Model 中使用

```go
package model

import "xiaozhizhang/pkg/core/model/common"

type Product struct {
    common.Model
    Name          string           `json:"name" gorm:"column:name"`
    ShowStartTime *common.FlexTime `json:"showStartTime" gorm:"column:show_start_time"`
    ShowEndTime   *common.FlexTime `json:"showEndTime" gorm:"column:show_end_time"`
}
```

### API 示例

#### 请求示例

支持多种格式的时间输入：

```json
{
  "name": "测试商品",
  "showStartTime": "2025-11-09 17:44:55",
  "showEndTime": "2025-12-31T23:59:59",
  "saleStartTime": null
}
```

#### 响应示例

统一输出为 RFC3339 格式：

```json
{
  "id": 1,
  "name": "测试商品",
  "showStartTime": "2025-11-09T17:44:55Z",
  "showEndTime": "2025-12-31T23:59:59Z",
  "saleStartTime": null
}
```

### 工具方法

#### 创建 FlexTime

```go
import (
    "time"
    "xiaozhizhang/pkg/core/model/common"
)

// 从 time.Time 创建
now := time.Now()
flexTime := common.NewFlexTime(now)

// 从 *time.Time 创建
var timePtr *time.Time = &now
flexTime := common.FromTime(timePtr)
```

#### 转换为标准 time.Time

```go
// 转换为 *time.Time
var flexTime *common.FlexTime
timePtr := flexTime.ToTime()

// 检查是否为零值
if flexTime.IsZero() {
    // 处理空时间
}

// 获取字符串表示
str := flexTime.String() // 输出: "2025-11-09 17:44:55"
```

### 数据库支持

`FlexTime` 实现了 `driver.Valuer` 和 `sql.Scanner` 接口，可以直接在 GORM 中使用：

```go
// 创建记录
product := &model.Product{
    Name:          "商品",
    ShowStartTime: common.NewFlexTime(time.Now()),
}
db.Create(product)

// 查询记录
var product model.Product
db.First(&product, 1)
if product.ShowStartTime != nil && !product.ShowStartTime.IsZero() {
    fmt.Println(product.ShowStartTime.Time)
}
```

### 注意事项

1. **使用指针类型**: 推荐使用 `*common.FlexTime` 而不是 `common.FlexTime`，这样可以区分 null 和零值。

2. **零值处理**: 
   - null 会被解析为 nil
   - 空字符串会被解析为零值时间
   - 零值时间序列化为 null

3. **时区处理**: 
   - 输入的时间如果没有时区信息，会被解析为 UTC
   - 输出的时间统一使用 RFC3339 格式，包含时区信息

4. **数据库迁移**: 如果从 `*time.Time` 迁移到 `*common.FlexTime`，不需要修改数据库表结构，两者在数据库中的存储格式相同。

### 错误处理

如果提供的时间格式无法被识别，会返回详细的错误信息：

```go
var req dto.CreateProductRequest
err := json.Unmarshal(data, &req)
if err != nil {
    // 错误信息示例: 无法解析时间格式: invalid-time-string, 错误: ...
    return err
}
```

## Flag 类型

用于表示是/否的标志字段：

- 值为 1 表示"是"
- 值为 2 表示"否"

```go
type Product struct {
    OnSale common.Flag `json:"onSale" gorm:"type:tinyint(1);default:2"`
}
```

## Model 类型

所有数据库模型都应该嵌入 `common.Model`，而不是 `gorm.Model`：

```go
type Product struct {
    common.Model  // 包含 ID, CreatedAt, UpdatedAt, DeletedAt
    Name string
}
```






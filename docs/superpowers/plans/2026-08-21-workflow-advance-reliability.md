# Workflow 推进可靠性改造 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 消除工作流推进链路上两处「事务已提交、后续动作却可能不执行」的窗口，使一次节点完成要么完整推进（状态已更新 + 下游任务已入库），要么完全没发生并可重试；彻底失败时表现为 Admin 可见的死信而非静默永久卡死。

**Architecture:** 边界2（workflow 状态提交 → 下游任务派发）用**事务透传**消灭——`ExecutorClient` 是进程内直调、executor DAO 全线 `mvc.ExtractDB`，`mvc.WithTxToContext` 包一层 ctx 即可让两次写入同事务。边界1（executor `AckJob` → workflow 回调）用 **outbox 模式**——`AckJob` 事务内插一条 executor job 承载回调，由 aio 进程内轻量 worker 消费，复用 executor 现成的重试/死信/Admin 可见性。

**Tech Stack:** Go 1.26.1、GORM、`pkg/core/mvc` 事务透传、`system/executor`、`system/workflow`。

**Spec:** `docs/superpowers/specs/2026-08-21-workflow-advance-reliability-design.md`

## Global Constraints

- **数据库双兼容**：所有 SQL 必须同时支持 MySQL 与 PostgreSQL。禁止 MySQL 专有类型（`longtext`/`tinyint(1)`/`datetime`）。跨方言差异走 `pkg/db/dialect.IsMySQL/IsPostgres`，能用 GORM 跨库 API 就不要写原生 SQL。
- **日志**：一律 `logger.GetLogger().WithEntryName("模块")` 或 `base.Logger`。**禁止 `fmt.Printf` / `fmt.Println`**。错误日志必须带上下文（id、状态、cause）。
- **错误**：一律 `errorc.NewErrorBuilder("模块")`，禁止 `errors.New` 出现在新增业务代码里。错误格式 `err.New("说明", cause).WithTraceID(ctx)`。
- **分层**：Controller → App → Service → DAO，不得跨层。DAO 内一律 `mvc.ExtractDB(ctx, d.db)`，禁止裸 `d.db`。
- **不改动 `pkg/sdk/**`**：下游 worker（nova-audio-go、tk-server）无需升级 SDK 或重新部署。
- **不改动 proto**：本计划不涉及 gRPC 接口变更。
- **注释**：新文件写文件头（职责 + 边界）；导出方法写参数/返回/注意事项；非显然分支写「为什么」。
- **软删除陷阱**：`common.Model` 内嵌 `gorm.DeletedAt`。幂等表的清理必须用 `Unscoped()` 硬删——软删除的行仍占用唯一索引，会让该 `job_id` 永久无法再登记。
- **SQLite 测试陷阱**：`gorm.io/driver/sqlite` **静默忽略** `FOR UPDATE`（实测通过、不报错），且 SQLite 无行锁。**任何关于行锁/死锁的断言在 SQLite 上通过都不构成证据**，必须在真实 MySQL/PostgreSQL 上验证（见 Task 7）。

---

## File Structure

**新建：**

| 文件 | 职责 |
|---|---|
| `system/workflow/internal/model/workflow_applied_callback.go` | 回调幂等标记表 model |
| `system/workflow/internal/dao/workflow_applied_callback_dao.go` | 幂等标记的登记与清理 |
| `system/executor/internal/worker/internal_worker.go` | aio 进程内 outbox 回调消费者 |
| `pkg/core/config/workflow.go` | `workflow` 配置段 |

**修改：**

| 文件 | 改动 |
|---|---|
| `system/executor/api/callback/callback.go` | 新增 outbox 契约常量与 payload 结构 |
| `system/workflow/internal/app/engine.go` | 5 处事务入口改造；`triggerNode` 移入事务；幂等接入；`fmt.Printf` 清理 |
| `system/workflow/internal/app/app.go` | `App` 增加幂等 DAO 字段；`OnJobCompleted` 走带幂等的新入口 |
| `system/workflow/module.go` | 构造并注入幂等 DAO |
| `system/workflow/migrate.go` | 幂等表 AutoMigrate |
| `system/executor/internal/service/executor_job_service.go` | `AckJob` 包外层事务 + 提交 outbox job；新增回调派发入口 |
| `system/executor/module.go` | 启停内部 worker |
| `pkg/core/start/config.go` | `Config` 新增 `Workflow` 字段 |
| `main.go` | 清理 cron 增加幂等表清理 |
| `resources/config.yaml.example` | 补 `workflow.callback-mode` 示例 |

---

## Task 1: 幂等标记表（model + DAO + migrate + 清理接线）

**Files:**
- Create: `system/workflow/internal/model/workflow_applied_callback.go`
- Create: `system/workflow/internal/dao/workflow_applied_callback_dao.go`
- Create: `system/workflow/internal/dao/workflow_applied_callback_dao_test.go`
- Modify: `system/workflow/migrate.go`
- Modify: `system/workflow/module.go`
- Modify: `system/workflow/internal/app/app.go`
- Modify: `main.go:146-170`

**Interfaces:**
- Consumes: 无（首个任务）
- Produces:
  - `model.WorkflowAppliedCallbackModel`（表名 `aio_workflow_applied_callback`）
  - `dao.NewWorkflowAppliedCallbackDao(db *gorm.DB, log *logger.Log) *WorkflowAppliedCallbackDao`
  - `(*WorkflowAppliedCallbackDao).MarkApplied(ctx context.Context, tx *gorm.DB, jobID, instanceID int64, nodeID string) (applied bool, err error)`
  - `(*WorkflowAppliedCallbackDao).CleanupBefore(ctx context.Context, before time.Time) (int64, error)`
  - `app.App` 新增字段 `AppliedCallbackDao *dao.WorkflowAppliedCallbackDao`
  - `(*workflow.Module).CleanupAppliedCallbacks(ctx context.Context, before time.Time) (int64, error)`

- [ ] **Step 1: 写失败的测试**

创建 `system/workflow/internal/dao/workflow_applied_callback_dao_test.go`：

```go
package dao

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/xsxdot/aio/system/workflow/internal/model"
	"github.com/xsxdot/gokit/logger"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newAppliedCallbackTestDao(t *testing.T) (*WorkflowAppliedCallbackDao, *gorm.DB) {
	t.Helper()
	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.WorkflowAppliedCallbackModel{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return NewWorkflowAppliedCallbackDao(db, logger.GetLogger()), db
}

// 首次登记返回 true，同 jobID 再次登记返回 false —— 这是整个 outbox 重试安全的基石。
func TestMarkAppliedIsIdempotent(t *testing.T) {
	ctx := context.Background()
	d, db := newAppliedCallbackTestDao(t)

	applied, err := d.MarkApplied(ctx, db, 1001, 77, "E3_MicroShotInspect")
	if err != nil {
		t.Fatalf("first MarkApplied: %v", err)
	}
	if !applied {
		t.Fatal("first MarkApplied = false, want true")
	}

	applied, err = d.MarkApplied(ctx, db, 1001, 77, "E3_MicroShotInspect")
	if err != nil {
		t.Fatalf("second MarkApplied: %v", err)
	}
	if applied {
		t.Fatal("second MarkApplied = true, want false (duplicate must be rejected)")
	}

	var count int64
	if err := db.Model(&model.WorkflowAppliedCallbackModel{}).Where("job_id = ?", 1001).Count(&count).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("row count = %d, want 1", count)
	}
}

// 不同 jobID 互不影响
func TestMarkAppliedDistinctJobIDs(t *testing.T) {
	ctx := context.Background()
	d, db := newAppliedCallbackTestDao(t)

	for _, id := range []int64{1, 2, 3} {
		applied, err := d.MarkApplied(ctx, db, id, 77, "N")
		if err != nil {
			t.Fatalf("MarkApplied(%d): %v", id, err)
		}
		if !applied {
			t.Fatalf("MarkApplied(%d) = false, want true", id)
		}
	}
}

// 清理必须硬删：软删除的行仍占唯一索引，会让该 jobID 永久无法再登记。
func TestCleanupBeforeHardDeletesSoUniqueKeyIsFreed(t *testing.T) {
	ctx := context.Background()
	d, db := newAppliedCallbackTestDao(t)

	if _, err := d.MarkApplied(ctx, db, 2002, 88, "N"); err != nil {
		t.Fatalf("MarkApplied: %v", err)
	}
	// 把创建时间拨到过去，使其落入清理窗口
	if err := db.Model(&model.WorkflowAppliedCallbackModel{}).
		Where("job_id = ?", 2002).
		Update("created_at", time.Now().Add(-48*time.Hour)).Error; err != nil {
		t.Fatalf("backdate: %v", err)
	}

	n, err := d.CleanupBefore(ctx, time.Now().Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("CleanupBefore: %v", err)
	}
	if n != 1 {
		t.Fatalf("CleanupBefore deleted %d rows, want 1", n)
	}

	var raw int64
	if err := db.Unscoped().Model(&model.WorkflowAppliedCallbackModel{}).
		Where("job_id = ?", 2002).Count(&raw).Error; err != nil {
		t.Fatalf("raw count: %v", err)
	}
	if raw != 0 {
		t.Fatalf("row still present after cleanup (raw count = %d) — 说明用了软删除，唯一索引未释放", raw)
	}

	// 清理后同 jobID 必须能重新登记
	applied, err := d.MarkApplied(ctx, db, 2002, 88, "N")
	if err != nil {
		t.Fatalf("re-MarkApplied: %v", err)
	}
	if !applied {
		t.Fatal("re-MarkApplied = false, want true — 唯一索引未被清理释放")
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
go test ./system/workflow/internal/dao/ -run 'TestMarkApplied|TestCleanupBefore' -v
```

预期：编译失败，`undefined: NewWorkflowAppliedCallbackDao` / `undefined: model.WorkflowAppliedCallbackModel`。

- [ ] **Step 3: 创建 model**

创建 `system/workflow/internal/model/workflow_applied_callback.go`：

```go
package model

import (
	"github.com/xsxdot/aio/pkg/core/model/common"
)

// WorkflowAppliedCallbackModel 工作流回调幂等标记表。
//
// 职责：为「一次 executor 任务完成 → 一次工作流推进」建立唯一性约束，
// 使承载回调的 outbox 任务在重试时不会重复推进同一个节点。
//
// 边界：只做幂等判定，不承载任何业务状态，也不参与 DAG 推进决策。
// 不能用 aio_workflow_checkpoint 表代替——loopback 边有意允许同一节点
// 产生多条 checkpoint，(instance_id, node_id) 维度不唯一。
type WorkflowAppliedCallbackModel struct {
	common.Model
	JobID      int64  `gorm:"column:job_id;not null;uniqueIndex:idx_wac_job_id" json:"job_id" comment:"来源 executor 任务ID，幂等键"`
	InstanceID int64  `gorm:"column:instance_id;not null;index:idx_wac_instance" json:"instance_id" comment:"工作流实例ID"`
	NodeID     string `gorm:"column:node_id;size:100;not null" json:"node_id" comment:"节点ID"`
}

// TableName 指定表名
func (WorkflowAppliedCallbackModel) TableName() string {
	return "aio_workflow_applied_callback"
}
```

- [ ] **Step 4: 创建 DAO**

创建 `system/workflow/internal/dao/workflow_applied_callback_dao.go`：

```go
// Package dao 中本文件负责工作流回调幂等标记的持久化。
//
// 职责：登记「某个 executor 任务的完成回调已被应用」，并提供过期标记清理。
//
// 边界：不解释回调内容，不参与 DAG 推进；登记失败必须让调用方感知，
// 不做静默降级——静默降级会让重复回调穿透到状态推进逻辑。
package dao

import (
	"context"
	"time"

	"github.com/xsxdot/aio/pkg/core/mvc"
	"github.com/xsxdot/aio/system/workflow/internal/model"
	errorc "github.com/xsxdot/gokit/err"
	"github.com/xsxdot/gokit/logger"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type WorkflowAppliedCallbackDao struct {
	mvc.IBaseDao[model.WorkflowAppliedCallbackModel]
	log *logger.Log
	err *errorc.ErrorBuilder
	db  *gorm.DB
}

// NewWorkflowAppliedCallbackDao 创建回调幂等标记 DAO
func NewWorkflowAppliedCallbackDao(db *gorm.DB, log *logger.Log) *WorkflowAppliedCallbackDao {
	return &WorkflowAppliedCallbackDao{
		IBaseDao: mvc.NewGormDao[model.WorkflowAppliedCallbackModel](db),
		log:      log,
		err:      errorc.NewErrorBuilder("WorkflowAppliedCallbackDao"),
		db:       db,
	}
}

// MarkApplied 登记一次回调的应用记录。
//
// 参数：
//   - ctx: 上下文
//   - tx: 调用方所在事务；必须与后续状态推进使用同一事务，否则幂等无意义
//   - jobID: 来源 executor 任务 ID，幂等键
//   - instanceID / nodeID: 仅用于排查，不参与判重
//
// 返回：
//   - applied=true 表示本次登记成功（首次处理，调用方应继续推进）
//   - applied=false 表示该 jobID 已处理过（调用方必须跳过，直接视为成功）
//   - 数据库错误
//
// 注意：
//   - 用 ON CONFLICT DO NOTHING 而非「先查后插」——后者在并发下有 TOCTOU 窗口
//   - 用 ON CONFLICT 而非捕获唯一键错误——各方言错误码不同，捕获法不可移植
func (d *WorkflowAppliedCallbackDao) MarkApplied(ctx context.Context, tx *gorm.DB, jobID, instanceID int64, nodeID string) (bool, error) {
	rec := &model.WorkflowAppliedCallbackModel{
		JobID:      jobID,
		InstanceID: instanceID,
		NodeID:     nodeID,
	}
	res := tx.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "job_id"}},
			DoNothing: true,
		}).
		Create(rec)
	if res.Error != nil {
		d.log.WithErr(res.Error).
			WithField("job_id", jobID).
			WithField("instance_id", instanceID).
			WithField("node_id", nodeID).
			Error("登记回调幂等标记失败")
		return false, d.err.New("登记回调幂等标记失败", res.Error).DB().WithTraceID(ctx)
	}
	return res.RowsAffected > 0, nil
}

// CleanupBefore 硬删除 before 之前登记的幂等标记，返回删除行数。
//
// 注意：必须 Unscoped 硬删。common.Model 内嵌 gorm.DeletedAt，软删除的行
// 仍然占用 job_id 唯一索引，会导致该任务 ID 永远无法再次登记。
func (d *WorkflowAppliedCallbackDao) CleanupBefore(ctx context.Context, before time.Time) (int64, error) {
	res := mvc.ExtractDB(ctx, d.db).
		Unscoped().
		Where("created_at < ?", before).
		Delete(&model.WorkflowAppliedCallbackModel{})
	if res.Error != nil {
		d.log.WithErr(res.Error).WithField("before", before).Error("清理回调幂等标记失败")
		return 0, d.err.New("清理回调幂等标记失败", res.Error).DB().WithTraceID(ctx)
	}
	d.log.WithField("before", before).WithField("deleted", res.RowsAffected).Info("回调幂等标记清理完成")
	return res.RowsAffected, nil
}
```

- [ ] **Step 5: 运行测试确认通过**

```bash
go test ./system/workflow/internal/dao/ -run 'TestMarkApplied|TestCleanupBefore' -v
```

预期：3 个用例全部 PASS。

- [ ] **Step 6: 接入 migrate**

修改 `system/workflow/migrate.go`，在 `db.AutoMigrate(...)` 参数列表末尾追加：

```go
	err := db.AutoMigrate(
		&model.WorkflowDefModel{},
		&model.WorkflowInstanceModel{},
		&model.WorkflowCheckpointModel{},
		&model.WorkflowAppliedCallbackModel{},
	)
```

- [ ] **Step 7: 注入 DAO 到 App 与 Module**

修改 `system/workflow/internal/app/app.go`（当前 `App` 结构体见 `app.go:24-31`，`NewApp` 见 `app.go:34-39`）：

```go
type App struct {
	DefService        *service.WorkflowDefService
	InstanceService   *service.WorkflowInstanceService
	CheckpointService *service.WorkflowCheckpointService
	ExecutorClient    *executorClient.ExecutorClient
	// AppliedCallbackDao 回调幂等标记，仅供 ReportNodeCompletedFromJob 在推进事务内使用
	AppliedCallbackDao *dao.WorkflowAppliedCallbackDao
	log                *logger.Log
	err                *errorc.ErrorBuilder
}

func NewApp(
	defSvc *service.WorkflowDefService,
	instSvc *service.WorkflowInstanceService,
	cpSvc *service.WorkflowCheckpointService,
	execClient *executorClient.ExecutorClient,
	acDao *dao.WorkflowAppliedCallbackDao,
) *App {
	return &App{
		DefService:         defSvc,
		InstanceService:    instSvc,
		CheckpointService:  cpSvc,
		ExecutorClient:     execClient,
		AppliedCallbackDao: acDao,
		log:                logger.GetLogger().WithEntryName("WorkflowApp"),
		err:                errorc.NewErrorBuilder("WorkflowApp"),
	}
}
```

需新增 import `"github.com/xsxdot/aio/system/workflow/internal/dao"`。

修改 `system/workflow/module.go` 的 `NewModule`：

```go
	cpDao := dao.NewWorkflowCheckpointDao(db, log)
	acDao := dao.NewWorkflowAppliedCallbackDao(db, log)   // 新增

	defSvc := service.NewWorkflowDefService(defDao, log)
	instSvc := service.NewWorkflowInstanceService(instDao, log)
	cpSvc := service.NewWorkflowCheckpointService(cpDao, log)

	internalApp := app.NewApp(defSvc, instSvc, cpSvc, executorModule.Client, acDao)   // 末尾新增参数
```

并在 `module.go` 追加导出方法：

```go
// CleanupAppliedCallbacks 清理 before 之前的回调幂等标记，返回删除行数。
// 由 main.go 的每日清理任务调用；标记只用于防重放，过期后无保留价值。
func (m *Module) CleanupAppliedCallbacks(ctx context.Context, before time.Time) (int64, error) {
	return m.internalApp.AppliedCallbackDao.CleanupBefore(ctx, before)
}
```

（`module.go` 需补 `context` 与 `time` 的 import。）

- [ ] **Step 8: 接入每日清理 cron**

修改 `main.go` 的「任务执行器清理」任务体（`main.go:153-161`），在 `CleanupOldJobs` 之后追加幂等标记清理：

```go
		func(ctx context.Context) error {
			base.Logger.Info("开始执行任务执行器清理任务")
			_, err := appRoot.ExecutorModule.Client.CleanupOldJobs(ctx, base.ENV, 7, 30, 90)
			if err != nil {
				base.Logger.WithErr(err).Error("任务执行器清理任务执行失败")
				return err
			}
			// 幂等标记保留 7 天：outbox 回调任务最多重试 5 次、总窗口远小于 7 天，
			// 超过该窗口的标记不可能再被重放命中。
			deleted, err := appRoot.WorkflowModule.CleanupAppliedCallbacks(ctx, time.Now().AddDate(0, 0, -7))
			if err != nil {
				base.Logger.WithErr(err).Error("回调幂等标记清理失败")
				return err
			}
			base.Logger.WithField("applied_callback_deleted", deleted).Info("任务执行器清理任务执行完成")
			return nil
		},
```

- [ ] **Step 9: 全量编译与回归**

```bash
go build ./... && go test ./system/workflow/... ./system/executor/...
```

预期：编译通过，已有测试全绿。

- [ ] **Step 10: 提交**

```bash
git add system/workflow/internal/model/workflow_applied_callback.go \
        system/workflow/internal/dao/workflow_applied_callback_dao.go \
        system/workflow/internal/dao/workflow_applied_callback_dao_test.go \
        system/workflow/migrate.go system/workflow/module.go \
        system/workflow/internal/app/app.go main.go
git commit -m "feat(workflow): 新增回调幂等标记表与清理接线

为 outbox 回调重试提供幂等基础：ON CONFLICT DO NOTHING 登记，
Unscoped 硬删清理（软删除会永久占用唯一索引）。"
```

---

## Task 2: 事务入口改造（5 处 base.DB.Transaction → mvc.ExtractDB）

**Files:**
- Modify: `system/workflow/internal/app/engine.go:415,958,1003,1264,1341`
- Test: `system/workflow/internal/app/engine_tx_nesting_test.go`（新建）

**Interfaces:**
- Consumes: 无
- Produces: `engine.go` 中 5 个事务入口在 ctx 携带事务时复用该事务（SavePoint），无事务时行为不变。为 Task 3 的事务透传扫清死锁前提。

**背景（实现者必读）：** `ReportNodeCompleted` 在事务内对实例行加了 `FOR UPDATE`。Task 3 会把 `triggerNode` 移进这个事务，而 `triggerNode` 对 condition 节点会递归调用 `ReportNodeCompleted`、对 approval 节点会调用 `updateInstanceStatusToWaitingWithLock` —— 这两者若仍用 `base.DB.Transaction` 就会**另开一个数据库连接**去锁同一行，而外层事务不可能在内层返回前提交，形成死锁。本任务是 Task 3 的前置条件，必须先做。

- [ ] **Step 1: 写失败的测试**

创建 `system/workflow/internal/app/engine_tx_nesting_test.go`：

```go
package app

import (
	"context"
	"strings"
	"testing"

	"github.com/xsxdot/aio/base"
	"github.com/xsxdot/aio/pkg/core/mvc"
	"github.com/xsxdot/aio/system/workflow/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newTxNestingTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.WorkflowInstanceModel{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return db
}

// 验证 mvc.ExtractDB(ctx, base.DB).Transaction 在 ctx 已携带事务时复用该事务，
// 而不是另开一个——这是 Task 3 事务透传不死锁的机制前提。
//
// 注意：SQLite 无行锁且静默忽略 FOR UPDATE，本用例只能证明「机制上复用了同一
// 事务」，不能证明真实数据库上不死锁。后者见 Task 7 的真机验证。
func TestExtractDBTransactionReusesOuterTx(t *testing.T) {
	db := newTxNestingTestDB(t)
	prev := base.DB
	base.DB = db
	t.Cleanup(func() { base.DB = prev })

	inst := &model.WorkflowInstanceModel{
		DefID: 1, DefVersion: 1, Status: model.InstanceStatusRunning,
		InitialState: `{}`, CurrentState: `{}`, ActiveNodeIDs: `[]`,
	}
	if err := db.Create(inst).Error; err != nil {
		t.Fatalf("create instance: %v", err)
	}

	ctx := context.Background()
	var innerTxPtr, outerTxPtr *gorm.DB

	err := mvc.ExtractDB(ctx, base.DB).Transaction(func(outer *gorm.DB) error {
		outerTxPtr = outer
		txCtx := mvc.WithTxToContext(ctx, outer)
		// 模拟 Task 3 中 triggerNode 递归回到某个事务入口的情形
		return mvc.ExtractDB(txCtx, base.DB).Transaction(func(inner *gorm.DB) error {
			innerTxPtr = inner
			return inner.Model(&model.WorkflowInstanceModel{}).
				Where("id = ?", inst.ID).
				Update("status", model.InstanceStatusWaiting).Error
		})
	})
	if err != nil {
		t.Fatalf("nested transaction failed: %v", err)
	}
	if outerTxPtr == nil || innerTxPtr == nil {
		t.Fatal("transaction callbacks did not run")
	}
	if outerTxPtr.Statement.ConnPool != innerTxPtr.Statement.ConnPool {
		t.Fatal("内层事务未复用外层连接池 —— 说明另开了事务，真机上会死锁")
	}

	var got model.WorkflowInstanceModel
	if err := db.Where("id = ?", inst.ID).First(&got).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got.Status != model.InstanceStatusWaiting {
		t.Fatalf("status = %s, want %s（嵌套事务的写入未提交）", got.Status, model.InstanceStatusWaiting)
	}
}

// 无外层事务时，ExtractDB 必须退化为 base.DB，行为与改造前完全一致。
func TestExtractDBTransactionWithoutOuterTxStillCommits(t *testing.T) {
	db := newTxNestingTestDB(t)
	prev := base.DB
	base.DB = db
	t.Cleanup(func() { base.DB = prev })

	inst := &model.WorkflowInstanceModel{
		DefID: 1, DefVersion: 1, Status: model.InstanceStatusRunning,
		InitialState: `{}`, CurrentState: `{}`, ActiveNodeIDs: `[]`,
	}
	if err := db.Create(inst).Error; err != nil {
		t.Fatalf("create instance: %v", err)
	}

	err := mvc.ExtractDB(context.Background(), base.DB).Transaction(func(tx *gorm.DB) error {
		return tx.Model(&model.WorkflowInstanceModel{}).
			Where("id = ?", inst.ID).
			Update("status", model.InstanceStatusCompleted).Error
	})
	if err != nil {
		t.Fatalf("transaction failed: %v", err)
	}

	var got model.WorkflowInstanceModel
	if err := db.Where("id = ?", inst.ID).First(&got).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got.Status != model.InstanceStatusCompleted {
		t.Fatalf("status = %s, want %s", got.Status, model.InstanceStatusCompleted)
	}
}
```

- [ ] **Step 2: 运行测试**

```bash
go test ./system/workflow/internal/app/ -run TestExtractDBTransaction -v
```

预期：两个用例 PASS（测的是 `mvc` 既有能力，本步骤用于锁定机制契约）。若 `TestExtractDBTransactionReusesOuterTx` 失败，**停止**并报告——事务透传方案的前提不成立。

- [ ] **Step 3: 替换 5 处事务入口**

在 `system/workflow/internal/app/engine.go` 中，将下列 5 处的 `base.DB.Transaction(` 替换为 `mvc.ExtractDB(ctx, base.DB).Transaction(`：

| 行号 | 所在方法 |
|---|---|
| 415 | `ReportNodeCompleted` |
| 958 | `updateInstanceStatusToWaitingWithLock` |
| 1003 | `RollbackToNode` |
| 1264 | `RetryNode` |
| 1341 | `SendSignal` |

补 import：`"github.com/xsxdot/aio/pkg/core/mvc"`（若尚未引入）。

- [ ] **Step 4: 添加「为什么」注释**

在 `ReportNodeCompleted`（第 415 行）的事务入口上方加注释：

```go
	// 用 ExtractDB 而非 base.DB：triggerNode 对 condition 节点会递归回到本方法、
	// 对 approval 节点会进入 updateInstanceStatusToWaitingWithLock，而外层事务
	// 已对本实例行持有 FOR UPDATE。若这里另开事务，内层会等外层释放行锁、
	// 外层又在等内层返回，必然死锁。ExtractDB 复用外层事务（SavePoint），
	// 无外层事务时行为与 base.DB 完全一致。
	err := mvc.ExtractDB(ctx, base.DB).Transaction(func(tx *gorm.DB) error {
```

在 `updateInstanceStatusToWaitingWithLock`（第 958 行）加简注：

```go
	// 同 ReportNodeCompleted：本方法会被事务内的 triggerNode 调用，必须复用外层事务
	return mvc.ExtractDB(ctx, base.DB).Transaction(func(tx *gorm.DB) error {
```

其余三处（`RollbackToNode` / `RetryNode` / `SendSignal`）当前不会被嵌套调用，加统一简注：

```go
	// 统一走 ExtractDB：与其他事务入口保持一致，避免以后被移入事务后踩死锁
```

- [ ] **Step 5: 运行全量测试**

```bash
go build ./... && go test ./system/workflow/... -v
```

预期：编译通过，全部测试 PASS。

- [ ] **Step 6: 提交**

```bash
git add system/workflow/internal/app/engine.go system/workflow/internal/app/engine_tx_nesting_test.go
git commit -m "refactor(workflow): 5 处事务入口改用 mvc.ExtractDB 支持嵌套

为事务透传扫清死锁前提：triggerNode 移入事务后，condition 递归与
approval 分支会重入这些入口，另开事务必然与外层 FOR UPDATE 死锁。"
```

---

## Task 3: triggerNode 移入事务（边界2 消灭）

**Files:**
- Modify: `system/workflow/internal/app/engine.go:432`（`fmt.Printf` 清理）
- Modify: `system/workflow/internal/app/engine.go:700-752`（事务边界）
- Test: `system/workflow/internal/app/engine_trigger_atomicity_test.go`（新建）

**Interfaces:**
- Consumes: Task 2 的 `mvc.ExtractDB` 事务入口
- Produces: `ReportNodeCompleted` 在触发下游失败时整体回滚（实例状态、checkpoint、下游任务全部不落库），不再把实例置为 FAILED。

- [ ] **Step 1: 写失败的测试**

创建 `system/workflow/internal/app/engine_trigger_atomicity_test.go`：

```go
package app

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/xsxdot/aio/base"
	"github.com/xsxdot/aio/pkg/core/mvc"
	"github.com/xsxdot/aio/system/workflow/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// 验证「状态推进 + 下游任务提交」的原子性：模拟下游提交失败，
// 整个事务必须回滚，实例状态与 checkpoint 都不得落库。
func TestAdvanceRollsBackWhenDownstreamSubmitFails(t *testing.T) {
	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&model.WorkflowInstanceModel{},
		&model.WorkflowCheckpointModel{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	prev := base.DB
	base.DB = db
	t.Cleanup(func() { base.DB = prev })

	inst := &model.WorkflowInstanceModel{
		DefID: 1, DefVersion: 1, Status: model.InstanceStatusRunning,
		InitialState: `{"data":{},"_sys":{}}`,
		CurrentState: `{"data":{},"_sys":{}}`,
		ActiveNodeIDs: `["A"]`,
	}
	if err := db.Create(inst).Error; err != nil {
		t.Fatalf("create instance: %v", err)
	}

	ctx := context.Background()
	sentinel := errors.New("submit downstream failed")

	// 直接验证事务语义：在事务内写状态与 checkpoint，然后模拟下游提交失败返回错误
	err = mvc.ExtractDB(ctx, base.DB).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.WorkflowInstanceModel{}).
			Where("id = ?", inst.ID).
			Update("active_node_ids", `["B"]`).Error; err != nil {
			return err
		}
		if err := tx.Create(&model.WorkflowCheckpointModel{
			InstanceID: inst.ID, NodeID: "A",
			NodeOutput: `{}`, StateAfter: `{"data":{},"_sys":{}}`,
		}).Error; err != nil {
			return err
		}
		return sentinel // 模拟 triggerNode 失败
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("err = %v, want sentinel", err)
	}

	var got model.WorkflowInstanceModel
	if err := db.Where("id = ?", inst.ID).First(&got).Error; err != nil {
		t.Fatalf("reload instance: %v", err)
	}
	if got.ActiveNodeIDs != `["A"]` {
		t.Fatalf("ActiveNodeIDs = %s, want [\"A\"] — 状态未回滚", got.ActiveNodeIDs)
	}
	if got.Status != model.InstanceStatusRunning {
		t.Fatalf("Status = %s, want RUNNING — 不应被置为 FAILED", got.Status)
	}

	var cpCount int64
	if err := db.Model(&model.WorkflowCheckpointModel{}).
		Where("instance_id = ?", inst.ID).Count(&cpCount).Error; err != nil {
		t.Fatalf("count checkpoints: %v", err)
	}
	if cpCount != 0 {
		t.Fatalf("checkpoint count = %d, want 0 — checkpoint 未回滚", cpCount)
	}
}
```

- [ ] **Step 2: 运行测试确认失败或通过**

```bash
go test ./system/workflow/internal/app/ -run TestAdvanceRollsBack -v
```

预期：PASS（本用例锁定的是事务语义契约，为下一步的结构改动提供回归保护）。

- [ ] **Step 3: 把 triggerNode 移入事务**

修改 `system/workflow/internal/app/engine.go`。把当前事务闭包末尾的 `return nil`（约第 729 行，紧跟 `tx.Save(&instance)` 之后）替换为下游触发逻辑：

```go
		if err := tx.Save(&instance).Error; err != nil {
			return err
		}

		// 事务透传：ExecutorClient 是进程内直调、executor DAO 全线走 mvc.ExtractDB，
		// 因此把 tx 放进 ctx 后，下游任务的 INSERT 与本次状态推进落在同一个事务。
		// 这消除了「状态已提交、任务未入库」的崩溃窗口——此前该窗口会让实例停在
		// RUNNING 且 ActiveNodeIDs 非空，但没有任何 executor job 存在，永久卡死。
		txCtx := mvc.WithTxToContext(ctx, tx)
		for _, nextNodeID := range nextNodeIDs {
			node := dag.GetNode(nextNodeID)
			if node == nil {
				continue
			}
			if err := a.triggerNode(txCtx, &instance, node, &dag, env); err != nil {
				a.log.WithErr(err).
					WithField("instance_id", instance.ID).
					WithField("node_id", node.ID).
					Error("触发后续节点失败，整体回滚本次推进")
				// 返回错误让整个事务回滚：要么状态推进且下游已入库，要么什么都没发生。
				// 由承载本次回调的 outbox 任务重试兜底，不再需要把实例置 FAILED。
				return err
			}
		}
		return nil
	})

	if err != nil {
		return a.err.New("完成节点流转失败", err)
	}

	a.log.WithField("instance_id", instanceID).
		WithField("node_id", nodeID).
		WithField("next_nodes", nextNodeIDs).
		Info("节点完成，工作流推进成功")
	return nil
}
```

同时**删除**原事务外的整段触发循环与防假死补救分支（原 `engine.go:735-751`，即 `// TX 提交成功后，触发新节点` 起至该 `for` 循环结束）。

- [ ] **Step 4: 清理 fmt.Printf**

修改 `system/workflow/internal/app/engine.go:432`：

```go
			a.log.WithField("instance_id", instanceID).
				WithField("node_id", nodeID).
				WithField("status", instance.Status).
				Info("实例已结束，仅记录迟到的成功回调")
```

替换原 `fmt.Printf("[TRACE-REPORT-LATE-SUCCESS] ...")`。若 `fmt` 在文件内已无其他用途则移除该 import。

- [ ] **Step 5: 运行全量测试**

```bash
go build ./... && go test ./system/workflow/... ./system/executor/...
```

预期：编译通过，全部 PASS。

- [ ] **Step 6: 提交**

```bash
git add system/workflow/internal/app/engine.go system/workflow/internal/app/engine_trigger_atomicity_test.go
git commit -m "fix(workflow): 下游任务派发移入推进事务，消除崩溃窗口

此前状态提交后、triggerNode 执行前崩溃会让实例停在 RUNNING 且
ActiveNodeIDs 非空但无任何 executor job，永久卡死且界面显示正常运行。
现在两者同事务提交，失败整体回滚交由 outbox 重试。"
```

---

## Task 4: ReportNodeCompletedFromJob（幂等接入）

**Files:**
- Modify: `system/workflow/internal/app/engine.go`（新增导出方法 + 幂等登记）
- Modify: `system/workflow/internal/app/app.go:143-169`（`OnJobCompleted` 改走新入口）
- Test: `system/workflow/internal/app/engine_idempotency_test.go`（新建）

**Interfaces:**
- Consumes: Task 1 的 `AppliedCallbackDao.MarkApplied`
- Produces:
  - `(*App).ReportNodeCompletedFromJob(ctx context.Context, jobID int64, instanceID int64, nodeID string, output map[string]interface{}, env string, subJobID ...int) error`
  - 现有 `ReportNodeCompleted(ctx, instanceID, nodeID, output, env, subJobID...)` 签名与行为不变（内部委托给 `jobID=0` 的实现）

- [ ] **Step 1: 写失败的测试**

创建 `system/workflow/internal/app/engine_idempotency_test.go`：

```go
package app

import (
	"context"
	"strings"
	"testing"

	"github.com/xsxdot/aio/system/workflow/internal/dao"
	"github.com/xsxdot/aio/system/workflow/internal/model"
	"github.com/xsxdot/gokit/logger"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// 同一个 jobID 登记两次，第二次必须被判定为已处理。
// 这是 outbox 回调重试不污染状态（尤其 append 模式重复追加）的唯一防线。
func TestAppliedCallbackGuardRejectsReplay(t *testing.T) {
	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.WorkflowAppliedCallbackModel{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	acDao := dao.NewWorkflowAppliedCallbackDao(db, logger.GetLogger())
	ctx := context.Background()

	applied, err := acDao.MarkApplied(ctx, db, 5001, 42, "E3_MicroShotInspect")
	if err != nil || !applied {
		t.Fatalf("first MarkApplied: applied=%v err=%v", applied, err)
	}
	applied, err = acDao.MarkApplied(ctx, db, 5001, 42, "E3_MicroShotInspect")
	if err != nil {
		t.Fatalf("second MarkApplied: %v", err)
	}
	if applied {
		t.Fatal("重放未被拦截：applied=true，append 模式的状态会被重复追加")
	}
}

// applyStateReducer 的 append 模式确实会在重放时重复追加 ——
// 本用例固化「为什么必须有幂等」这一事实，防止后人误删幂等表。
func TestAppendModeIsNotNaturallyIdempotent(t *testing.T) {
	data := workflowStateData{}
	nodeConfig := map[string]interface{}{
		"state_update_mode": "append",
		"output_key":        "shots",
	}
	output := map[string]interface{}{"index": 1}

	applyStateReducer(data, output, nodeConfig)
	applyStateReducer(data, output, nodeConfig)

	arr, ok := data["shots"].([]interface{})
	if !ok {
		t.Fatalf("data[\"shots\"] type = %T, want []interface{}", data["shots"])
	}
	if len(arr) != 2 {
		t.Fatalf("len = %d, want 2 —— 若为 1 说明 reducer 已幂等，本计划的幂等表需重新评估", len(arr))
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
go test ./system/workflow/internal/app/ -run 'TestAppliedCallbackGuard|TestAppendModeIsNot' -v
```

预期：`TestAppliedCallbackGuardRejectsReplay` 编译通过并 PASS（依赖 Task 1）；`TestAppendModeIsNotNaturallyIdempotent` PASS，证实 append 非幂等。

- [ ] **Step 3: 实现 ReportNodeCompletedFromJob**

在 `system/workflow/internal/app/engine.go` 中，把现有 `ReportNodeCompleted` 重命名为内部方法并新增两个入口：

```go
// ReportNodeCompleted 上报节点完成并推进工作流（无幂等保护的入口）。
//
// 参数：
//   - instanceID / nodeID: 目标实例与节点
//   - output: 节点输出；含 error_msg 键时走 error 边
//   - env: Executor 任务隔离环境，空则取实例的 Env
//   - subJobID: Map 子任务序号，非 Map 场景不传
//
// 注意：供 condition 节点递归与人工审批推进使用，这两类调用不来自可重试
// 载体，无需幂等。来自 executor 回调的推进必须用 ReportNodeCompletedFromJob。
func (a *App) ReportNodeCompleted(ctx context.Context, instanceID int64, nodeID string, output map[string]interface{}, env string, subJobID ...int) error {
	return a.reportNodeCompleted(ctx, 0, instanceID, nodeID, output, env, subJobID...)
}

// ReportNodeCompletedFromJob 上报由 executor 任务触发的节点完成，带幂等保护。
//
// 参数：
//   - jobID: 触发本次回调的 executor 任务 ID，用作幂等键；必须 > 0
//   - 其余参数同 ReportNodeCompleted
//
// 返回：
//   - nil 表示本次推进成功，或该 jobID 已被处理过（重复回调，调用方应视为成功）
//   - 非 nil 表示推进失败，调用方应让承载本次回调的任务重试
//
// 注意：幂等登记与状态推进在同一事务内完成，因此「登记成功但推进失败」
// 不可能发生——两者一起回滚。
func (a *App) ReportNodeCompletedFromJob(ctx context.Context, jobID int64, instanceID int64, nodeID string, output map[string]interface{}, env string, subJobID ...int) error {
	if jobID <= 0 {
		return a.err.New("ReportNodeCompletedFromJob 要求 jobID > 0", nil).WithTraceID(ctx)
	}
	return a.reportNodeCompleted(ctx, jobID, instanceID, nodeID, output, env, subJobID...)
}

func (a *App) reportNodeCompleted(ctx context.Context, jobID int64, instanceID int64, nodeID string, output map[string]interface{}, env string, subJobID ...int) error {
	// ... 原 ReportNodeCompleted 方法体 ...
}
```

- [ ] **Step 4: 在事务开头插入幂等登记**

在 `reportNodeCompleted` 的事务闭包**最开头**（`tx.Clauses(clause.Locking{...})` 加锁之前）插入：

```go
	var skippedAsDuplicate bool
	err := mvc.ExtractDB(ctx, base.DB).Transaction(func(tx *gorm.DB) error {
		// 幂等登记必须在推进逻辑之前、且与推进同事务：承载回调的 outbox 任务
		// 会重试，而 applyStateReducer 的 append 模式重放会重复追加输出，
		// 造成状态污染（nova 的 Map 节点大量使用该模式汇聚结果）。
		if jobID > 0 {
			applied, mErr := a.AppliedCallbackDao.MarkApplied(ctx, tx, jobID, instanceID, nodeID)
			if mErr != nil {
				return mErr
			}
			if !applied {
				skippedAsDuplicate = true
				return nil
			}
		}

		// ... 原有推进逻辑 ...
	})

	if err != nil {
		return a.err.New("完成节点流转失败", err)
	}
	if skippedAsDuplicate {
		// 必须打日志：否则重复回调静默发生，排查时完全看不见。
		a.log.WithField("job_id", jobID).
			WithField("instance_id", instanceID).
			WithField("node_id", nodeID).
			Info("回调重复，命中幂等标记已跳过")
		return nil
	}
```

- [ ] **Step 5: OnJobCompleted 改走新入口**

修改 `system/workflow/internal/app/app.go:166`，把：

```go
	if err := a.ReportNodeCompleted(ctx, instanceID, nodeID, output, callbackEnv, subJobID); err != nil {
```

改为：

```go
	if err := a.ReportNodeCompletedFromJob(ctx, int64(jobID), instanceID, nodeID, output, callbackEnv, subJobID); err != nil {
```

并把该方法内原本「记日志后吞掉错误」的处理改为向上返回错误——`OnJobCompleted` 现在运行在可重试的 outbox 任务里，吞错误等于放弃重试。相应修改 `callback.JobCompletionHandler` 接口：

```go
// JobCompletionHandler 任务完成处理器。
//
// 注意：返回错误表示本次回调未成功应用，承载该回调的 outbox 任务会重试；
// 实现方必须保证按同一 jobID 重复调用是安全的（幂等）。
type JobCompletionHandler interface {
	OnJobCompleted(ctx context.Context, jobID uint64, callbackData, resultJSON string) error
}
```

同步修改 `system/executor/internal/service/executor_job_service.go` 中两处 `handler.OnJobCompleted(...)` 调用点以处理返回值（Task 5 会重构这段，此处先让其编译通过：记录错误日志）。

- [ ] **Step 6: 添加日志**

在 `reportNodeCompleted` 补齐关键节点日志：

- 入口：`Debug`，字段 `job_id` / `instance_id` / `node_id` / `sub_job_id`
- 幂等命中跳过：`Info`（见 Step 4）
- 事务失败：`Error`，带 `instance_id` / `node_id` / 完整 cause
- 成功退出：`Info`，带 `instance_id` / `node_id` / `next_nodes`（Task 3 已加）

- [ ] **Step 7: 运行全量测试**

```bash
go build ./... && go test ./system/workflow/... ./system/executor/...
```

预期：编译通过，全部 PASS。

- [ ] **Step 8: 提交**

```bash
git add system/workflow/internal/app/engine.go system/workflow/internal/app/app.go \
        system/workflow/internal/app/engine_idempotency_test.go \
        system/executor/api/callback/callback.go \
        system/executor/internal/service/executor_job_service.go
git commit -m "feat(workflow): 新增带幂等保护的回调推进入口

ReportNodeCompletedFromJob 在推进事务内登记 jobID 幂等标记，重复回调
直接跳过。JobCompletionHandler 改为返回 error，让失败能触发重试。"
```

---

## Task 5: 配置项 + AckJob 提交 outbox（边界1 服务端）

**Files:**
- Create: `pkg/core/config/workflow.go`
- Modify: `pkg/core/start/config.go`
- Modify: `resources/config.yaml.example`
- Modify: `system/executor/api/callback/callback.go`
- Modify: `system/executor/internal/service/executor_job_service.go:339-388`
- Test: `system/executor/internal/service/executor_job_ack_outbox_test.go`（新建）

**Interfaces:**
- Consumes: Task 4 的 `JobCompletionHandler.OnJobCompleted(...) error`
- Produces:
  - `config.WorkflowConfig{ CallbackMode string }`
  - `callback.InternalTargetService = "aio"`、`callback.MethodJobCompletedCallback = "internal.job_completed_callback"`
  - `callback.CallbackPayload{ JobID uint64; Source, CallbackData, ResultJSON string }`
  - `(*ExecutorJobService).DispatchCompletionCallback(ctx context.Context, source string, jobID uint64, callbackData, resultJSON string) error`

- [ ] **Step 1: 写失败的测试**

创建 `system/executor/internal/service/executor_job_ack_outbox_test.go`：

```go
package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/xsxdot/aio/system/executor/api/callback"
	"github.com/xsxdot/aio/system/executor/internal/dao"
	"github.com/xsxdot/aio/system/executor/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newAckOutboxTestService(t *testing.T) (*ExecutorJobService, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.ExecutorJobModel{}, &model.ExecutorJobAttemptModel{}); err != nil {
		t.Fatal(err)
	}
	d := dao.NewExecutorJobDAOWithDB(db)
	// err 字段由 Task 5 Step 5 新增；struct 字面量构造必须显式赋值，否则
	// 错误分支上 nil ErrorBuilder 会 panic。
	return &ExecutorJobService{
		dao:      d,
		handlers: make(map[string]callback.JobCompletionHandler),
		err:      errorc.NewErrorBuilder("ExecutorJobService"),
	}, db
}

// AckJob 成功且 job.Source 非空时，必须在同一事务内产生一条 outbox 回调任务。
func TestAckJobCreatesOutboxCallbackJob(t *testing.T) {
	ctx := context.Background()
	s, db := newAckOutboxTestService(t)

	now := time.Now()
	until := now.Add(time.Minute)
	job := &model.ExecutorJobModel{
		Env: "dev", TargetService: "tk-server", Method: "split_video",
		Status: model.JobStatusRunning, Attempts: 1, MaxAttempts: 3,
		DedupKey: "wf_1_node_A_1", Source: "workflow",
		CallbackData: `{"instance_id":1,"node_id":"A","env":"dev"}`,
		LeaseOwner:   "c-1", LeaseUntil: &until, NextRunAt: &now,
	}
	if err := db.Create(job).Error; err != nil {
		t.Fatal(err)
	}

	if err := s.AckJob(ctx, uint64(job.ID), 1, "c-1",
		model.JobStatusSucceeded, "", `{"ok":true}`, 0, false, 0, ""); err != nil {
		t.Fatalf("AckJob: %v", err)
	}

	var outbox model.ExecutorJobModel
	wantDedup := callback.OutboxDedupKeyPrefix + strconv.FormatInt(job.ID, 10)
	if err := db.Where("dedup_key = ?", wantDedup).First(&outbox).Error; err != nil {
		t.Fatalf("outbox job 未创建 (dedup_key=%s): %v", wantDedup, err)
	}
	if outbox.TargetService != callback.InternalTargetService {
		t.Fatalf("TargetService = %q, want %q", outbox.TargetService, callback.InternalTargetService)
	}
	if outbox.Method != callback.MethodJobCompletedCallback {
		t.Fatalf("Method = %q, want %q", outbox.Method, callback.MethodJobCompletedCallback)
	}
	// Source 必须为空，否则这条回调任务自身 Ack 时又会生成回调，无限递归
	if outbox.Source != "" {
		t.Fatalf("outbox.Source = %q, want \"\" —— 非空会导致回调无限递归", outbox.Source)
	}

	var payload callback.CallbackPayload
	if err := json.Unmarshal([]byte(outbox.ArgsJSON), &payload); err != nil {
		t.Fatalf("解析 ArgsJSON: %v", err)
	}
	if payload.JobID != uint64(job.ID) || payload.Source != "workflow" {
		t.Fatalf("payload = %+v, want JobID=%d Source=workflow", payload, job.ID)
	}
	if payload.ResultJSON != `{"ok":true}` {
		t.Fatalf("payload.ResultJSON = %q", payload.ResultJSON)
	}
}

// job.Source 为空时不得产生 outbox 任务
func TestAckJobWithoutSourceCreatesNoOutbox(t *testing.T) {
	ctx := context.Background()
	s, db := newAckOutboxTestService(t)

	now := time.Now()
	until := now.Add(time.Minute)
	job := &model.ExecutorJobModel{
		Env: "dev", TargetService: "tk-server", Method: "split_video",
		Status: model.JobStatusRunning, Attempts: 1, MaxAttempts: 3,
		DedupKey: "plain_1", Source: "",
		LeaseOwner: "c-1", LeaseUntil: &until, NextRunAt: &now,
	}
	if err := db.Create(job).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.AckJob(ctx, uint64(job.ID), 1, "c-1",
		model.JobStatusSucceeded, "", `{}`, 0, false, 0, ""); err != nil {
		t.Fatalf("AckJob: %v", err)
	}

	var count int64
	if err := db.Model(&model.ExecutorJobModel{}).
		Where("method = ?", callback.MethodJobCompletedCallback).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("outbox job count = %d, want 0", count)
	}
}
```

import 块需包含：`context`、`encoding/json`、`strconv`、`testing`、`time`、`github.com/xsxdot/aio/system/executor/api/callback`、`github.com/xsxdot/aio/system/executor/internal/dao`、`github.com/xsxdot/aio/system/executor/internal/model`、`errorc "github.com/xsxdot/gokit/err"`、`gorm.io/driver/sqlite`、`gorm.io/gorm`。

- [ ] **Step 2: 运行测试确认失败**

```bash
go test ./system/executor/internal/service/ -run TestAckJob -v
```

预期：编译失败，`undefined: callback.InternalTargetService`。

- [ ] **Step 3: 扩展 callback 契约**

修改 `system/executor/api/callback/callback.go`：

```go
// Package callback 定义任务完成回调的对外契约。
//
// 职责：声明回调处理器接口，以及承载回调的 outbox 任务的固定字段与载荷结构。
//
// 边界：不含任何实现，仅为 executor 内部服务与消费者共享的契约，
// 放在 api/ 下以避免 service 与 worker 互相 import 形成环。
package callback

import "context"

const (
	// InternalTargetService outbox 回调任务的目标服务名，固定为 aio 自身。
	InternalTargetService = "aio"
	// MethodJobCompletedCallback outbox 回调任务的方法名。
	MethodJobCompletedCallback = "internal.job_completed_callback"
	// OutboxDedupKeyPrefix outbox 回调任务的幂等键前缀，完整形式为 {prefix}{jobID}。
	OutboxDedupKeyPrefix = "jobcb_"
	// OutboxMaxAttempts outbox 回调任务的最大尝试次数，超过后转死信、Admin 可见。
	OutboxMaxAttempts = 5
	// OutboxPriority outbox 回调任务的优先级，高于普通业务任务以尽快被领取。
	OutboxPriority = 10
)

// CallbackPayload 是 outbox 回调任务 ArgsJSON 的结构。
type CallbackPayload struct {
	JobID        uint64 `json:"job_id"`
	Source       string `json:"source"`
	CallbackData string `json:"callback_data"`
	ResultJSON   string `json:"result_json"`
}

// JobCompletionHandler 任务完成处理器（供 Workflow 等组件实现，按 Source 注册）。
//
// 注意：返回错误表示本次回调未成功应用，承载它的 outbox 任务会重试；
// 实现方必须保证按同一 jobID 重复调用是安全的（幂等）。
type JobCompletionHandler interface {
	OnJobCompleted(ctx context.Context, jobID uint64, callbackData, resultJSON string) error
}
```

- [ ] **Step 4: 新增配置**

创建 `pkg/core/config/workflow.go`：

```go
package config

// WorkflowConfig 工作流模块配置。
type WorkflowConfig struct {
	// CallbackMode 任务完成回调的投递方式。
	//
	// 取值：
	//   - ""（默认）/ "outbox"：AckJob 事务内提交 outbox 任务，由 aio 内部 worker 消费，失败可重试
	//   - "sync"：退回改造前的同步进程内回调，失败即丢失
	//
	// 刻意用字符串而非 bool：struct 反序列化时 bool 零值为 false，
	// 若定义成「默认 true 的 bool」，配置缺失反而会得到 false。
	// 字符串空值语义明确——未配置即新行为，退回必须显式声明。
	//
	// 这是过渡开关，稳定运行一个版本后应连同 sync 分支一并删除。
	CallbackMode string `yaml:"callback-mode" json:"callback-mode"`
}
```

修改 `pkg/core/start/config.go`，在 `Config` 结构体末尾追加字段：

```go
	Workflow     config.WorkflowConfig     `yaml:"workflow"`
```

修改 `resources/config.yaml.example`，追加：

```yaml
# 工作流模块
workflow:
  # 任务完成回调投递方式：留空或 outbox（默认，可重试）；sync 退回同步回调（过渡用，将移除）
  callback-mode: ""
```

- [ ] **Step 5: 改造 AckJob**

修改 `system/executor/internal/service/executor_job_service.go`，把 `AckJob` 整体重写为：

```go
// AckJob 确认任务执行结果。
//
// 参数：
//   - jobID / attemptNo / consumerID: 定位任务与校验租约归属
//   - status: 本次执行结果
//   - errorMsg / errorType / resultJSON: 执行输出
//   - retryAfter / stopRetry / addMaxAttempts: 重试控制
//
// 返回：确认失败或 outbox 提交失败时返回错误，调用方应重试整个 Ack。
//
// 注意：状态更新与回调 outbox 任务在同一事务内提交，因此不存在
// 「任务已终态但回调丢失」的窗口——这正是本方法改造前的永久卡死成因。
func (s *ExecutorJobService) AckJob(ctx context.Context, jobID uint64, attemptNo int32, consumerID string,
	status model.JobStatus, errorMsg, resultJSON string, retryAfter int32,
	stopRetry bool, addMaxAttempts int32, errorType string) error {

	return mvc.ExtractDB(ctx, base.DB).Transaction(func(tx *gorm.DB) error {
		txCtx := mvc.WithTxToContext(ctx, tx)

		job, preErr := s.dao.GetByID(txCtx, jobID)
		var source, callbackData, env string
		if preErr == nil && job != nil {
			source, callbackData, env = job.Source, job.CallbackData, job.Env
		}

		if err := s.dao.AckJob(txCtx, jobID, attemptNo, consumerID, status,
			errorMsg, resultJSON, retryAfter, stopRetry, addMaxAttempts, errorType); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return s.err.New("任务不存在或租约信息不匹配", err).WithTraceID(ctx)
			}
			return err
		}

		base.Logger.WithField("job_id", jobID).WithField("status", status).Info("任务确认成功")

		if source == "" {
			return nil
		}

		payloadJSON := resultJSON
		needCallback := false
		switch status {
		case model.JobStatusSucceeded:
			needCallback = true
		case model.JobStatusFailed:
			// 重试耗尽转死信时，Workflow 需要收到回调以走 error 边。
			// 必须在同事务内回读——DAO 刚写入的状态尚未提交，走 base.DB 读不到。
			after, aErr := s.dao.GetByID(txCtx, jobID)
			if aErr == nil && after != nil && after.Status == model.JobStatusDead {
				needCallback = true
				b, _ := json.Marshal(map[string]string{"error_msg": errorMsg})
				payloadJSON = string(b)
			}
		}
		if !needCallback {
			return nil
		}

		if !callbackViaOutbox() {
			// 过渡开关：退回改造前的同步回调。稳定后连同本分支一并删除。
			s.mu.RLock()
			handler := s.handlers[source]
			s.mu.RUnlock()
			if handler == nil {
				return nil
			}
			if cbErr := handler.OnJobCompleted(ctx, jobID, callbackData, payloadJSON); cbErr != nil {
				base.Logger.WithErr(cbErr).WithField("job_id", jobID).WithField("source", source).
					Error("同步回调失败（sync 模式无重试，工作流可能卡死）")
			}
			return nil
		}

		return s.submitCallbackOutbox(txCtx, env, source, jobID, callbackData, payloadJSON)
	})
}

// submitCallbackOutbox 在当前事务内提交一条承载完成回调的 outbox 任务。
//
// 注意：Source 必须留空。若非空，这条回调任务自身 Ack 时会再次生成回调任务，
// 形成无限递归。
func (s *ExecutorJobService) submitCallbackOutbox(ctx context.Context, env, source string,
	jobID uint64, callbackData, resultJSON string) error {

	payload := callback.CallbackPayload{
		JobID:        jobID,
		Source:       source,
		CallbackData: callbackData,
		ResultJSON:   resultJSON,
	}
	argsJSON, err := json.Marshal(payload)
	if err != nil {
		return s.err.New("序列化回调载荷失败", err).WithTraceID(ctx)
	}

	dedupKey := callback.OutboxDedupKeyPrefix + strconv.FormatUint(jobID, 10)
	if _, err := s.SubmitJob(ctx, &dto.SubmitJobInput{
		Env:              env,
		TargetService:    callback.InternalTargetService,
		Method:           callback.MethodJobCompletedCallback,
		ArgsJSON:         string(argsJSON),
		MaxAttempts:      callback.OutboxMaxAttempts,
		Priority:         callback.OutboxPriority,
		DedupKey:         dedupKey,
		RetryBackoffType: dto.RetryBackoffExponential,
		Source:           "",
	}); err != nil {
		base.Logger.WithErr(err).WithField("job_id", jobID).WithField("source", source).
			Error("提交回调 outbox 任务失败，本次 Ack 将整体回滚")
		return err
	}

	base.Logger.WithField("job_id", jobID).
		WithField("source", source).
		WithField("dedup_key", dedupKey).
		Info("回调 outbox 任务已入队")
	return nil
}

// callbackViaOutbox 判定是否走 outbox 回调。
//
// 注意：配置缺失（含测试环境 base.Configures 为 nil）时返回 true——
// 未配置即新行为，退回同步回调必须显式配置 workflow.callback-mode: sync。
func callbackViaOutbox() bool {
	if base.Configures == nil {
		return true
	}
	return base.Configures.Config.Workflow.CallbackMode != "sync"
}

// DispatchCompletionCallback 按 source 路由到已注册的完成处理器。
//
// 参数：source 为提交任务时指定的来源标识；jobID 为原始任务 ID。
//
// 返回：未注册对应 source 时返回错误（让 outbox 任务重试，
// 避免注册时序问题导致回调被静默丢弃）。
func (s *ExecutorJobService) DispatchCompletionCallback(ctx context.Context, source string,
	jobID uint64, callbackData, resultJSON string) error {

	s.mu.RLock()
	handler := s.handlers[source]
	s.mu.RUnlock()
	if handler == nil {
		return s.err.New("未注册 source 对应的完成处理器: "+source, nil).WithTraceID(ctx)
	}
	return handler.OnJobCompleted(ctx, jobID, callbackData, resultJSON)
}
```

补齐 import：`encoding/json`、`strconv`、`gorm.io/gorm`、`github.com/xsxdot/aio/pkg/core/mvc`、`github.com/xsxdot/aio/system/executor/api/dto`、`errorc "github.com/xsxdot/gokit/err"`。

`ExecutorJobService` 当前**没有** `err` 字段（见 `executor_job_service.go:29-33`），必须新增：

```go
type ExecutorJobService struct {
	dao      *dao.ExecutorJobDAO
	handlers map[string]callback.JobCompletionHandler
	mu       sync.RWMutex
	err      *errorc.ErrorBuilder
}

func NewExecutorJobService() *ExecutorJobService {
	return &ExecutorJobService{
		dao:      dao.NewExecutorJobDAO(),
		handlers: make(map[string]callback.JobCompletionHandler),
		err:      errorc.NewErrorBuilder("ExecutorJobService"),
	}
}
```

同时把该文件内已有的 `errors.New("任务不存在或租约信息不匹配")` 等业务错误改用 `s.err.New(...)`（`requireEnv` 等纯参数校验保持原样，不在本次范围）。

- [ ] **Step 6: 运行测试确认通过**

```bash
go test ./system/executor/internal/service/ -run TestAckJob -v
```

预期：两个用例 PASS。

- [ ] **Step 7: 运行全量测试**

```bash
go build ./... && go test ./system/executor/... ./system/workflow/...
```

- [ ] **Step 8: 提交**

```bash
git add pkg/core/config/workflow.go pkg/core/start/config.go resources/config.yaml.example \
        system/executor/api/callback/callback.go \
        system/executor/internal/service/executor_job_service.go \
        system/executor/internal/service/executor_job_ack_outbox_test.go
git commit -m "feat(executor): AckJob 事务内提交回调 outbox 任务

状态更新与回调投递同事务，消除「任务已终态但回调丢失」窗口。
新增 workflow.callback-mode 过渡开关（sync 可退回同步回调）。"
```

---

## Task 6: 内部 worker（边界1 消费端）

**Files:**
- Create: `system/executor/internal/worker/internal_worker.go`
- Create: `system/executor/internal/worker/internal_worker_test.go`
- Modify: `system/executor/module.go`
- Modify: `main.go`（注册优雅停止）

**Interfaces:**
- Consumes: Task 5 的 `callback.*` 常量与 `(*ExecutorJobService).DispatchCompletionCallback`
- Produces:
  - `worker.NewInternalCallbackWorker(runner JobRunner, env, consumerID string, log *logger.Log) *InternalCallbackWorker`
  - `(*InternalCallbackWorker).Start()` / `Stop()`
  - `(*Module).StartInternalWorker(env, consumerID string)` / `(*Module).StopInternalWorker()`

- [ ] **Step 1: 写失败的测试**

创建 `system/executor/internal/worker/internal_worker_test.go`：

```go
package worker

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/xsxdot/aio/system/executor/api/callback"
	"github.com/xsxdot/aio/system/executor/internal/model"
	"github.com/xsxdot/aio/system/executor/internal/service"
	"github.com/xsxdot/gokit/logger"
)

type fakeRunner struct {
	acquired   []*service.AcquiredJobResult
	dispatched []callback.CallbackPayload
	acked      []model.JobStatus
	dispatchErr error
}

func (f *fakeRunner) AcquireJobs(ctx context.Context, req service.AcquireJobsRequest) ([]*service.AcquiredJobResult, error) {
	out := f.acquired
	f.acquired = nil // 只投递一轮，避免测试无限循环
	return out, nil
}

func (f *fakeRunner) AckJob(ctx context.Context, jobID uint64, attemptNo int32, consumerID string,
	status model.JobStatus, errorMsg, resultJSON string, retryAfter int32,
	stopRetry bool, addMaxAttempts int32, errorType string) error {
	f.acked = append(f.acked, status)
	return nil
}

func (f *fakeRunner) DispatchCompletionCallback(ctx context.Context, source string,
	jobID uint64, callbackData, resultJSON string) error {
	f.dispatched = append(f.dispatched, callback.CallbackPayload{
		JobID: jobID, Source: source, CallbackData: callbackData, ResultJSON: resultJSON,
	})
	return f.dispatchErr
}

// newOutboxJob 构造一条 outbox 回调任务。
// outboxID 是 outbox 任务自身的 ID（worker 用它 Ack）；
// originJobID 是载荷里的原始业务任务 ID（派发给 handler 的那个）。两者必须区分。
func newOutboxJob(t *testing.T, outboxID int64, originJobID uint64, source string) *service.AcquiredJobResult {
	t.Helper()
	args, err := json.Marshal(callback.CallbackPayload{
		JobID: originJobID, Source: source,
		CallbackData: `{"instance_id":7,"node_id":"A","env":"dev"}`,
		ResultJSON:   `{"ok":true}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	job := &model.ExecutorJobModel{
		Env: "dev", TargetService: callback.InternalTargetService,
		Method: callback.MethodJobCompletedCallback, ArgsJSON: string(args),
	}
	job.ID = outboxID
	return &service.AcquiredJobResult{Job: job, AttemptNo: 1, ConsumerID: "aio-1-slot-0"}
}

// 领到 outbox 任务后按 source 路由派发，并以 succeeded 确认。
func TestWorkerDispatchesAndAcksSucceeded(t *testing.T) {
	job := newOutboxJob(t, 555, 9001, "workflow")
	f := &fakeRunner{acquired: []*service.AcquiredJobResult{job}}
	w := NewInternalCallbackWorker(f, "dev", "aio-1", logger.GetLogger())

	w.pollOnce(context.Background())

	if len(f.dispatched) != 1 {
		t.Fatalf("dispatched = %d, want 1", len(f.dispatched))
	}
	if f.dispatched[0].JobID != 9001 || f.dispatched[0].Source != "workflow" {
		t.Fatalf("dispatched payload = %+v", f.dispatched[0])
	}
	if len(f.acked) != 1 || f.acked[0] != model.JobStatusSucceeded {
		t.Fatalf("acked = %v, want [succeeded]", f.acked)
	}
}

// 派发失败必须以 failed 确认，让 executor 重试而不是静默丢弃。
func TestWorkerAcksFailedOnDispatchError(t *testing.T) {
	job := newOutboxJob(t, 556, 9002, "workflow")
	f := &fakeRunner{
		acquired:    []*service.AcquiredJobResult{job},
		dispatchErr: errors.New("handler boom"),
	}
	w := NewInternalCallbackWorker(f, "dev", "aio-1", logger.GetLogger())

	w.pollOnce(context.Background())

	if len(f.acked) != 1 || f.acked[0] != model.JobStatusFailed {
		t.Fatalf("acked = %v, want [failed]", f.acked)
	}
}

// ArgsJSON 无法解析时以 failed 确认（重试到死信后人工可见），不得 panic。
func TestWorkerAcksFailedOnBadPayload(t *testing.T) {
	// ID 来自内嵌的 common.Model，Go 不允许在复合字面量里设置提升字段，必须单独赋值
	badJob := &model.ExecutorJobModel{
		Env:           "dev",
		TargetService: callback.InternalTargetService,
		Method:        callback.MethodJobCompletedCallback,
		ArgsJSON:      `{not json`,
	}
	badJob.ID = 557
	f := &fakeRunner{acquired: []*service.AcquiredJobResult{{
		Job: badJob, AttemptNo: 1, ConsumerID: "aio-1-slot-0",
	}}}
	w := NewInternalCallbackWorker(f, "dev", "aio-1", logger.GetLogger())

	w.pollOnce(context.Background())

	if len(f.dispatched) != 0 {
		t.Fatalf("dispatched = %d, want 0", len(f.dispatched))
	}
	if len(f.acked) != 1 || f.acked[0] != model.JobStatusFailed {
		t.Fatalf("acked = %v, want [failed]", f.acked)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
go test ./system/executor/internal/worker/ -v
```

预期：编译失败，`undefined: NewInternalCallbackWorker`。

- [ ] **Step 3: 实现 worker**

创建 `system/executor/internal/worker/internal_worker.go`：

```go
// Package worker 提供 aio 进程内的回调消费者。
//
// 职责：消费 executor 中 target_service=aio、method=internal.job_completed_callback
// 的 outbox 任务，按 source 路由到已注册的 JobCompletionHandler。
//
// 边界：不承载任何业务逻辑，不处理其他 method；只消费本进程 env 的任务——
// 跨 env 的回调需由对应 env 的 aio 实例消费（见设计文档 §4.1）。
// 直调 service 而非绕 gRPC：同进程无需网络层，也避免自连自身的启动顺序依赖。
package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/xsxdot/aio/system/executor/api/callback"
	"github.com/xsxdot/aio/system/executor/internal/dao"
	"github.com/xsxdot/aio/system/executor/internal/model"
	"github.com/xsxdot/aio/system/executor/internal/service"
	"github.com/xsxdot/gokit/logger"
)

const (
	pollInterval  = time.Second
	leaseDuration = 60
	maxConcurrent = 5
)

// JobRunner 是内部 worker 依赖的最小任务接口，由 ExecutorJobService 实现。
type JobRunner interface {
	AcquireJobs(ctx context.Context, req service.AcquireJobsRequest) ([]*service.AcquiredJobResult, error)
	AckJob(ctx context.Context, jobID uint64, attemptNo int32, consumerID string,
		status model.JobStatus, errorMsg, resultJSON string, retryAfter int32,
		stopRetry bool, addMaxAttempts int32, errorType string) error
	DispatchCompletionCallback(ctx context.Context, source string, jobID uint64,
		callbackData, resultJSON string) error
}

// InternalCallbackWorker 消费 outbox 回调任务的进程内 worker。
type InternalCallbackWorker struct {
	runner     JobRunner
	env        string
	consumerID string
	log        *logger.Log

	stopCtx    context.Context
	stopCancel context.CancelFunc
	wg         sync.WaitGroup
	startOnce  sync.Once
	stopOnce   sync.Once
}

// NewInternalCallbackWorker 创建内部回调 worker。
//
// 参数：
//   - runner: 任务领取/确认/派发能力，生产环境传 *service.ExecutorJobService
//   - env: 消费的环境标识，通常为 base.ENV
//   - consumerID: 实例标识，多实例下必须互不相同
func NewInternalCallbackWorker(runner JobRunner, env, consumerID string, log *logger.Log) *InternalCallbackWorker {
	ctx, cancel := context.WithCancel(context.Background())
	return &InternalCallbackWorker{
		runner:     runner,
		env:        env,
		consumerID: consumerID,
		log:        log,
		stopCtx:    ctx,
		stopCancel: cancel,
	}
}

// Start 启动轮询循环。重复调用无副作用。
func (w *InternalCallbackWorker) Start() {
	w.startOnce.Do(func() {
		w.wg.Add(1)
		go func() {
			defer w.wg.Done()
			w.log.WithField("env", w.env).WithField("consumer_id", w.consumerID).
				Info("内部回调 worker 已启动")
			ticker := time.NewTicker(pollInterval)
			defer ticker.Stop()
			for {
				select {
				case <-w.stopCtx.Done():
					w.log.Info("内部回调 worker 已停止")
					return
				case <-ticker.C:
					w.pollOnce(w.stopCtx)
				}
			}
		}()
	})
}

// Stop 停止轮询并等待在途回调结束。重复调用无副作用。
func (w *InternalCallbackWorker) Stop() {
	w.stopOnce.Do(func() {
		w.stopCancel()
		w.wg.Wait()
	})
}

// pollOnce 领取并处理一轮 outbox 回调任务。
// 单独导出为方法便于测试直接驱动一轮，不必等待 ticker。
func (w *InternalCallbackWorker) pollOnce(ctx context.Context) {
	consumerIDs := make([]string, 0, maxConcurrent)
	for i := 0; i < maxConcurrent; i++ {
		consumerIDs = append(consumerIDs, fmt.Sprintf("%s-slot-%d", w.consumerID, i))
	}

	jobs, err := w.runner.AcquireJobs(ctx, service.AcquireJobsRequest{
		Env:           w.env,
		TargetService: callback.InternalTargetService,
		Methods:       []string{callback.MethodJobCompletedCallback},
		ConsumerIDs:   consumerIDs,
		LeaseDuration: leaseDuration,
		Mode:          dao.AcquireJobsModeFillSlots,
	})
	if err != nil {
		w.log.WithErr(err).WithField("env", w.env).Error("内部回调 worker 领取任务失败")
		return
	}
	if len(jobs) == 0 {
		return
	}
	w.log.WithField("count", len(jobs)).Debug("内部回调 worker 领到任务")

	for _, j := range jobs {
		w.handle(ctx, j)
	}
}

func (w *InternalCallbackWorker) handle(ctx context.Context, j *service.AcquiredJobResult) {
	jobID := uint64(j.Job.ID)
	started := time.Now()

	var payload callback.CallbackPayload
	if err := json.Unmarshal([]byte(j.Job.ArgsJSON), &payload); err != nil {
		w.log.WithErr(err).WithField("outbox_job_id", jobID).
			Error("解析回调载荷失败，标记失败等待重试")
		w.ack(ctx, jobID, j, model.JobStatusFailed, "解析回调载荷失败: "+err.Error(), "")
		return
	}

	w.log.WithField("outbox_job_id", jobID).
		WithField("origin_job_id", payload.JobID).
		WithField("source", payload.Source).
		Debug("开始派发完成回调")

	if err := w.runner.DispatchCompletionCallback(ctx, payload.Source, payload.JobID,
		payload.CallbackData, payload.ResultJSON); err != nil {
		w.log.WithErr(err).
			WithField("outbox_job_id", jobID).
			WithField("origin_job_id", payload.JobID).
			WithField("source", payload.Source).
			Error("派发完成回调失败，标记失败等待重试")
		w.ack(ctx, jobID, j, model.JobStatusFailed, err.Error(), "")
		return
	}

	w.log.WithField("outbox_job_id", jobID).
		WithField("origin_job_id", payload.JobID).
		WithField("source", payload.Source).
		WithField("cost_ms", time.Since(started).Milliseconds()).
		Info("完成回调派发成功")
	w.ack(ctx, jobID, j, model.JobStatusSucceeded, "", "")
}

func (w *InternalCallbackWorker) ack(ctx context.Context, jobID uint64,
	j *service.AcquiredJobResult, status model.JobStatus, errMsg, resultJSON string) {
	if err := w.runner.AckJob(ctx, jobID, j.AttemptNo, j.ConsumerID,
		status, errMsg, resultJSON, 0, false, 0, ""); err != nil {
		// 确认失败不重试：租约到期后 executor 会重投，重复派发由 workflow 侧幂等表拦截。
		w.log.WithErr(err).WithField("outbox_job_id", jobID).WithField("status", status).
			Error("内部回调 worker 确认任务失败，等待租约到期重投")
	}
}
```

- [ ] **Step 4: 运行测试确认通过**

```bash
go test ./system/executor/internal/worker/ -v
```

预期：三个用例 PASS。

- [ ] **Step 5: 接线到 Module 与 main**

修改 `system/executor/module.go`，`Module` 增加字段 `internalWorker *worker.InternalCallbackWorker`，并新增：

```go
// StartInternalWorker 启动进程内 outbox 回调消费者。
//
// 参数：
//   - env: 消费的环境标识，传 base.ENV
//   - consumerID: 实例标识，多实例下必须互不相同（优先用 registry 自注册的
//     instanceKey，未启用自注册时用 hostname-pid）
func (m *Module) StartInternalWorker(env, consumerID string) {
	m.internalWorker = worker.NewInternalCallbackWorker(
		m.internalApp.JobService, env, consumerID,
		base.Logger.WithEntryName("InternalCallbackWorker"),
	)
	m.internalWorker.Start()
}

// StopInternalWorker 停止内部回调消费者并等待在途回调结束。
func (m *Module) StopInternalWorker() {
	if m.internalWorker != nil {
		m.internalWorker.Stop()
	}
}
```

修改 `main.go`，在 `appRoot := app.NewApp()` 之后、注册 cron 之前插入：

```go
	// 启动内部回调 worker：消费 AckJob 产生的 outbox 回调任务。
	// consumerID 必须跨实例唯一，否则多实例会争抢同一 slot 导致领取受阻。
	hostname, _ := os.Hostname()
	callbackConsumerID := fmt.Sprintf("aio-%s-%d", hostname, os.Getpid())
	appRoot.ExecutorModule.StartInternalWorker(base.ENV, callbackConsumerID)
	base.Logger.WithField("consumer_id", callbackConsumerID).Info("内部回调 worker 已注册")
```

并在 main.go 现有的优雅关闭逻辑中调用 `appRoot.ExecutorModule.StopInternalWorker()`。若 `main.go` 尚无 `os` import 则补上。

- [ ] **Step 6: 全量编译与测试**

```bash
go build ./... && go test ./...
```

预期：编译通过，全部 PASS（依赖真实数据库的 executor DAO 测试会 SKIP，属正常）。

- [ ] **Step 7: 提交**

```bash
git add system/executor/internal/worker/ system/executor/module.go main.go
git commit -m "feat(executor): 新增进程内 outbox 回调 worker

直调 service 消费 internal.job_completed_callback 任务并按 source 路由。
多实例下 consumerID 唯一，靠 SKIP LOCKED 天然去重。"
```

---

## Task 7: 真机数据库验证（本地执行，不派发）

> **执行者注意：本任务不得派发给远程 executor。** 它需要真实 MySQL 与 PostgreSQL 实例，且验证结论依赖人工判读。若你是被派发的远程执行者，请跳过本任务并在 ledger 中如实记为「未验证，留待审核者本地执行」，**不要**用 SQLite 结果代替。

**Files:**
- Create: `system/workflow/internal/app/engine_deadlock_realdb_test.go`

**Interfaces:**
- Consumes: Task 2、Task 3 的事务改造
- Produces: 真实数据库上「事务透传不死锁」的证据

**背景：** `gorm.io/driver/sqlite` **静默忽略** `FOR UPDATE`（已实测：不报错、无行锁）。因此 Task 2/3 的 SQLite 测试只能证明机制上复用了同一事务，**不能证明真实数据库上不死锁**。这是本计划中唯一必须真机验证的判据。

- [ ] **Step 1: 准备两个一次性数据库**

```bash
docker run -d --rm --name aio-test-pg -e POSTGRES_PASSWORD=test -e POSTGRES_DB=aio_test -p 55432:5432 postgres:16
docker run -d --rm --name aio-test-mysql -e MYSQL_ROOT_PASSWORD=test -e MYSQL_DATABASE=aio_test -p 53306:3306 mysql:8
```

**警告：** 不要指向 dev/prod 数据库——测试会 `DropTable`。

- [ ] **Step 2: 写真机死锁验证测试**

创建 `system/workflow/internal/app/engine_deadlock_realdb_test.go`：

```go
package app

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/xsxdot/aio/base"
	"github.com/xsxdot/aio/pkg/core/mvc"
	"github.com/xsxdot/aio/system/workflow/internal/model"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func openRealDB(t *testing.T, dialect string) *gorm.DB {
	t.Helper()
	var (
		db  *gorm.DB
		err error
	)
	switch dialect {
	case "postgres":
		dsn := os.Getenv("AIO_WORKFLOW_TEST_POSTGRES_URL")
		if dsn == "" {
			t.Skip("AIO_WORKFLOW_TEST_POSTGRES_URL is not set")
		}
		db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	case "mysql":
		dsn := os.Getenv("AIO_WORKFLOW_TEST_MYSQL_DSN")
		if dsn == "" {
			t.Skip("AIO_WORKFLOW_TEST_MYSQL_DSN is not set")
		}
		db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	default:
		t.Fatalf("unsupported dialect %q", dialect)
	}
	if err != nil {
		t.Fatalf("open %s: %v", dialect, err)
	}
	if err := db.Migrator().DropTable(&model.WorkflowAppliedCallbackModel{}, &model.WorkflowInstanceModel{}); err != nil {
		t.Fatalf("drop tables: %v", err)
	}
	if err := db.AutoMigrate(&model.WorkflowInstanceModel{}, &model.WorkflowAppliedCallbackModel{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func forEachRealDialect(t *testing.T, fn func(t *testing.T, db *gorm.DB)) {
	t.Helper()
	for _, d := range []string{"postgres", "mysql"} {
		d := d
		t.Run(d, func(t *testing.T) { fn(t, openRealDB(t, d)) })
	}
}

// 核心判据：外层事务对实例行持有 FOR UPDATE 时，内层通过 ExtractDB 重入
// 同一实例行的加锁读必须立即成功（SavePoint 复用同一事务），不得超时。
// 若改造被回退成 base.DB.Transaction，本用例会挂到超时后失败。
func TestNestedLockingDoesNotDeadlockOnRealDB(t *testing.T) {
	forEachRealDialect(t, func(t *testing.T, db *gorm.DB) {
		prev := base.DB
		base.DB = db
		t.Cleanup(func() { base.DB = prev })

		inst := &model.WorkflowInstanceModel{
			DefID: 1, DefVersion: 1, Status: model.InstanceStatusRunning,
			InitialState: `{"data":{},"_sys":{}}`,
			CurrentState: `{"data":{},"_sys":{}}`,
			ActiveNodeIDs: `["A"]`,
		}
		if err := db.Create(inst).Error; err != nil {
			t.Fatalf("create instance: %v", err)
		}

		done := make(chan error, 1)
		go func() {
			ctx := context.Background()
			done <- mvc.ExtractDB(ctx, base.DB).Transaction(func(outer *gorm.DB) error {
				var locked model.WorkflowInstanceModel
				if err := outer.Clauses(clause.Locking{Strength: "UPDATE"}).
					Where("id = ?", inst.ID).First(&locked).Error; err != nil {
					return err
				}
				txCtx := mvc.WithTxToContext(ctx, outer)
				// 模拟 condition 递归重入 ReportNodeCompleted 的事务入口
				return mvc.ExtractDB(txCtx, base.DB).Transaction(func(inner *gorm.DB) error {
					var again model.WorkflowInstanceModel
					if err := inner.Clauses(clause.Locking{Strength: "UPDATE"}).
						Where("id = ?", inst.ID).First(&again).Error; err != nil {
						return err
					}
					return inner.Model(&model.WorkflowInstanceModel{}).
						Where("id = ?", inst.ID).
						Update("active_node_ids", `["B"]`).Error
				})
			})
		}()

		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("嵌套加锁事务失败: %v", err)
			}
		case <-time.After(15 * time.Second):
			t.Fatal("嵌套加锁事务超时 —— 出现死锁，说明存在未改造为 ExtractDB 的事务入口")
		}

		var got model.WorkflowInstanceModel
		if err := db.Where("id = ?", inst.ID).First(&got).Error; err != nil {
			t.Fatalf("reload: %v", err)
		}
		if got.ActiveNodeIDs != `["B"]` {
			t.Fatalf("ActiveNodeIDs = %s, want [\"B\"]", got.ActiveNodeIDs)
		}
	})
}

// 幂等表的 ON CONFLICT DO NOTHING 在两种真实方言上行为一致。
func TestMarkAppliedOnConflictAcrossRealDialects(t *testing.T) {
	forEachRealDialect(t, func(t *testing.T, db *gorm.DB) {
		rec := func() *model.WorkflowAppliedCallbackModel {
			return &model.WorkflowAppliedCallbackModel{JobID: 7777, InstanceID: 1, NodeID: "A"}
		}
		first := db.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "job_id"}}, DoNothing: true,
		}).Create(rec())
		if first.Error != nil || first.RowsAffected != 1 {
			t.Fatalf("first insert: err=%v rows=%d, want rows=1", first.Error, first.RowsAffected)
		}
		second := db.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "job_id"}}, DoNothing: true,
		}).Create(rec())
		if second.Error != nil {
			t.Fatalf("second insert error: %v（期望静默跳过而非报错）", second.Error)
		}
		if second.RowsAffected != 0 {
			t.Fatalf("second insert rows = %d, want 0", second.RowsAffected)
		}
	})
}
```

- [ ] **Step 3: 运行真机测试**

```bash
AIO_WORKFLOW_TEST_POSTGRES_URL="host=127.0.0.1 port=55432 user=postgres password=test dbname=aio_test sslmode=disable" \
AIO_WORKFLOW_TEST_MYSQL_DSN="root:test@tcp(127.0.0.1:53306)/aio_test?charset=utf8mb4&parseTime=True&loc=Local" \
go test ./system/workflow/internal/app/ -run 'TestNestedLockingDoesNotDeadlock|TestMarkAppliedOnConflict' -v -timeout 120s
```

预期：`postgres` 与 `mysql` 两个子测试均 PASS，**且没有 SKIP**。若出现 SKIP，说明环境变量未生效，本任务未完成。

- [ ] **Step 4: 记录证据**

把完整测试输出粘贴到本任务下方作为验收证据。**不得**以「代码看起来对」或 SQLite 结果代替。

- [ ] **Step 5: 清理测试数据库**

```bash
docker stop aio-test-pg aio-test-mysql
```

- [ ] **Step 6: 提交**

```bash
git add system/workflow/internal/app/engine_deadlock_realdb_test.go
git commit -m "test(workflow): 新增真机数据库嵌套加锁与幂等冲突验证

SQLite 静默忽略 FOR UPDATE，死锁判据只能在真实 MySQL/PostgreSQL 上成立。"
```

---

## Self-Review

**Spec 覆盖核对：**

| Spec 章节 | 对应 Task |
|---|---|
| §2.1 事务透传改动本体 | Task 3 Step 3 |
| §2.2 语义变化（失败整体回滚） | Task 3 Step 3；测试 Task 3 Step 1 |
| §2.3 5 处 `base.DB.Transaction` | Task 2 Step 3-4 |
| §3.1 outbox job 形态（含 Source 留空） | Task 5 Step 3、Step 5；测试 Task 5 Step 1 |
| §3.2 AckJob 事务包装 | Task 5 Step 5 |
| §3.3 内部 worker 规格 | Task 6 Step 3、Step 5 |
| §3.4 幂等表与不复用 checkpoint 的理由 | Task 1（表+DAO）、Task 4（接入） |
| §4.1 env 约束 | Task 6 Step 3 包注释 |
| §4.2 死信表现 | Task 5 Step 3（`OutboxMaxAttempts`） |
| §4.3 回退开关（字符串而非 bool） | Task 5 Step 4、Step 5 |
| §4.4 不做的四件事 | Global Constraints + 各 Task 未涉及 |
| §4.5 `fmt.Printf` 清理 | Task 3 Step 4 |
| §5 测试策略 8 项 | Task 1/3/4/5/6 各 Step 1 + Task 7 |
| §6 日志与注释 | Task 4 Step 6、Task 5/6 代码内已含 |
| §7 文件清单 | File Structure 节 |

无遗漏。

**类型一致性核对：**

- `MarkApplied(ctx, tx, jobID, instanceID int64, nodeID string) (bool, error)` —— Task 1 定义，Task 4 Step 4 按此调用 ✓
- `CleanupBefore(ctx, before time.Time) (int64, error)` —— Task 1 定义，Task 1 Step 7 经 `Module.CleanupAppliedCallbacks` 调用 ✓
- `JobCompletionHandler.OnJobCompleted(...) error` —— Task 4 Step 5 改签名，Task 5 Step 3 契约、Step 5 同步分支、`DispatchCompletionCallback` 均按返回 error 使用 ✓
- `AcquireJobs(ctx, service.AcquireJobsRequest) ([]*service.AcquiredJobResult, error)` —— 与现有 `executor_job_service.go:203` 签名一致（值传参非指针）✓
- `AckJob` 11 参数签名 —— Task 6 `JobRunner` 与现有 `executor_job_service.go:339` 一致 ✓
- `callback.CallbackPayload` 字段 —— Task 5 Step 3 定义，Task 5 Step 1 测试与 Task 6 worker 一致 ✓

**占位符扫描：** 无 TBD/TODO；所有代码步骤含可直接使用的完整代码块；Task 4 Step 3 的「原方法体」为明确的重命名操作而非省略。

---

## 派发提示

按 CLAUDE.md §4，本计划可整份派发给 linux-01 上的 codex，**但 Task 7 必须留在本地由审核者执行**——它需要真实 MySQL/PostgreSQL 容器，且其结论是整个事务透传方案是否成立的唯一真机判据。派发前请在派发内容中删除 Task 7，或明确标注「本 task 由审核者执行，不派发」（Task 7 开头已含该标注）。

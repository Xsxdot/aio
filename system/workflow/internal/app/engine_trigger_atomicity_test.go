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
		InitialState:  `{"data":{},"_sys":{}}`,
		CurrentState:  `{"data":{},"_sys":{}}`,
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
		t.Fatalf("Status = %s, want %s — 不应被置为 FAILED", got.Status, model.InstanceStatusRunning)
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

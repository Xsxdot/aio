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

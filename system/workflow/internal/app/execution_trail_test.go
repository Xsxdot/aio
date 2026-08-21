package app

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/xsxdot/aio/system/workflow/internal/dao"
	"github.com/xsxdot/aio/system/workflow/internal/model"
	"github.com/xsxdot/aio/system/workflow/internal/service"
	"github.com/xsxdot/gokit/logger"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestGetExecutionTrailOmitsStateAfterByDefault(t *testing.T) {
	ctx := context.Background()
	a, cpSvc, instanceID := newExecutionTrailTestApp(t)

	lightCheckpoints, err := cpSvc.ListTrailByInstanceIDOrderByCreatedAsc(ctx, instanceID)
	if err != nil {
		t.Fatalf("list lightweight checkpoints: %v", err)
	}
	if len(lightCheckpoints) != 2 {
		t.Fatalf("len(lightCheckpoints) = %d, want 2", len(lightCheckpoints))
	}
	if lightCheckpoints[0].StateAfter != "" {
		t.Fatalf("lightweight checkpoint loaded state_after, bytes=%d", len(lightCheckpoints[0].StateAfter))
	}

	fullCheckpoints, err := cpSvc.ListByInstanceIDOrderByCreatedAsc(ctx, instanceID)
	if err != nil {
		t.Fatalf("list full checkpoints: %v", err)
	}
	if fullCheckpoints[0].StateAfter == "" {
		t.Fatal("full checkpoint did not load state_after")
	}

	trail, err := a.GetExecutionTrail(ctx, instanceID)
	if err != nil {
		t.Fatalf("get execution trail: %v", err)
	}
	if got := trail.Checkpoints[0].StateAfter; got != nil {
		t.Fatalf("default trail StateAfter = %#v, want nil", got)
	}
	trailJSON, err := json.Marshal(trail)
	if err != nil {
		t.Fatalf("marshal trail: %v", err)
	}
	if strings.Contains(string(trailJSON), "state_after") {
		t.Fatalf("default trail JSON should omit state_after: %s", string(trailJSON))
	}
}

func TestGetExecutionTrailCanIncludeStateAfter(t *testing.T) {
	ctx := context.Background()
	a, _, instanceID := newExecutionTrailTestApp(t)

	trail, err := a.GetExecutionTrailWithOptions(ctx, instanceID, ExecutionTrailOptions{
		IncludeStateAfter: true,
	})
	if err != nil {
		t.Fatalf("get execution trail with state_after: %v", err)
	}
	if len(trail.Checkpoints) != 2 {
		t.Fatalf("len(Checkpoints) = %d, want 2", len(trail.Checkpoints))
	}
	if trail.Checkpoints[0].StateAfter == nil {
		t.Fatal("StateAfter = nil, want full state")
	}
	data, ok := trail.Checkpoints[0].StateAfter["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("StateAfter[data] type = %T, want map[string]interface{}", trail.Checkpoints[0].StateAfter["data"])
	}
	if data["large"] == "" {
		t.Fatal("StateAfter[data][large] is empty")
	}
}

func TestGetExecutionStateReturnsCurrentState(t *testing.T) {
	ctx := context.Background()
	a, _, instanceID := newExecutionTrailTestApp(t)

	state, err := a.GetExecutionState(ctx, instanceID, "")
	if err != nil {
		t.Fatalf("get current execution state: %v", err)
	}
	if state.NotFound {
		t.Fatal("current state NotFound = true, want false")
	}
	if state.NodeID != "" {
		t.Fatalf("current state NodeID = %q, want empty", state.NodeID)
	}
	if state.StateJSON != `{"data":{"final":"ok"},"_sys":{}}` {
		t.Fatalf("current StateJSON = %s", state.StateJSON)
	}
	if state.CreatedAt == "" {
		t.Fatal("current CreatedAt is empty")
	}
}

func TestGetExecutionStateReturnsLatestNodeStateAfter(t *testing.T) {
	ctx := context.Background()
	a, _, instanceID := newExecutionTrailTestApp(t)

	state, err := a.GetExecutionState(ctx, instanceID, "node-a")
	if err != nil {
		t.Fatalf("get node execution state: %v", err)
	}
	if state.NotFound {
		t.Fatal("node state NotFound = true, want false")
	}
	if state.NodeID != "node-a" {
		t.Fatalf("node state NodeID = %q, want node-a", state.NodeID)
	}
	if !strings.Contains(state.StateJSON, `"large"`) {
		t.Fatalf("node StateJSON = %s", state.StateJSON)
	}
	if state.CreatedAt == "" {
		t.Fatal("node CreatedAt is empty")
	}
}

func TestGetExecutionStateReturnsNotFoundForUnexecutedNode(t *testing.T) {
	ctx := context.Background()
	a, _, instanceID := newExecutionTrailTestApp(t)

	state, err := a.GetExecutionState(ctx, instanceID, "node-missing")
	if err != nil {
		t.Fatalf("get missing node execution state: %v", err)
	}
	if !state.NotFound {
		t.Fatal("missing node state NotFound = false, want true")
	}
}

func newExecutionTrailTestApp(t *testing.T) (*App, *service.WorkflowCheckpointService, int64) {
	t.Helper()

	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.WorkflowInstanceModel{}, &model.WorkflowCheckpointModel{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	log := logger.GetLogger()
	instDao := dao.NewWorkflowInstanceDao(db, log)
	cpDao := dao.NewWorkflowCheckpointDao(db, log)
	instSvc := service.NewWorkflowInstanceService(instDao, log)
	cpSvc := service.NewWorkflowCheckpointService(cpDao, log)

	instance := &model.WorkflowInstanceModel{
		DefID:         1,
		DefVersion:    1,
		Status:        model.InstanceStatusCompleted,
		InitialState:  `{"data":{},"_sys":{}}`,
		CurrentState:  `{"data":{"final":"ok"},"_sys":{}}`,
		ActiveNodeIDs: `[]`,
	}
	if err := db.Create(instance).Error; err != nil {
		t.Fatalf("create instance: %v", err)
	}

	largeState := `{"data":{"large":"` + strings.Repeat("x", 4096) + `"},"_sys":{}}`
	checkpoints := []*model.WorkflowCheckpointModel{
		{
			InstanceID: instance.ID,
			NodeID:     "node-a",
			NodeOutput: `{"ok":true}`,
			StateAfter: largeState,
		},
		{
			InstanceID: instance.ID,
			NodeID:     "node-b",
			NodeOutput: `{"done":true}`,
			StateAfter: largeState,
		},
	}
	if err := db.Create(&checkpoints).Error; err != nil {
		t.Fatalf("create checkpoints: %v", err)
	}

	return NewApp(nil, instSvc, cpSvc, nil, nil), cpSvc, instance.ID
}

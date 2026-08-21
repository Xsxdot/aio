package app

import (
	"context"
	"fmt"
	"testing"

	"github.com/xsxdot/aio/system/workflow/internal/model"
)

// 纯 condition 组成的回环链是一个能通过 DAG.Validate 的合法定义 ——
// 本用例固化「为什么必须有跳数护栏」这一事实。
//
// detectCycle 对 IsLoopback 边直接跳过，环对校验不可见；若哪天校验改为
// 也检测回环边，本用例会失败，届时可重新评估护栏是否还有必要。
func TestPureConditionLoopbackIsAcceptedByValidate(t *testing.T) {
	d := &model.DAG{
		Nodes: []model.Node{
			{ID: "START", Type: model.NodeTypeTask},
			{ID: "C1", Type: model.NodeTypeCondition},
			{ID: "C2", Type: model.NodeTypeCondition},
		},
		Edges: []model.Edge{
			{From: "START", To: "C1"},
			{From: "C1", To: "C2"},
			{From: "C2", To: "C1", IsLoopback: true},
		},
	}
	if err := d.Validate(); err != nil {
		t.Fatalf("校验拒绝了纯 condition 回环: %v —— 若校验已能拦截，跳数护栏的必要性需重新评估", err)
	}
}

// 最外层调用必须被识别为 outermost，且拿到一个全新的护栏。
func TestEnterAdvanceHopIdentifiesOutermost(t *testing.T) {
	ctx, g, outermost, exceeded := enterAdvanceHop(context.Background(), "A")
	if !outermost {
		t.Fatal("首次调用未被识别为最外层，超限时将没有人负责善后")
	}
	if exceeded {
		t.Fatal("首跳即判超限")
	}
	if g == nil {
		t.Fatal("护栏为 nil")
	}

	_, g2, outermost2, _ := enterAdvanceHop(ctx, "B")
	if outermost2 {
		t.Fatal("嵌套调用被误判为最外层，会导致每一层都去终结实例")
	}
	if g2 != g {
		t.Fatal("嵌套调用拿到了不同的护栏，深处的超限事实传不回最外层")
	}
}

// 递归到上限时必须停下，且最外层持有的护栏能看到 limitHit。
// 这是护栏的核心契约：深处置位、最外层善后。
func TestEnterAdvanceHopTripsAtLimit(t *testing.T) {
	ctx, outerGuard, outermost, exceeded := enterAdvanceHop(context.Background(), "hop0")
	if !outermost || exceeded {
		t.Fatalf("初始状态异常: outermost=%v exceeded=%v", outermost, exceeded)
	}

	trippedAt := -1
	// 首跳已消耗 1，再往下推足够多层，必须在上限处停住而不是一路递归下去
	for i := 1; i <= maxAdvanceHops+10; i++ {
		var ex bool
		ctx, _, _, ex = enterAdvanceHop(ctx, fmt.Sprintf("hop%d", i))
		if ex {
			trippedAt = i
			break
		}
	}

	if trippedAt == -1 {
		t.Fatalf("推进 %d 层仍未触发上限，护栏形同虚设", maxAdvanceHops+10)
	}
	if trippedAt != maxAdvanceHops {
		t.Fatalf("在第 %d 跳触发上限，期望第 %d 跳", trippedAt, maxAdvanceHops)
	}
	if !outerGuard.limitHit {
		t.Fatal("最外层护栏未看到 limitHit，善后逻辑不会被触发，实例将停在 RUNNING")
	}
	if len(outerGuard.path) == 0 {
		t.Fatal("未记录递归路径，超限后无法定位是哪条回环")
	}
}

// 正常深度（远小于上限）不得被误伤：护栏只该拦住配错的回环，
// 不该限制合法的多级 condition 链。
func TestEnterAdvanceHopAllowsNormalDepth(t *testing.T) {
	ctx := context.Background()
	for i := 0; i < 8; i++ {
		var exceeded bool
		ctx, _, _, exceeded = enterAdvanceHop(ctx, fmt.Sprintf("n%d", i))
		if exceeded {
			t.Fatalf("正常深度第 %d 跳被误判超限", i)
		}
	}
}

package app

import (
	"testing"

	"github.com/xsxdot/aio/system/workflow/internal/model"
)

func TestEdgeTargetEligibleForScheduling(t *testing.T) {
	edgeLoop := model.Edge{From: "E", To: "A", IsLoopback: true}
	edgeFwd := model.Edge{From: "D", To: "E", IsLoopback: false}

	if !edgeTargetEligibleForScheduling(edgeLoop, 99, false) {
		t.Fatal("loopback should schedule even when target already has checkpoints")
	}
	if edgeTargetEligibleForScheduling(edgeLoop, 0, true) {
		t.Fatal("loopback should not schedule when target already in active set")
	}
	if edgeTargetEligibleForScheduling(edgeFwd, 1, false) {
		t.Fatal("non-loopback should not schedule when cpCount > 0")
	}
	if !edgeTargetEligibleForScheduling(edgeFwd, 0, false) {
		t.Fatal("non-loopback should schedule first time")
	}
}

// 职责：为「单次推进内的同步递归」提供跳数护栏，防止纯 condition 回环把
// 进程递归到栈溢出。
//
// 边界：只约束不落地即返回的同步跳转（当前仅 condition 节点走这条路径）。
// 经由 executor 任务或人工审批驱动的正常回环迭代不受影响——那些每一跳都是
// 独立的一次推进，进入本护栏时计数从零开始。
package app

import (
	"context"

	"github.com/xsxdot/aio/base"
	"github.com/xsxdot/aio/pkg/core/mvc"
	"github.com/xsxdot/aio/system/workflow/internal/model"
	"gorm.io/gorm"
)

// maxAdvanceHops 单次推进允许的同步递归跳数上限。
//
// 为什么需要它：condition 节点不落地即递归回推进入口，而 DAG 的回环边
// （Edge.IsLoopback）被 detectCycle 显式豁免环路检测，因此「纯 condition
// 组成的回环」是一个能通过 DAG.Validate 的合法定义，运行时会无限递归。
// 真实工作流的连续 condition 链是个位数，64 远高于任何正常用法——触到即
// 说明 DAG 配错了，而不是业务变复杂了。
const maxAdvanceHops = 64

type advanceHopKeyType struct{}
type advanceGuardKeyType struct{}

// advanceGuard 在一次推进的整条递归链上共享，把「深处触到上限」这个事实带回
// 最外层。
//
// 不用哨兵 error + errors.Is：递归链中间层会用 errorc 包装错误，该包装链是否
// 支持 Unwrap 不由本包决定，靠错误身份传递会在包装实现变化时静默失效。
type advanceGuard struct {
	limitHit bool
	// path 记录本次递归链经过的节点，超限时用于定位是哪条回环。
	// 长度天然被 maxAdvanceHops 限制。
	path []string
}

// enterAdvanceHop 在推进入口登记一跳。
//
// 参数 nodeID 仅用于超限时的诊断路径。
// 返回：
//   - newCtx: 携带递增后跳数的 ctx，必须用它替换原 ctx 继续往下传，否则
//     递归深度不会被计数
//   - g: 本条递归链共享的护栏
//   - outermost: 本次调用是否为整条链的最外层，决定谁负责超限善后
//   - exceeded: 已达上限，调用方应立即返回错误让整条嵌套事务回滚
func enterAdvanceHop(ctx context.Context, nodeID string) (newCtx context.Context, g *advanceGuard, outermost, exceeded bool) {
	hop, _ := ctx.Value(advanceHopKeyType{}).(int)
	g, _ = ctx.Value(advanceGuardKeyType{}).(*advanceGuard)
	outermost = g == nil
	if outermost {
		g = &advanceGuard{}
		ctx = context.WithValue(ctx, advanceGuardKeyType{}, g)
	}
	g.path = append(g.path, nodeID)
	if hop >= maxAdvanceHops {
		g.limitHit = true
		return ctx, g, outermost, true
	}
	return context.WithValue(ctx, advanceHopKeyType{}, hop+1), g, outermost, false
}

// failInstanceForHopLimit 在独立事务中把实例终结为 FAILED。
//
// 为什么必须终结而不是让它重试：触到跳数上限意味着 DAG 里存在纯同步回环，
// 重试不可能让它收敛。若不终结，承载回调的 outbox 任务会一直重试到耗尽，
// 实例停在 RUNNING 永久卡死——正是本次可靠性改造要消除的状态。
//
// 注意：本方法吞掉自身错误，只记日志。它运行在推进已失败的善后路径上，
// 再抛错只会把一个可诊断的失败换成另一个。
func (a *App) failInstanceForHopLimit(ctx context.Context, instanceID int64, path []string) {
	err := mvc.ExtractDB(ctx, base.DB).Transaction(func(tx *gorm.DB) error {
		inst, err := a.InstanceService.FindByIdForUpdate(ctx, tx, instanceID)
		if err != nil {
			return err
		}
		if inst == nil {
			return nil
		}
		inst.Status = model.InstanceStatusFailed
		inst.ActiveNodeIDs = "[]"
		return a.InstanceService.SaveWithTx(ctx, tx, inst)
	})
	if err != nil {
		a.log.WithErr(err).
			WithField("instance_id", instanceID).
			Error("回环跳数超限后标记实例 FAILED 失败，实例可能停在 RUNNING")
		return
	}
	a.log.WithField("instance_id", instanceID).
		WithField("max_hops", maxAdvanceHops).
		WithField("path", path).
		Error("单次推进的同步递归跳数超限，DAG 存在纯 condition 回环，实例已终结为 FAILED")
}

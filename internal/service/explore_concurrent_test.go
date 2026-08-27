package service

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
)

// TestConcurrentExploreAndListingNoEmptyMidRotate 复现"二十位同事同时探索并拉取清单"
// 的并发场景：同一计划的探索（写入）与冲突列举（读取）必须互斥，清单不得在
// 「旧冲突已清空、新冲突尚未写完」的中间态被读成空。
//
// 该场景在修复前会失败：SaveConflicts 先 DELETE 再逐条 INSERT，期间夹了一个
// time.Sleep 用以放大竞态；而 Conflicts() 列举又不持有串行锁，于是列举者恰好在
// DELETE 之后、INSERT 之前读取，便会拿到空清单。修复后，SaveConflicts 的清空与
// 重写是单事务原子完成，且 Conflicts() 与探索在同一 serialMu 上互斥，列举者
// 要么读到替换前的完整旧清单、要么读到替换后的完整新清单，永不读空。
func TestConcurrentExploreAndListingNoEmptyMidRotate(t *testing.T) {
	const colleagues = 20
	const rounds = 40

	svc := newTestService(t)
	planID, _ := seedSplitPlan(t, svc)

	// 先做一次探索，确保计划已存在一份非空冲突清单，作为后续并发轮转的基线。
	// 经由 VerifyAndExplore 把计划推进到 verifying → conflicted，后续 RunExplore
	// 才能在合法状态下反复重写清单（conflicted 态稳定，无需再转换状态）。
	if cs, err := svc.VerifyAndExplore(context.Background(), planID); err != nil {
		t.Fatalf("baseline explore: %v", err)
	} else if len(cs) == 0 {
		t.Fatalf("baseline explore should seed a non-empty conflict list")
	}

	var stop atomic.Bool
	var emptyHits atomic.Int64
	var wg sync.WaitGroup

	// 一半同事不停探索（反复重写同计划冲突清单），一半不停拉取清单。
	for i := 0; i < colleagues; i++ {
		wg.Add(1)
		go func(role int) {
			defer wg.Done()
			for !stop.Load() {
				if role%2 == 0 {
					if _, err := svc.RunExplore(context.Background(), planID); err != nil {
						t.Errorf("explore err: %v", err)
						return
					}
				} else {
					cs, err := svc.Conflicts(planID)
					if err != nil {
						t.Errorf("list conflicts err: %v", err)
						return
					}
					// 探索总是会重新产出非空清单（区域 west 的 customer_name 高冲突），
					// 因此任何一次合法的完整读取都不应为空。读到空即说明撞上了
					// 清空-重写的中间态。
					if len(cs) == 0 {
						emptyHits.Add(1)
					}
				}
			}
		}(i)
	}

	// 跑足够多轮，让探索与列举充分交错。
	for r := 0; r < rounds; r++ {
		if _, err := svc.RunExplore(context.Background(), planID); err != nil {
			t.Fatalf("drive explore round %d: %v", r, err)
		}
	}

	stop.Store(true)
	wg.Wait()

	if n := emptyHits.Load(); n != 0 {
		t.Fatalf("列举在探索轮转期间读到空清单 %d 次（应恒非空）：探索与列举未互斥", n)
	}
}

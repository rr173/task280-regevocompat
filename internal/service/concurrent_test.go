package service

import (
	"context"
	"sync"
	"testing"

	"task280-regevocompat/internal/model"
)

// schemaVersionByTag 在已登记的 schema 版本中按 tag 查找。
func schemaVersionByTag(t *testing.T, svc *Service, tag string) model.SchemaVersionID {
	t.Helper()
	versions, err := svc.Store().ListSchemaVersions()
	if err != nil {
		t.Fatalf("list schema versions: %v", err)
	}
	for _, v := range versions {
		if v.Tag == tag {
			return v.ID
		}
	}
	t.Fatalf("schema version tag %q not found", tag)
	return ""
}

// TestConcurrentDeclareWindowAndExploreResolvesConflict 复现二十位同事并发声明窗口并重新探索的场景。
//
// 修复前：DeclareWindow 与 RunExplore 不共享串行锁，并发时探索可能读到「窗口尚未声明」的中间态，
// 按无窗口计算冲突并把计划置为 conflicted，导致即便声明了窗口也卡在冲突。
// 修复后：窗口声明与探索串行，声明后再探索必能看到该窗口并消解冲突。
func TestConcurrentDeclareWindowAndExploreResolvesConflict(t *testing.T) {
	svc := newTestService(t)
	planID, westID := seedSplitPlan(t, svc)
	v1ID := schemaVersionByTag(t, svc, "v1")
	v2ID := schemaVersionByTag(t, svc, "v2")

	// 先确认基线探索能发现 west 的未消解冲突。
	conflicts, err := svc.VerifyAndExplore(context.Background(), planID)
	if err != nil {
		t.Fatalf("baseline explore: %v", err)
	}
	foundBaseline := false
	for _, c := range conflicts {
		if c.RegionID == westID && c.Field == "customer_name" && !c.Resolved {
			foundBaseline = true
		}
	}
	if !foundBaseline {
		t.Fatalf("baseline should find unresolved west conflict, got %+v", conflicts)
	}

	// 二十位同事同时声明兼容窗口并立即重新探索。
	const N = 20
	adapterPayload := `[{"output":"customer_name","inputs":["first_name","last_name"],"op":"concat_space"}]`

	var wg sync.WaitGroup
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := svc.DeclareWindow(planID, v1ID, v2ID, model.RuleAdapter, adapterPayload); err != nil {
				t.Logf("declare window: %v", err)
			}
			if _, err := svc.RunExplore(context.Background(), planID); err != nil {
				t.Errorf("concurrent explore: %v", err)
			}
		}()
	}
	wg.Wait()

	// 所有同事结束后，最终一次探索应当读到全部已声明的窗口并把冲突消解。
	finalConflicts, err := svc.RunExplore(context.Background(), planID)
	if err != nil {
		t.Fatalf("final explore: %v", err)
	}
	for _, c := range finalConflicts {
		if c.RegionID == westID && c.Field == "customer_name" && !c.Resolved {
			t.Fatalf("声明窗口后探索仍存在未消解冲突，说明声明与探索交错：conflict=%+v", c)
		}
	}
	plan, err := svc.s.GetPlan(planID)
	if err != nil {
		t.Fatal(err)
	}
	if plan.State != model.PlanPublishable {
		t.Fatalf("声明窗口后计划应为 publishable，实际 %s（声明与探索未串行）", plan.State)
	}
}

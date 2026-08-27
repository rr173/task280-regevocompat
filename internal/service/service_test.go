package service

import (
	"context"
	"path/filepath"
	"testing"

	"task280-regevocompat/internal/compat"
	"task280-regevocompat/internal/migration"
	"task280-regevocompat/internal/model"
	"task280-regevocompat/internal/region"
	"task280-regevocompat/internal/schemaver"
	"task280-regevocompat/internal/store"
)

func newTestService(t *testing.T) *Service {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "svc.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return New(st)
}

func seedSplitPlan(t *testing.T, svc *Service) (model.PlanID, model.RegionID) {
	t.Helper()
	s := svc.Store()
	v1, err := schemaver.Register(s, "v1", []model.Field{{Name: "customer_name", Type: model.FieldString}})
	if err != nil {
		t.Fatal(err)
	}
	v2, err := schemaver.Register(s, "v2", []model.Field{
		{Name: "first_name", Type: model.FieldString},
		{Name: "last_name", Type: model.FieldString},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := region.Register(s, "east", v2.ID); err != nil {
		t.Fatal(err)
	}
	west, err := region.Register(s, "west", v1.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := region.MarkLagging(s, west.ID); err != nil {
		t.Fatal(err)
	}
	plan, err := migration.CreatePlan(s, "split", v1.ID, v2.ID)
	if err != nil {
		t.Fatal(err)
	}
	split, err := migration.AddStep(s, plan.ID, 0, model.StepSplit, "customer_name",
		[]string{"first_name", "last_name"}, true, false, "split")
	if err != nil {
		t.Fatal(err)
	}
	stop, err := migration.AddStep(s, plan.ID, 1, model.StepStopDualWrite, "customer_name",
		nil, false, true, "stop")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := migration.Advance(s, split.ID, model.StepDualWrite); err != nil {
		t.Fatal(err)
	}
	if _, err := migration.Advance(s, stop.ID, model.StepDualWrite); err != nil {
		t.Fatal(err)
	}
	if _, err := migration.Advance(s, stop.ID, model.StepBackfill); err != nil {
		t.Fatal(err)
	}
	if _, err := migration.Advance(s, stop.ID, model.StepFinalize); err != nil {
		t.Fatal(err)
	}
	return plan.ID, west.ID
}

func TestVerifyAndExploreFindsWestConflict(t *testing.T) {
	svc := newTestService(t)
	planID, westID := seedSplitPlan(t, svc)
	conflicts, err := svc.VerifyAndExplore(context.Background(), planID)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, c := range conflicts {
		if c.RegionID == westID && c.Field == "customer_name" && !c.Resolved {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected unresolved west customer_name conflict, got %+v", conflicts)
	}
}

// TestConflictListReflectsResolutionAfterReexplore 锁定回归：
// 同一计划先探索出未消解冲突，声明兼容窗口后重新探索，再通过
// svc.Conflicts（GET /api/plans/{id}/conflicts 走的缓存读路径）取冲突清单，
// 必须能看到消解结果，而非停留在第一轮的未消解结果。
func TestConflictListReflectsResolutionAfterReexplore(t *testing.T) {
	svc := newTestService(t)
	planID, westID := seedSplitPlan(t, svc)

	// 第一轮：探索出区域 west 的未消解冲突。
	if _, err := svc.VerifyAndExplore(context.Background(), planID); err != nil {
		t.Fatalf("explore #1: %v", err)
	}
	before, err := svc.Conflicts(planID)
	if err != nil {
		t.Fatalf("list conflicts #1: %v", err)
	}
	var unresolved bool
	for _, c := range before {
		if c.RegionID == westID && c.Field == "customer_name" {
			if c.Resolved {
				t.Fatalf("冲突在声明窗口前不应被消解: %+v", c)
			}
			unresolved = true
		}
	}
	if !unresolved {
		t.Fatalf("第一轮应列出未消解的 west customer_name 冲突，got %+v", before)
	}

	// 声明兼容窗口（读路径适配器重建 customer_name）消解冲突。
	adapterPayload := `[{"output":"customer_name","inputs":["first_name","last_name"],"op":"concat_space"}]`
	// 用第一轮清单里已落库的 reader/writer 版本 ID 声明窗口，避免对版本 ID 字面量的假设。
	var readerVer, writerVer model.SchemaVersionID
	for _, c := range before {
		if c.RegionID == westID && c.Field == "customer_name" {
			readerVer = c.ReaderVersionID
			writerVer = c.WriterVersionID
		}
	}
	if readerVer == "" || writerVer == "" {
		t.Fatalf("无法从第一轮冲突中取得 reader/writer 版本 ID")
	}
	if _, err := compat.DeclareWindow(svc.s, planID, readerVer, writerVer, model.RuleAdapter, adapterPayload); err != nil {
		t.Fatalf("declare window: %v", err)
	}

	// 第二轮：重新探索，写路径产出不变但窗口已覆盖，冲突应被标记为消解。
	if _, err := svc.RunExplore(context.Background(), planID); err != nil {
		t.Fatalf("explore #2: %v", err)
	}

	// 关键断言：走 svc.Conflicts（ListConflictsByPlan，原缓存读路径）必须看到消解结果。
	after, err := svc.Conflicts(planID)
	if err != nil {
		t.Fatalf("list conflicts #2: %v", err)
	}
	for _, c := range after {
		if c.RegionID == westID && c.Field == "customer_name" && !c.Resolved {
			t.Fatalf("声明窗口并重新探索后，冲突清单仍返回第一轮的未消解结果: %+v", c)
		}
	}
}

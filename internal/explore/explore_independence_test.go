package explore

import (
	"path/filepath"
	"testing"

	"task280-regevocompat/internal/migration"
	"task280-regevocompat/internal/model"
	"task280-regevocompat/internal/region"
	"task280-regevocompat/internal/schemaver"
	"task280-regevocompat/internal/store"
)

// TestExploreIndependentAcrossPlans 复现「列出甲计划的冲突路径后再列出乙计划，
// 甲计划那份清单被改成乙计划路径」的回归：两次 Explore 返回的切片必须互不影响。
// 根因是 Explore 曾复用包级 exploreScratch 切片并直接返回其底层数组，后一次调用
// 的 append 会覆写前一次返回切片的元素（底层数组共享）。
func TestExploreIndependentAcrossPlans(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "ex2.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })

	// v1: 单字段 customer_name；v2: 拆分为 first_name + last_name。
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

	// east 跑 v2（已升级，无冲突）；west 跑 v1（落后，产生 customer_name 冲突）。
	if _, err := region.Register(s, "east", v2.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := region.Register(s, "west", v1.ID); err != nil {
		t.Fatal(err)
	}

	// 甲计划：split customer_name + stop_dual_write，并推进到 finalize，使写路径丢弃 customer_name。
	planA, err := migration.CreatePlan(s, "splitA", v1.ID, v2.ID)
	if err != nil {
		t.Fatal(err)
	}
	splitA, err := migration.AddStep(s, planA.ID, 0, model.StepSplit, "customer_name",
		[]string{"first_name", "last_name"}, true, false, "splitA")
	if err != nil {
		t.Fatal(err)
	}
	stopA, err := migration.AddStep(s, planA.ID, 1, model.StepStopDualWrite, "customer_name",
		nil, false, true, "stopA")
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []model.StepID{splitA.ID, stopA.ID} {
		if _, err := migration.Advance(s, id, model.StepDualWrite); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := migration.Advance(s, stopA.ID, model.StepBackfill); err != nil {
		t.Fatal(err)
	}
	if _, err := migration.Advance(s, stopA.ID, model.StepFinalize); err != nil {
		t.Fatal(err)
	}

	// 乙计划：同样的拆分演进，独立计划 ID。
	planB, err := migration.CreatePlan(s, "splitB", v1.ID, v2.ID)
	if err != nil {
		t.Fatal(err)
	}
	splitB, err := migration.AddStep(s, planB.ID, 0, model.StepSplit, "customer_name",
		[]string{"first_name", "last_name"}, true, false, "splitB")
	if err != nil {
		t.Fatal(err)
	}
	stopB, err := migration.AddStep(s, planB.ID, 1, model.StepStopDualWrite, "customer_name",
		nil, false, true, "stopB")
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []model.StepID{splitB.ID, stopB.ID} {
		if _, err := migration.Advance(s, id, model.StepDualWrite); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := migration.Advance(s, stopB.ID, model.StepBackfill); err != nil {
		t.Fatal(err)
	}
	if _, err := migration.Advance(s, stopB.ID, model.StepFinalize); err != nil {
		t.Fatal(err)
	}

	// 先列甲，再列乙——正是报告里的操作顺序。
	gotA, err := Explore(s, planA)
	if err != nil {
		t.Fatal(err)
	}
	gotB, err := Explore(s, planB)
	if err != nil {
		t.Fatal(err)
	}

	// 列完乙之后，甲的切片不应被污染为乙计划的路径。
	for _, c := range gotA {
		if c.PlanID != planA.ID {
			t.Fatalf("甲计划清单被污染：期望 PlanID=%s，实际 %s", planA.ID, c.PlanID)
		}
	}
	for _, c := range gotB {
		if c.PlanID != planB.ID {
			t.Fatalf("乙计划清单有误：期望 PlanID=%s，实际 %s", planB.ID, c.PlanID)
		}
	}

	// 各自应至少捕获到 west 的 customer_name 未消解冲突。
	findWest := func(cs []*model.ConflictPath) bool {
		for _, c := range cs {
			if c.Field == "customer_name" && !c.Resolved {
				return true
			}
		}
		return false
	}
	if !findWest(gotA) {
		t.Fatalf("甲计划应检出 west customer_name 未消解冲突，got %+v", gotA)
	}
	if !findWest(gotB) {
		t.Fatalf("乙计划应检出 west customer_name 未消解冲突，got %+v", gotB)
	}
}

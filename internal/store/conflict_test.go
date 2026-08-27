package store

import (
	"path/filepath"
	"testing"

	"task280-regevocompat/internal/model"
)

// TestListConflictsByPlanIndependentAcrossPlans 复现「甲计划清单被改成乙计划路径」的回归：
// 先列出甲计划的冲突，再列出乙计划的冲突；两次返回的切片必须互不影响——甲的切片里
// 不应出现乙的 PlanID / 字段。根因是 ListConflictsByPlan 曾复用包级 conflictScratch
// 切片并直接返回其底层数组，导致后一次调用的覆写会污染前一次返回的切片。
func TestListConflictsByPlanIndependentAcrossPlans(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "conf.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })

	// 甲计划：一条冲突，字段 alpha。
	planA := model.PlanID("pl_a")
	if err := s.SaveConflicts(planA, []*model.ConflictPath{
		{
			ID:              "cf_a1",
			PlanID:          planA,
			RegionID:        "rg_a",
			ReaderVersionID: "sv_a",
			WriterVersionID: "sv_b",
			StepID:          "st_a",
			Field:           "alpha",
			Reason:          "甲计划冲突",
			Severity:        "high",
			Resolved:        false,
			DetectedAt:      100,
		},
	}); err != nil {
		t.Fatal(err)
	}
	// 乙计划：一条冲突，字段 beta。
	planB := model.PlanID("pl_b")
	if err := s.SaveConflicts(planB, []*model.ConflictPath{
		{
			ID:              "cf_b1",
			PlanID:          planB,
			RegionID:        "rg_b",
			ReaderVersionID: "sv_a",
			WriterVersionID: "sv_c",
			StepID:          "st_b",
			Field:           "beta",
			Reason:          "乙计划冲突",
			Severity:        "high",
			Resolved:        false,
			DetectedAt:      200,
		},
	}); err != nil {
		t.Fatal(err)
	}

	// 先列甲，再列乙——正是报告里的操作顺序。
	gotA, err := s.ListConflictsByPlan(planA)
	if err != nil {
		t.Fatal(err)
	}
	gotB, err := s.ListConflictsByPlan(planB)
	if err != nil {
		t.Fatal(err)
	}

	if len(gotA) != 1 {
		t.Fatalf("甲计划应返回 1 条冲突，实际 %d", len(gotA))
	}
	if len(gotB) != 1 {
		t.Fatalf("乙计划应返回 1 条冲突，实际 %d", len(gotB))
	}

	// 列完乙之后，甲的切片不应被污染为乙的路径。
	if gotA[0].PlanID != planA || gotA[0].Field != "alpha" {
		t.Fatalf("甲计划清单被污染：期望 plan=%s field=alpha，实际 plan=%s field=%s",
			planA, gotA[0].PlanID, gotA[0].Field)
	}
	if gotB[0].PlanID != planB || gotB[0].Field != "beta" {
		t.Fatalf("乙计划清单有误：期望 plan=%s field=beta，实际 plan=%s field=%s",
			planB, gotB[0].PlanID, gotB[0].Field)
	}
}

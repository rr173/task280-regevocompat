package service

import (
	"context"
	"fmt"
	"os"

	"task280-regevocompat/internal/compat"
	"task280-regevocompat/internal/migration"
	"task280-regevocompat/internal/model"
	"task280-regevocompat/internal/region"
	"task280-regevocompat/internal/schemaver"
	"task280-regevocompat/internal/semantic"
	"task280-regevocompat/internal/store"
)

// SmokeTest 执行确定性自检并验证持久化与重启恢复：
//  1. 登记 schema 版本 v1(customer_name) / v2(first_name,last_name) 及读写语义；
//  2. 登记区域副本 A(已升级到 v2) 与 B(滞后于 v1)；
//  3. 创建字段拆分演进计划，执行 split(双写) 与 stop_dual_write(停止双写)；
//  4. 探索冲突：区域 B 旧读路径期望 customer_name，写路径已停双写仅产出 first/last → 高冲突；
//  5. 声明兼容窗口（读路径适配器 customer_name = first_name + " " + last_name）消解冲突；
//  6. 发布不可变兼容快照并封存计划；
//  7. 关闭并重新打开数据库，验证状态、区域、冲突与快照均持久化恢复。
//
// 全部断言通过则返回 nil，否则返回描述性错误。
func SmokeTest(dbPath string) error {
	_ = os.Remove(dbPath)
	_ = os.Remove(dbPath + "-wal")
	_ = os.Remove(dbPath + "-shm")
	s, err := store.Open(dbPath)
	if err != nil {
		return fmt.Errorf("smoke open: %w", err)
	}
	svc := New(s)

	// 1. schema 版本与语义
	v1, err := schemaver.Register(s, "v1", []model.Field{{Name: "customer_name", Type: model.FieldString}})
	if err != nil {
		return fmt.Errorf("register v1: %w", err)
	}
	v2, err := schemaver.Register(s, "v2", []model.Field{
		{Name: "first_name", Type: model.FieldString},
		{Name: "last_name", Type: model.FieldString},
	})
	if err != nil {
		return fmt.Errorf("register v2: %w", err)
	}
	for _, v := range []*model.SchemaVersion{v1, v2} {
		if _, err := semantic.Register(s, v.ID, model.SemanticRead); err != nil {
			return fmt.Errorf("register read semantic: %w", err)
		}
		if _, err := semantic.Register(s, v.ID, model.SemanticWrite); err != nil {
			return fmt.Errorf("register write semantic: %w", err)
		}
	}

	// 2. 区域副本
	rA, err := region.Register(s, "region-east", v2.ID)
	if err != nil {
		return fmt.Errorf("register region A: %w", err)
	}
	if _, err := region.Upgrade(s, rA.ID, v2.ID); err != nil {
		return fmt.Errorf("upgrade region A: %w", err)
	}
	rB, err := region.Register(s, "region-west", v1.ID)
	if err != nil {
		return fmt.Errorf("register region B: %w", err)
	}
	if _, err := region.MarkLagging(s, rB.ID); err != nil {
		return fmt.Errorf("mark region B lagging: %w", err)
	}

	// 3. 演进计划与步骤
	plan, err := migration.CreatePlan(s, "field-split customer_name", v1.ID, v2.ID)
	if err != nil {
		return fmt.Errorf("create plan: %w", err)
	}
	split, err := migration.AddStep(s, plan.ID, 0, model.StepSplit, "customer_name",
		[]string{"first_name", "last_name"}, true, false, "拆分 customer_name 为 first_name+last_name，双写期保留旧字段")
	if err != nil {
		return fmt.Errorf("add split step: %w", err)
	}
	stop, err := migration.AddStep(s, plan.ID, 1, model.StepStopDualWrite, "customer_name",
		nil, false, true, "停止双写，写路径不再产出 customer_name")
	if err != nil {
		return fmt.Errorf("add stop step: %w", err)
	}
	if _, err := migration.Advance(s, split.ID, model.StepDualWrite); err != nil {
		return fmt.Errorf("advance split: %w", err)
	}
	if _, err := migration.Advance(s, stop.ID, model.StepDualWrite); err != nil {
		return fmt.Errorf("advance stop dual_write: %w", err)
	}
	if _, err := migration.Advance(s, stop.ID, model.StepBackfill); err != nil {
		return fmt.Errorf("advance stop backfill: %w", err)
	}
	if _, err := migration.Advance(s, stop.ID, model.StepFinalize); err != nil {
		return fmt.Errorf("advance stop finalize: %w", err)
	}

	// 4. 探索冲突
	conflicts, err := svc.VerifyAndExplore(context.Background(), plan.ID)
	if err != nil {
		return fmt.Errorf("explore #1: %w", err)
	}
	var bConflict *model.ConflictPath
	for _, c := range conflicts {
		if c.RegionID == rB.ID && c.Field == "customer_name" {
			bConflict = c
		}
	}
	if bConflict == nil {
		return fmt.Errorf("断言失败：区域 B 的 customer_name 冲突未被检测到（冲突数=%d）", len(conflicts))
	}
	if bConflict.Resolved {
		return fmt.Errorf("断言失败：区域 B 冲突在声明窗口前不应被消解")
	}
	if bConflict.Severity != "high" {
		return fmt.Errorf("断言失败：区域 B 冲突严重度应为 high，实际 %s", bConflict.Severity)
	}
	p1, err := s.GetPlan(plan.ID)
	if err != nil {
		return err
	}
	if p1.State != model.PlanConflicted {
		return fmt.Errorf("断言失败：存在未消解冲突时计划应为 conflicted，实际 %s", p1.State)
	}

	// 5. 声明兼容窗口消解冲突
	adapterPayload := `[{"output":"customer_name","inputs":["first_name","last_name"],"op":"concat_space"}]`
	if _, err := compat.DeclareWindow(s, plan.ID, v1.ID, v2.ID, model.RuleAdapter, adapterPayload); err != nil {
		return fmt.Errorf("declare window: %w", err)
	}
	conflicts, err = svc.RunExplore(context.Background(), plan.ID)
	if err != nil {
		return fmt.Errorf("explore #2: %w", err)
	}
	for _, c := range conflicts {
		if c.RegionID == rB.ID && c.Field == "customer_name" && !c.Resolved {
			return fmt.Errorf("断言失败：声明窗口后区域 B 冲突仍未被消解")
		}
	}
	p2, err := s.GetPlan(plan.ID)
	if err != nil {
		return err
	}
	if p2.State != model.PlanPublishable {
		return fmt.Errorf("断言失败：冲突全部消解后计划应为 publishable，实际 %s", p2.State)
	}

	// 6. 发布快照并封存
	if _, err := svc.Publish(context.Background(), plan.ID); err != nil {
		return fmt.Errorf("publish snapshot: %w", err)
	}
	if _, err := svc.Seal(context.Background(), plan.ID); err != nil {
		return fmt.Errorf("seal plan: %w", err)
	}

	// 7. 关闭并重新打开数据库，验证恢复
	planID := plan.ID
	regionBID := rB.ID
	if err := s.Close(); err != nil {
		return fmt.Errorf("close: %w", err)
	}
	s2, err := store.Open(dbPath)
	if err != nil {
		return fmt.Errorf("reopen: %w", err)
	}
	defer s2.Close()
	svc2 := New(s2)

	p3, err := svc2.Store().GetPlan(planID)
	if err != nil {
		return fmt.Errorf("reopen get plan: %w", err)
	}
	if p3.State != model.PlanSealed {
		return fmt.Errorf("恢复断言失败：计划应为 sealed，实际 %s", p3.State)
	}
	rb, err := svc2.Store().GetRegion(regionBID)
	if err != nil {
		return fmt.Errorf("reopen get region: %w", err)
	}
	if rb.State != model.RegionLagging {
		return fmt.Errorf("恢复断言失败：区域 B 应为 lagging，实际 %s", rb.State)
	}
	cs, err := svc2.Conflicts(planID)
	if err != nil {
		return fmt.Errorf("reopen conflicts: %w", err)
	}
	if len(cs) == 0 {
		return fmt.Errorf("恢复断言失败：冲突路径应持久化")
	}
	snaps, err := svc2.Store().ListSnapshotsByPlan(planID)
	if err != nil {
		return fmt.Errorf("reopen snapshots: %w", err)
	}
	published := false
	for _, sn := range snaps {
		if sn.State == model.SnapPublished {
			published = true
		}
	}
	if !published {
		return fmt.Errorf("恢复断言失败：应存在已发布的兼容快照")
	}

	fmt.Println("smoke-test OK: 场景构建、冲突定位、兼容窗口消解、快照发布与重启恢复全部通过")
	return nil
}

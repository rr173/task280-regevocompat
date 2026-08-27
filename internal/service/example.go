package service

import (
	"context"
	"fmt"

	"task280-regevocompat/internal/compat"
	"task280-regevocompat/internal/migration"
	"task280-regevocompat/internal/model"
	"task280-regevocompat/internal/region"
	"task280-regevocompat/internal/schemaver"
	"task280-regevocompat/internal/semantic"
	"task280-regevocompat/internal/store"
)

// SeedExample 在当前存储中构建一个完整示例场景，演示全流程：
//   - 登记 v1(customer_name) / v2(first_name,last_name) 与读写语义；
//   - 登记区域 A(已升级 v2) 与 B(滞后 v1)；
//   - 创建字段拆分演进计划并执行 split(双写) 与 stop_dual_write；
//   - 探索冲突（区域 B 的 customer_name 高冲突）→ 声明兼容窗口消解 → 重新探索；
//   - 发布不可变兼容快照并封存计划。
//
// 返回最终封存的计划，便于前端展示。
func SeedExample(s *store.Store) (*model.MigrationPlan, error) {
	svc := New(s)

	v1, err := schemaver.Register(s, "v1", []model.Field{{Name: "customer_name", Type: model.FieldString}})
	if err != nil {
		return nil, fmt.Errorf("register v1: %w", err)
	}
	v2, err := schemaver.Register(s, "v2", []model.Field{
		{Name: "first_name", Type: model.FieldString},
		{Name: "last_name", Type: model.FieldString},
	})
	if err != nil {
		return nil, fmt.Errorf("register v2: %w", err)
	}
	for _, v := range []*model.SchemaVersion{v1, v2} {
		if _, err := semantic.Register(s, v.ID, model.SemanticRead); err != nil {
			return nil, fmt.Errorf("register read semantic: %w", err)
		}
		if _, err := semantic.Register(s, v.ID, model.SemanticWrite); err != nil {
			return nil, fmt.Errorf("register write semantic: %w", err)
		}
	}

	rA, err := region.Register(s, "region-east", v2.ID)
	if err != nil {
		return nil, fmt.Errorf("register region A: %w", err)
	}
	if _, err := region.Upgrade(s, rA.ID, v2.ID); err != nil {
		return nil, fmt.Errorf("upgrade region A: %w", err)
	}
	rB, err := region.Register(s, "region-west", v1.ID)
	if err != nil {
		return nil, fmt.Errorf("register region B: %w", err)
	}
	if _, err := region.MarkLagging(s, rB.ID); err != nil {
		return nil, fmt.Errorf("mark region B lagging: %w", err)
	}

	plan, err := migration.CreatePlan(s, "example-field-split", v1.ID, v2.ID)
	if err != nil {
		return nil, fmt.Errorf("create plan: %w", err)
	}
	split, err := migration.AddStep(s, plan.ID, 0, model.StepSplit, "customer_name",
		[]string{"first_name", "last_name"}, true, false, "拆分 customer_name 为 first_name+last_name（双写保留旧字段）")
	if err != nil {
		return nil, fmt.Errorf("add split step: %w", err)
	}
	stop, err := migration.AddStep(s, plan.ID, 1, model.StepStopDualWrite, "customer_name",
		nil, false, true, "停止双写，写路径不再产出 customer_name")
	if err != nil {
		return nil, fmt.Errorf("add stop step: %w", err)
	}
	if _, err := migration.Advance(s, split.ID, model.StepDualWrite); err != nil {
		return nil, fmt.Errorf("advance split: %w", err)
	}
	if _, err := migration.Advance(s, stop.ID, model.StepDualWrite); err != nil {
		return nil, fmt.Errorf("advance stop dual_write: %w", err)
	}
	if _, err := migration.Advance(s, stop.ID, model.StepBackfill); err != nil {
		return nil, fmt.Errorf("advance stop backfill: %w", err)
	}
	if _, err := migration.Advance(s, stop.ID, model.StepFinalize); err != nil {
		return nil, fmt.Errorf("advance stop finalize: %w", err)
	}

	if _, err := svc.VerifyAndExplore(context.Background(), plan.ID); err != nil {
		return nil, fmt.Errorf("explore #1: %w", err)
	}
	adapterPayload := `[{"output":"customer_name","inputs":["first_name","last_name"],"op":"concat_space"}]`
	if _, err := compat.DeclareWindow(s, plan.ID, v1.ID, v2.ID, model.RuleAdapter, adapterPayload); err != nil {
		return nil, fmt.Errorf("declare window: %w", err)
	}
	if _, err := svc.RunExplore(context.Background(), plan.ID); err != nil {
		return nil, fmt.Errorf("explore #2: %w", err)
	}
	if _, err := svc.Publish(context.Background(), plan.ID); err != nil {
		return nil, fmt.Errorf("publish snapshot: %w", err)
	}
	if _, err := svc.Seal(context.Background(), plan.ID); err != nil {
		return nil, fmt.Errorf("seal plan: %w", err)
	}
	return s.GetPlan(plan.ID)
}

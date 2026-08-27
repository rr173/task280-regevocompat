// Package migration 维护演进计划与迁移步骤的状态流转，并推导写路径字段集合。
package migration

import (
	"fmt"

	"task280-regevocompat/internal/model"
	"task280-regevocompat/internal/store"
)

// CreatePlan 创建一个演进计划（草稿态），需指定基线版本与目标版本。
func CreatePlan(s *store.Store, name string, baseline, target model.SchemaVersionID) (*model.MigrationPlan, error) {
	if name == "" {
		return nil, fmt.Errorf("%w: plan name required", model.ErrInvalidArgument)
	}
	if baseline == "" || target == "" {
		return nil, fmt.Errorf("%w: baseline and target version required", model.ErrInvalidArgument)
	}
	if baseline == target {
		return nil, fmt.Errorf("%w: baseline and target must differ", model.ErrInvalidArgument)
	}
	if _, err := s.GetSchemaVersion(baseline); err != nil {
		return nil, fmt.Errorf("baseline version: %w", err)
	}
	if _, err := s.GetSchemaVersion(target); err != nil {
		return nil, fmt.Errorf("target version: %w", err)
	}
	p := &model.MigrationPlan{
		ID:                model.PlanID(model.GenID("pl")),
		Name:              name,
		State:             model.PlanDraft,
		BaselineVersionID: baseline,
		TargetVersionID:   target,
		SealedAt:          0,
		CreatedAt:         store.NowMillis(),
	}
	if err := s.SavePlan(p); err != nil {
		return nil, err
	}
	return p, nil
}

// Get 按 ID 读取计划。
func Get(s *store.Store, id model.PlanID) (*model.MigrationPlan, error) {
	return s.GetPlan(id)
}

// List 列出全部计划。
func List(s *store.Store) ([]*model.MigrationPlan, error) {
	return s.ListPlans()
}

// Verify 将计划置为验证中（draft/conflicted -> verifying）。
func Verify(s *store.Store, id model.PlanID) (*model.MigrationPlan, error) {
	p, err := s.GetPlan(id)
	if err != nil {
		return nil, err
	}
	if !model.CanTransition("plan", p.State, model.PlanVerifying) {
		return nil, fmt.Errorf("%w: plan %s state %s -> verifying", model.ErrInvalidState, id, p.State)
	}
	p.State = model.PlanVerifying
	if err := s.SavePlan(p); err != nil {
		return nil, err
	}
	return p, nil
}

// SetPublishable 将计划置为可发布（验证通过、无未消解冲突时）。
func SetPublishable(s *store.Store, id model.PlanID) (*model.MigrationPlan, error) {
	p, err := s.GetPlan(id)
	if err != nil {
		return nil, err
	}
	if !model.CanTransition("plan", p.State, model.PlanPublishable) {
		return nil, fmt.Errorf("%w: plan %s state %s -> publishable", model.ErrInvalidState, id, p.State)
	}
	p.State = model.PlanPublishable
	if err := s.SavePlan(p); err != nil {
		return nil, err
	}
	return p, nil
}

// SetConflicted 将计划置为存在冲突。
func SetConflicted(s *store.Store, id model.PlanID) (*model.MigrationPlan, error) {
	p, err := s.GetPlan(id)
	if err != nil {
		return nil, err
	}
	if !model.CanTransition("plan", p.State, model.PlanConflicted) {
		return nil, fmt.Errorf("%w: plan %s state %s -> conflicted", model.ErrInvalidState, id, p.State)
	}
	p.State = model.PlanConflicted
	if err := s.SavePlan(p); err != nil {
		return nil, err
	}
	return p, nil
}

// Seal 封存计划（publishable -> sealed，终态不可改写）。
func Seal(s *store.Store, id model.PlanID) (*model.MigrationPlan, error) {
	p, err := s.GetPlan(id)
	if err != nil {
		return nil, err
	}
	if !model.CanTransition("plan", p.State, model.PlanSealed) {
		return nil, fmt.Errorf("%w: plan %s state %s -> sealed", model.ErrInvalidState, id, p.State)
	}
	p.State = model.PlanSealed
	p.SealedAt = store.NowMillis()
	if err := s.SavePlan(p); err != nil {
		return nil, err
	}
	return p, nil
}

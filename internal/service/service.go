// Package service 编排各业务包，提供探索落库、状态裁决与示例场景。
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"task280-regevocompat/internal/compat"
	"task280-regevocompat/internal/explore"
	"task280-regevocompat/internal/migration"
	"task280-regevocompat/internal/model"
	"task280-regevocompat/internal/schemaver"
	"task280-regevocompat/internal/store"
)

// Service 持有存储句柄，提供高层编排。
type Service struct {
	s        *store.Store
	serialMu sync.Mutex
}

// New 构造 Service。
func New(s *store.Store) *Service { return &Service{s: s} }

// Store 暴露底层存储。
func (svc *Service) Store() *store.Store { return svc.s }

// RegisterSchema 登记 schema 版本并保持哨兵错误可 errors.Is。
func (svc *Service) RegisterSchema(tag string, fields []model.Field) (*model.SchemaVersion, error) {
	v, err := schemaver.Register(svc.s, tag, fields)
	if err != nil {
		return nil, fmt.Errorf("register schema: %w", err)
	}
	return v, nil
}

// GetPlan 读取演进计划并保持哨兵错误可 errors.Is。
func (svc *Service) GetPlan(id model.PlanID) (*model.MigrationPlan, error) {
	p, err := svc.s.GetPlan(id)
	if err != nil {
		return nil, fmt.Errorf("get plan: %w", err)
	}
	return p, nil
}

// GetSnapshot 返回已落库的兼容快照，不再按 live 冲突重算内容。
func (svc *Service) GetSnapshot(id model.SnapshotID) (*model.CompatSnapshot, error) {
	snap, err := svc.s.GetSnapshot(id)
	if err != nil {
		return nil, fmt.Errorf("get snapshot: %w", err)
	}
	return snap, nil
}

// ListSnapshots 列出计划的已落库快照。
func (svc *Service) ListSnapshots(planID model.PlanID) ([]*model.CompatSnapshot, error) {
	return svc.s.ListSnapshotsByPlan(planID)
}

// RunExplore 执行冲突探索：持久化冲突路径，并依据未消解冲突将计划置为 conflicted / publishable。
func (svc *Service) RunExplore(ctx context.Context, planID model.PlanID) ([]*model.ConflictPath, error) {
	return svc.runExploreLocked(ctx, planID)
}

// VerifyAndExplore 将计划置为验证中并执行探索。
func (svc *Service) VerifyAndExplore(ctx context.Context, planID model.PlanID) ([]*model.ConflictPath, error) {
	svc.serialMu.Lock()
	defer svc.serialMu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if _, err := migration.Verify(svc.s, planID); err != nil {
		return nil, err
	}
	return svc.runExploreLocked(ctx, planID)
}

func (svc *Service) runExploreLocked(ctx context.Context, planID model.PlanID) ([]*model.ConflictPath, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	plan, err := svc.s.GetPlan(planID)
	if err != nil {
		return nil, err
	}
	conflicts, err := explore.Explore(svc.s, plan)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := svc.s.SaveConflicts(planID, conflicts); err != nil {
		return nil, err
	}
	hasUnresolved := false
	for _, c := range conflicts {
		if !c.Resolved {
			hasUnresolved = true
			break
		}
	}
	if hasUnresolved {
		if plan.State != model.PlanConflicted {
			if _, err := migration.SetConflicted(svc.s, planID); err != nil {
				return nil, err
			}
		}
	} else {
		if plan.State != model.PlanPublishable {
			if _, err := migration.SetPublishable(svc.s, planID); err != nil {
				return nil, err
			}
		}
	}
	return conflicts, nil
}

// Conflicts 返回计划的冲突路径。
func (svc *Service) Conflicts(planID model.PlanID) ([]*model.ConflictPath, error) {
	return svc.s.ListConflictsByPlan(planID)
}

// DeclareWindow 在同计划串行锁内声明兼容窗口，避免与探索交错。
func (svc *Service) DeclareWindow(planID model.PlanID, reader, writer model.SchemaVersionID, ruleType model.WindowRuleType, payload string) (*model.CompatWindow, error) {
	svc.serialMu.Lock()
	defer svc.serialMu.Unlock()
	return compat.DeclareWindow(svc.s, planID, reader, writer, ruleType, payload)
}

// Seal 在同计划串行锁内封存计划。
func (svc *Service) Seal(ctx context.Context, planID model.PlanID) (*model.MigrationPlan, error) {
	svc.serialMu.Lock()
	defer svc.serialMu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return migration.Seal(svc.s, planID)
}

// Publish 在同计划串行锁内发布不可变兼容快照。
func (svc *Service) Publish(ctx context.Context, planID model.PlanID) (*model.CompatSnapshot, error) {
	svc.serialMu.Lock()
	defer svc.serialMu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	plan, err := svc.s.GetPlan(planID)
	if err != nil {
		return nil, err
	}
	conflicts, err := svc.s.ListConflictsByPlan(planID)
	if err != nil {
		return nil, err
	}
	content, err := BuildVerificationContent(plan, conflicts)
	if err != nil {
		return nil, err
	}
	return compat.PublishSnapshot(svc.s, planID, content)
}

// BuildVerificationContent 汇总验证结果，生成兼容快照内容（JSON）。
func BuildVerificationContent(plan *model.MigrationPlan, conflicts []*model.ConflictPath) (string, error) {
	unresolved := 0
	for _, c := range conflicts {
		if !c.Resolved {
			unresolved++
		}
	}
	payload := map[string]interface{}{
		"plan_id":         string(plan.ID),
		"plan_name":       plan.Name,
		"plan_state":      plan.State,
		"total_conflicts": len(conflicts),
		"unresolved":      unresolved,
		"compatible":      unresolved == 0,
	}
	b, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal verification content: %w", err)
	}
	return string(b), nil
}

// Package compat 维护兼容窗口的声明/撤销与不可变兼容快照的发布。
package compat

import (
	"fmt"

	"task280-regevocompat/internal/model"
	"task280-regevocompat/internal/store"
)

// DeclareWindow 为计划声明一个兼容窗口（覆盖 reader×writer 组合并给出消解规则）。
func DeclareWindow(s *store.Store, planID model.PlanID, reader, writer model.SchemaVersionID,
	ruleType model.WindowRuleType, payload string) (*model.CompatWindow, error) {
	if reader == "" || writer == "" {
		return nil, fmt.Errorf("%w: reader and writer version required", model.ErrInvalidArgument)
	}
	if _, err := s.GetSchemaVersion(reader); err != nil {
		return nil, fmt.Errorf("reader version: %w", err)
	}
	if _, err := s.GetSchemaVersion(writer); err != nil {
		return nil, fmt.Errorf("writer version: %w", err)
	}
	if ruleType != model.RuleAdapter && ruleType != model.RuleUpgradeRequired {
		return nil, fmt.Errorf("%w: unknown rule type %q", model.ErrInvalidArgument, ruleType)
	}
	w := &model.CompatWindow{
		ID:              model.WindowID(model.GenID("cw")),
		PlanID:          planID,
		ReaderVersionID: reader,
		WriterVersionID: writer,
		RuleType:        ruleType,
		RulePayload:     payload,
		State:           model.WindowActive,
		ValidFrom:       store.NowMillis(),
		ValidTo:         0,
		CreatedAt:       store.NowMillis(),
	}
	if err := s.SaveWindow(w); err != nil {
		return nil, err
	}
	return w, nil
}

// RevokeWindow 撤销一个兼容窗口（active -> revoked）。
func RevokeWindow(s *store.Store, id model.WindowID) (*model.CompatWindow, error) {
	w, err := s.GetWindow(id)
	if err != nil {
		return nil, err
	}
	if w.State != model.WindowActive {
		return nil, fmt.Errorf("%w: window %s not active", model.ErrInvalidState, id)
	}
	w.State = model.WindowRevoked
	if err := s.SaveWindow(w); err != nil {
		return nil, err
	}
	return w, nil
}

// ListWindows 列出计划的兼容窗口。
func ListWindows(s *store.Store, planID model.PlanID) ([]*model.CompatWindow, error) {
	return s.ListWindowsByPlan(planID)
}

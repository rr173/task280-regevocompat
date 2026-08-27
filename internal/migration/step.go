package migration

import (
	"encoding/json"
	"fmt"

	"task280-regevocompat/internal/model"
	"task280-regevocompat/internal/store"
)

// AddStep 向计划追加一个迁移步骤。split 类步骤可声明双写；stop_dual_write 步骤声明停止双写后丢弃的字段。
func AddStep(s *store.Store, planID model.PlanID, ordinal int, kind model.StepKind,
	from string, to []string, dualWrite, stopDualWrite bool, desc string) (*model.MigrationStep, error) {
	p, err := s.GetPlan(planID)
	if err != nil {
		return nil, err
	}
	if p.State == model.PlanSealed {
		return nil, fmt.Errorf("%w: cannot add step to sealed plan", model.ErrSealed)
	}
	if ordinal < 0 {
		return nil, fmt.Errorf("%w: ordinal must be >= 0", model.ErrInvalidArgument)
	}
	switch kind {
	case model.StepSplit:
		if from == "" || len(to) == 0 {
			return nil, fmt.Errorf("%w: split requires from_field and to_fields", model.ErrInvalidArgument)
		}
	case model.StepStopDualWrite:
		if from == "" {
			return nil, fmt.Errorf("%w: stop_dual_write requires from_field", model.ErrInvalidArgument)
		}
		stopDualWrite = true
	case model.StepAdd, model.StepDrop, model.StepRename, model.StepMerge, model.StepTypeChange:
		if from == "" {
			return nil, fmt.Errorf("%w: %s requires from_field", model.ErrInvalidArgument, kind)
		}
	default:
		return nil, fmt.Errorf("%w: unknown step kind %q", model.ErrInvalidArgument, kind)
	}
	toJSON, err := json.Marshal(to)
	if err != nil {
		return nil, fmt.Errorf("marshal to_fields: %w", err)
	}
	st := &model.MigrationStep{
		ID:            model.StepID(model.GenID("st")),
		PlanID:        planID,
		Ordinal:       ordinal,
		Kind:          kind,
		Description:   desc,
		FromField:     from,
		ToFieldsJSON:  string(toJSON),
		DualWrite:     dualWrite,
		StopDualWrite: stopDualWrite,
		State:         model.StepPending,
		CreatedAt:     store.NowMillis(),
	}
	if err := s.SaveStep(st); err != nil {
		return nil, err
	}
	return st, nil
}

// Advance 推进迁移步骤状态（需满足状态机流转）。
func Advance(s *store.Store, stepID model.StepID, to string) (*model.MigrationStep, error) {
	st, err := s.GetStep(stepID)
	if err != nil {
		return nil, err
	}
	if !model.CanTransition("step", st.State, to) {
		return nil, fmt.Errorf("%w: step %s state %s -> %s", model.ErrInvalidState, stepID, st.State, to)
	}
	st.State = to
	if err := s.SaveStep(st); err != nil {
		return nil, err
	}
	return st, nil
}

// ListSteps 列出计划的迁移步骤（按 ordinal）。
func ListSteps(s *store.Store, planID model.PlanID) ([]*model.MigrationStep, error) {
	return s.ListStepsByPlan(planID)
}

// ToFields 解析步骤的 to_fields_json。
func ToFields(st *model.MigrationStep) []string {
	var out []string
	if st.ToFieldsJSON == "" {
		return out
	}
	_ = json.Unmarshal([]byte(st.ToFieldsJSON), &out)
	return out
}

// stepExecuted 判断步骤是否已对写路径产生作用（dual_write/backfill/finalize 视为已执行）。
func stepExecuted(state string) bool {
	switch state {
	case model.StepDualWrite, model.StepBackfill, model.StepFinalize:
		return true
	default:
		return false
	}
}

// WriterFields 推导当前迁移阶段下，写路径实际产出的字段集合。
//
// 规则：
//   - 初始：写路径产出基线版本(V1)字段。
//   - 已执行的 split 步骤：追加 to_fields，并保留 from_field（双写）。
//   - 已执行的 stop_dual_write 步骤：从写路径集合移除 from_field（停止双写，旧字段不再写出）。
func WriterFields(baseline *model.SchemaVersion, steps []*model.MigrationStep) []string {
	set := map[string]bool{}
	for _, f := range baseline.Fields {
		set[f.Name] = true
	}
	for _, st := range steps {
		if !stepExecuted(st.State) {
			continue
		}
		switch st.Kind {
		case model.StepSplit:
			for _, t := range ToFields(st) {
				set[t] = true // 追加新字段
			}
			set[st.FromField] = true // 双写保留旧字段
		case model.StepAdd:
			set[st.FromField] = true
		case model.StepStopDualWrite:
			delete(set, st.FromField) // 停止双写，丢弃旧字段
		}
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	return out
}

// Package explore 枚举「区域读版本 × 写阶段字段集合」组合，定位不兼容路径。
package explore

import (
	"fmt"

	"task280-regevocompat/internal/migration"
	"task280-regevocompat/internal/model"
	"task280-regevocompat/internal/semantic"
	"task280-regevocompat/internal/store"
)

var exploreScratch []*model.ConflictPath

// Explore 对一个演进计划执行冲突探索：
//   - 写路径字段集合由已执行步骤推导（migration.WriterFields）。
//   - 对每个区域副本，以其当前读版本解释写路径产出字段；缺失且无法被兼容窗口适配器
//     重建的读版本字段即为冲突（高严重度：数据误解释）。
//
// 返回全部检测到的不兼容组合，Resolved 标记是否已被兼容窗口消解。
func Explore(s *store.Store, plan *model.MigrationPlan) ([]*model.ConflictPath, error) {
	regions, err := s.ListRegions()
	if err != nil {
		return nil, fmt.Errorf("list regions: %w", err)
	}
	baseline, err := s.GetSchemaVersion(plan.BaselineVersionID)
	if err != nil {
		return nil, fmt.Errorf("baseline version: %w", err)
	}
	target, err := s.GetSchemaVersion(plan.TargetVersionID)
	if err != nil {
		return nil, fmt.Errorf("target version: %w", err)
	}
	steps, err := s.ListStepsByPlan(plan.ID)
	if err != nil {
		return nil, fmt.Errorf("list steps: %w", err)
	}
	windows, err := s.ListWindowsByPlan(plan.ID)
	if err != nil {
		return nil, fmt.Errorf("list windows: %w", err)
	}

	writerFields := migration.WriterFields(baseline, steps)
	writerSet := map[string]bool{}
	for _, f := range writerFields {
		writerSet[f] = true
	}

	// 收集覆盖 (reader, writer) 组合的活动窗口适配器。
	adaptersByPair := map[string][]semantic.Adapter{}
	for _, w := range windows {
		if w.State != model.WindowActive {
			continue
		}
		adps, err := semantic.ParseAdapters(w.RulePayload)
		if err != nil {
			return nil, fmt.Errorf("parse window %s adapters: %w", w.ID, err)
		}
		key := string(w.ReaderVersionID) + "->" + string(w.WriterVersionID)
		adaptersByPair[key] = append(adaptersByPair[key], adps...)
	}

	// 找到丢弃某字段的 stop_dual_write 步骤，用于冲突溯源。
	dropStep := map[string]model.StepID{}
	for _, st := range steps {
		if st.Kind == model.StepStopDualWrite && st.FromField != "" {
			dropStep[st.FromField] = st.ID
		}
	}

	exploreScratch = exploreScratch[:0]
	var conflicts []*model.ConflictPath
	now := store.NowMillis()
	for _, r := range regions {
		reader, err := s.GetSchemaVersion(r.CurrentVersionID)
		if err != nil {
			return nil, fmt.Errorf("region %s version: %w", r.ID, err)
		}
		key := string(reader.ID) + "->" + string(target.ID)
		adapters := adaptersByPair[key]

		for _, f := range reader.Fields {
			if writerSet[f.Name] {
				continue // 写路径产出该字段，读路径可读到
			}
			// 写路径不产出该字段，尝试适配器重建
			rebuilt, ok := semantic.ApplyAdapterByFields(f.Name, writerFields, adapters)
			resolved := ok
			cp := &model.ConflictPath{
				ID:              model.ConflictID(model.GenID("cf")),
				PlanID:          plan.ID,
				RegionID:        r.ID,
				ReaderVersionID: reader.ID,
				WriterVersionID: target.ID,
				StepID:          dropStep[f.Name],
				Field:           f.Name,
				Severity:        "high",
				Resolved:        resolved,
				DetectedAt:      now,
			}
			if resolved {
				cp.Reason = fmt.Sprintf("字段 %s 写路径未产出，但被兼容窗口适配器重建（%s）", f.Name, rebuilt)
			} else {
				cp.Reason = fmt.Sprintf("区域 %s 读版本 %s 期望字段 %s，但写路径当前仅产出 %v，且无兼容窗口覆盖",
					r.Name, reader.Tag, f.Name, writerFields)
			}
			exploreScratch = append(exploreScratch, cp)
			conflicts = exploreScratch
		}
	}
	return conflicts, nil
}

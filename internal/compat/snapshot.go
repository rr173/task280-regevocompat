package compat

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"task280-regevocompat/internal/model"
	"task280-regevocompat/internal/store"
)

// PublishSnapshot 发布一个不可变兼容快照：先将同计划已发布的快照置为 superseded，再写入新快照。
func PublishSnapshot(s *store.Store, planID model.PlanID, content string) (*model.CompatSnapshot, error) {
	if content == "" {
		return nil, fmt.Errorf("%w: snapshot content required", model.ErrInvalidArgument)
	}
	prev, err := s.ListSnapshotsByPlan(planID)
	if err != nil {
		return nil, err
	}
	newID := model.SnapshotID(model.GenID("sn"))
	now := store.NowMillis()
	for _, p := range prev {
		if p.State == model.SnapPublished {
			p.State = model.SnapSuperseded
			p.SupersededBy = newID
			if err := s.SaveSnapshot(p); err != nil {
				return nil, err
			}
		}
	}
	h := sha256.Sum256([]byte(content))
	snap := &model.CompatSnapshot{
		ID:          newID,
		PlanID:      planID,
		State:       model.SnapPublished,
		ContentJSON: content,
		Hash:        hex.EncodeToString(h[:])[:16],
		CreatedAt:   now,
	}
	if err := s.SaveSnapshot(snap); err != nil {
		return nil, err
	}
	return snap, nil
}

// Supersede 将指定快照置为被替代（published -> superseded）。
func Supersede(s *store.Store, id model.SnapshotID, by model.SnapshotID) (*model.CompatSnapshot, error) {
	snap, err := s.GetSnapshot(id)
	if err != nil {
		return nil, err
	}
	if !model.CanTransition("snapshot", snap.State, model.SnapSuperseded) {
		return nil, fmt.Errorf("%w: snapshot %s state %s -> superseded", model.ErrInvalidState, id, snap.State)
	}
	snap.State = model.SnapSuperseded
	snap.SupersededBy = by
	if err := s.SaveSnapshot(snap); err != nil {
		return nil, err
	}
	return snap, nil
}

// ListSnapshots 列出计划的兼容快照。
func ListSnapshots(s *store.Store, planID model.PlanID) ([]*model.CompatSnapshot, error) {
	return s.ListSnapshotsByPlan(planID)
}

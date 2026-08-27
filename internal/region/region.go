// Package region 维护区域副本的登记与版本状态流转。
package region

import (
	"fmt"

	"task280-regevocompat/internal/model"
	"task280-regevocompat/internal/store"
)

// Register 登记一个区域副本，初始状态为 old，运行给定 schema 版本。
func Register(s *store.Store, name string, versionID model.SchemaVersionID) (*model.RegionReplica, error) {
	if name == "" {
		return nil, fmt.Errorf("%w: region name required", model.ErrInvalidArgument)
	}
	if versionID == "" {
		return nil, fmt.Errorf("%w: version required", model.ErrInvalidArgument)
	}
	if _, err := s.GetSchemaVersion(versionID); err != nil {
		return nil, fmt.Errorf("region references unknown version: %w", err)
	}
	r := &model.RegionReplica{
		ID:               model.RegionID(model.GenID("rg")),
		Name:             name,
		CurrentVersionID: versionID,
		State:            model.RegionOld,
		UpgradedAt:       0,
		CreatedAt:        store.NowMillis(),
	}
	if err := s.SaveRegion(r); err != nil {
		return nil, err
	}
	return r, nil
}

// Get 按 ID 读取区域副本。
func Get(s *store.Store, id model.RegionID) (*model.RegionReplica, error) {
	return s.GetRegion(id)
}

// List 列出全部区域副本。
func List(s *store.Store) ([]*model.RegionReplica, error) {
	return s.ListRegions()
}

// SetVersion 设置区域副本当前运行的 schema 版本（不触发状态流转，仅更新版本指针）。
func SetVersion(s *store.Store, id model.RegionID, versionID model.SchemaVersionID) (*model.RegionReplica, error) {
	r, err := s.GetRegion(id)
	if err != nil {
		return nil, err
	}
	if _, err := s.GetSchemaVersion(versionID); err != nil {
		return nil, fmt.Errorf("region references unknown version: %w", err)
	}
	r.CurrentVersionID = versionID
	if err := s.SaveRegion(r); err != nil {
		return nil, err
	}
	return r, nil
}

// Upgrade 将区域副本标记为已升级到给定版本（状态流转 old/transitional/lagging -> upgraded）。
func Upgrade(s *store.Store, id model.RegionID, versionID model.SchemaVersionID) (*model.RegionReplica, error) {
	r, err := s.GetRegion(id)
	if err != nil {
		return nil, err
	}
	if !model.CanTransition("region", r.State, model.RegionUpgraded) {
		return nil, fmt.Errorf("%w: region %s state %s -> upgraded", model.ErrInvalidState, id, r.State)
	}
	r.State = model.RegionUpgraded
	r.CurrentVersionID = versionID
	r.UpgradedAt = store.NowMillis()
	if err := s.SaveRegion(r); err != nil {
		return nil, err
	}
	return r, nil
}

// MarkLagging 将区域副本标记为滞后（落后于目标版本）。
func MarkLagging(s *store.Store, id model.RegionID) (*model.RegionReplica, error) {
	r, err := s.GetRegion(id)
	if err != nil {
		return nil, err
	}
	if !model.CanTransition("region", r.State, model.RegionLagging) {
		return nil, fmt.Errorf("%w: region %s state %s -> lagging", model.ErrInvalidState, id, r.State)
	}
	r.State = model.RegionLagging
	if err := s.SaveRegion(r); err != nil {
		return nil, err
	}
	return r, nil
}

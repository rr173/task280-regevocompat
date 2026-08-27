package model

// 状态机：演进计划、区域副本、迁移步骤、兼容快照的合法流转。

// Plan 状态。
const (
	PlanDraft       = "draft"
	PlanVerifying   = "verifying"
	PlanConflicted  = "conflicted"
	PlanPublishable = "publishable"
	PlanSealed      = "sealed"
)

// Region 状态。
const (
	RegionOld          = "old"          // 旧版本
	RegionTransitional = "transitional" // 过渡
	RegionUpgraded     = "upgraded"     // 已升级
	RegionLagging      = "lagging"      // 滞后
)

// Step 状态。
const (
	StepPending   = "pending"    // 待执行
	StepDualWrite = "dual_write" // 双写
	StepBackfill  = "backfill"   // 回填
	StepFinalize  = "finalize"   // 收尾
	StepRollback  = "rollback"    // 撤销
)

// Snapshot 状态。
const (
	SnapDraft      = "draft"
	SnapPublished  = "published"
	SnapSuperseded = "superseded"
)

// CompatWindow 状态。
const (
	WindowActive  = "active"
	WindowRevoked = "revoked"
)

// planTransitions 演进计划的合法流转。
var planTransitions = map[string][]string{
	PlanDraft:       {PlanVerifying},
	PlanVerifying:   {PlanConflicted, PlanPublishable, PlanDraft},
	PlanConflicted:  {PlanVerifying, PlanDraft, PlanPublishable},
	PlanPublishable: {PlanSealed, PlanVerifying},
	PlanSealed:      {}, // 终态
}

// regionTransitions 区域副本的合法流转。
var regionTransitions = map[string][]string{
	RegionOld:          {RegionTransitional, RegionUpgraded, RegionLagging},
	RegionTransitional: {RegionUpgraded, RegionLagging, RegionOld},
	RegionUpgraded:     {RegionLagging, RegionTransitional},
	RegionLagging:      {RegionUpgraded, RegionTransitional, RegionOld},
}

// stepTransitions 迁移步骤的合法流转。
var stepTransitions = map[string][]string{
	StepPending:   {StepDualWrite, StepRollback},
	StepDualWrite: {StepBackfill, StepRollback},
	StepBackfill:  {StepFinalize, StepRollback},
	StepFinalize:  {StepRollback},
	StepRollback:  {}, // 终态
}

// snapshotTransitions 兼容快照的合法流转。
var snapshotTransitions = map[string][]string{
	SnapDraft:      {SnapPublished},
	SnapPublished:  {SnapSuperseded},
	SnapSuperseded: {}, // 终态
}

// CanTransition 判断 from -> to 是否合法。
func CanTransition(kind, from, to string) bool {
	var m map[string][]string
	switch kind {
	case "plan":
		m = planTransitions
	case "region":
		m = regionTransitions
	case "step":
		m = stepTransitions
	case "snapshot":
		m = snapshotTransitions
	default:
		return false
	}
	for _, s := range m[from] {
		if s == to {
			return true
		}
	}
	return false
}

// IsTerminal 判断某状态是否为终态。
func IsTerminal(kind, state string) bool {
	var m map[string][]string
	switch kind {
	case "plan":
		m = planTransitions
	case "region":
		m = regionTransitions
	case "step":
		m = stepTransitions
	case "snapshot":
		m = snapshotTransitions
	default:
		return false
	}
	return len(m[state]) == 0
}

// Package model 定义跨区域数据库模式演进兼容验证服务的领域实体、状态常量与错误类型。
package model

import "errors"

// 各类实体标识。
type (
	SchemaVersionID string
	RegionID        string
	PlanID          string
	StepID          string
	SemanticID      string
	WindowID        string
	ConflictID      string
	SnapshotID      string
)

// FieldType 字段类型。
type FieldType string

const (
	FieldString FieldType = "string"
	FieldInt    FieldType = "int"
	FieldBool   FieldType = "bool"
	FieldFloat  FieldType = "float"
)

// Field 是 schema 版本中的一个字段定义。
type Field struct {
	Name string    `json:"name"`
	Type FieldType `json:"type"`
}

// SchemaVersion 描述某一版本的数据库模式（字段集合 + 内容哈希）。
type SchemaVersion struct {
	ID          SchemaVersionID `json:"id"`
	Tag         string          `json:"tag"`
	ContentHash string          `json:"content_hash"`
	Fields      []Field         `json:"fields"`
	CreatedAt   int64           `json:"created_at"`
}

// RegionReplica 是一个区域副本，记录其当前运行的 schema 版本。
type RegionReplica struct {
	ID              RegionID        `json:"id"`
	Name            string          `json:"name"`
	CurrentVersionID SchemaVersionID `json:"current_version_id"`
	State           string          `json:"state"`
	UpgradedAt      int64           `json:"upgraded_at"`
	CreatedAt       int64           `json:"created_at"`
}

// MigrationPlan 是一次分阶段 schema 演进计划。
type MigrationPlan struct {
	ID               PlanID          `json:"id"`
	Name             string          `json:"name"`
	State            string          `json:"state"`
	BaselineVersionID SchemaVersionID `json:"baseline_version_id"`
	TargetVersionID  SchemaVersionID `json:"target_version_id"`
	SealedAt         int64           `json:"sealed_at"`
	CreatedAt        int64           `json:"created_at"`
}

// StepKind 迁移步骤种类。
type StepKind string

const (
	StepSplit         StepKind = "split"           // 字段拆分：一个字段 -> 多个字段
	StepMerge         StepKind = "merge"           // 字段合并
	StepAdd           StepKind = "add"             // 加列
	StepDrop          StepKind = "drop"            // 删列
	StepRename        StepKind = "rename"          // 改名
	StepTypeChange    StepKind = "type_change"     // 改类型
	StepStopDualWrite StepKind = "stop_dual_write" // 停止双写（关键阶段）
)

// MigrationStep 是演进计划中的一个迁移步骤。
type MigrationStep struct {
	ID            StepID   `json:"id"`
	PlanID        PlanID   `json:"plan_id"`
	Ordinal       int      `json:"ordinal"`
	Kind          StepKind `json:"kind"`
	Description   string   `json:"description"`
	FromField     string   `json:"from_field"`
	ToFieldsJSON  string   `json:"to_fields_json"` // JSON 数组
	DualWrite     bool     `json:"dual_write"`     // 该步骤期间是否双写（旧+新字段）
	StopDualWrite bool     `json:"stop_dual_write"` // 该步骤执行后停止双写
	State         string   `json:"state"`
	CreatedAt     int64     `json:"created_at"`
}

// SemanticKind 语义种类。
type SemanticKind string

const (
	SemanticRead  SemanticKind = "read"
	SemanticWrite SemanticKind = "write"
)

// Semantic 记录某一 schema 版本对应的读/写路径声明。
type Semantic struct {
	ID              SemanticID      `json:"id"`
	SchemaVersionID SchemaVersionID `json:"schema_version_id"`
	Kind            SemanticKind    `json:"kind"`
	Hash            string          `json:"hash"`
	CreatedAt       int64           `json:"created_at"`
}

// WindowRuleType 兼容窗口规则种类。
type WindowRuleType string

const (
	// RuleAdapter 读路径适配器：用写版本字段组合出读版本字段（如 customer_name = fn + " " + ln）。
	RuleAdapter WindowRuleType = "adapter"
	// RuleUpgradeRequired 要求区域在停止双写前完成升级。
	RuleUpgradeRequired WindowRuleType = "upgrade_required"
)

// CompatWindow 声明一个兼容窗口：覆盖「读版本 × 写版本」组合并给出消解规则。
type CompatWindow struct {
	ID              WindowID       `json:"id"`
	PlanID          PlanID         `json:"plan_id"`
	ReaderVersionID SchemaVersionID `json:"reader_version_id"`
	WriterVersionID SchemaVersionID `json:"writer_version_id"`
	RuleType        WindowRuleType `json:"rule_type"`
	RulePayload     string         `json:"rule_payload"` // JSON：adapter 描述或约束
	State           string         `json:"state"`
	ValidFrom       int64          `json:"valid_from"`
	ValidTo         int64          `json:"valid_to"`
	CreatedAt       int64          `json:"created_at"`
}

// ConflictPath 记录一次检测到的不兼容组合。
type ConflictPath struct {
	ID              ConflictID     `json:"id"`
	PlanID          PlanID         `json:"plan_id"`
	RegionID        RegionID       `json:"region_id"`
	ReaderVersionID SchemaVersionID `json:"reader_version_id"`
	WriterVersionID SchemaVersionID `json:"writer_version_id"`
	StepID          StepID         `json:"step_id"`
	Field           string         `json:"field"`    // 无法被读路径解释的字段
	Reason          string         `json:"reason"`
	Severity        string         `json:"severity"` // high | low
	Resolved        bool           `json:"resolved"`
	DetectedAt      int64          `json:"detected_at"`
}

// CompatSnapshot 是不可变兼容快照。
type CompatSnapshot struct {
	ID          SnapshotID `json:"id"`
	PlanID      PlanID     `json:"plan_id"`
	State       string     `json:"state"`
	ContentJSON string     `json:"content_json"`
	Hash        string     `json:"hash"`
	SupersededBy SnapshotID `json:"superseded_by"`
	CreatedAt   int64       `json:"created_at"`
}

// SampleRecord 是语义模拟用的样本记录。
type SampleRecord struct {
	ID              string          `json:"id"`
	SchemaVersionID SchemaVersionID `json:"schema_version_id"`
	PayloadJSON     string          `json:"payload_json"`
	Hash            string          `json:"hash"`
	CreatedAt       int64           `json:"created_at"`
}

// AuditEvent 追加式审计事件。
type AuditEvent struct {
	ID        string `json:"id"`
	Action    string `json:"action"`
	Detail    string `json:"detail"`
	CreatedAt int64  `json:"created_at"`
}

// 统一错误。
var (
	ErrNotFound         = errors.New("not found")
	ErrInvalidState     = errors.New("invalid state transition")
	ErrDuplicate        = errors.New("duplicate entry")
	ErrSealed           = errors.New("entity is sealed and immutable")
	ErrMissingField     = errors.New("missing required field")
	ErrInvalidArgument  = errors.New("invalid argument")
	ErrVersionRegress   = errors.New("schema version regression not allowed")
	ErrMigrationCycle   = errors.New("migration cycle detected")
	ErrDualWriteMissing = errors.New("dual-write target missing")
)

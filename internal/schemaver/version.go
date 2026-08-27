// Package schemaver 维护 schema 版本的登记、内容哈希与读取。
package schemaver

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"

	"task280-regevocompat/internal/model"
	"task280-regevocompat/internal/store"
)

// hashFields 对字段集合（按名称排序）做确定性哈希，保证相同定义得到相同内容哈希。
func hashFields(fields []model.Field) string {
	cp := append([]model.Field{}, fields...)
	sort.Slice(cp, func(i, j int) bool { return cp[i].Name < cp[j].Name })
	b, _ := json.Marshal(cp)
	sum := sha256.Sum256(b)
	return fmt.Sprintf("%x", sum)[:16]
}

// Register 登记一个 schema 版本，自动计算内容哈希并持久化。
func Register(s *store.Store, tag string, fields []model.Field) (*model.SchemaVersion, error) {
	if tag == "" {
		return nil, fmt.Errorf("%w: tag required", model.ErrInvalidArgument)
	}
	if len(fields) == 0 {
		return nil, fmt.Errorf("%w: at least one field required", model.ErrInvalidArgument)
	}
	seen := map[string]bool{}
	for _, f := range fields {
		if f.Name == "" {
			return nil, fmt.Errorf("%w: field name required", model.ErrInvalidArgument)
		}
		if seen[f.Name] {
			return nil, fmt.Errorf("duplicate field %q: %w", f.Name, model.ErrDuplicate)
		}
		seen[f.Name] = true
	}
	v := &model.SchemaVersion{
		ID:          model.SchemaVersionID(model.GenID("sv")),
		Tag:         tag,
		ContentHash: hashFields(fields),
		Fields:      fields,
		CreatedAt:   store.NowMillis(),
	}
	if err := s.SaveSchemaVersion(v); err != nil {
		return nil, err
	}
	return v, nil
}

// Get 按 ID 读取 schema 版本。
func Get(s *store.Store, id model.SchemaVersionID) (*model.SchemaVersion, error) {
	return s.GetSchemaVersion(id)
}

// List 列出全部 schema 版本。
func List(s *store.Store) ([]*model.SchemaVersion, error) {
	return s.ListSchemaVersions()
}

// FieldNames 返回 schema 版本的字段名集合（有序）。
func FieldNames(v *model.SchemaVersion) []string {
	names := make([]string, 0, len(v.Fields))
	for _, f := range v.Fields {
		names = append(names, f.Name)
	}
	sort.Strings(names)
	return names
}

// HasField 判断 schema 版本是否包含某字段。
func HasField(v *model.SchemaVersion, name string) bool {
	for _, f := range v.Fields {
		if f.Name == name {
			return true
		}
	}
	return false
}

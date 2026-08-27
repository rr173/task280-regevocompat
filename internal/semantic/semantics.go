// Package semantic 模拟跨版本读写路径：写路径按版本编码记录，读路径按版本解码记录，
// 并支持通过兼容窗口的读路径适配器重建缺失字段。
package semantic

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"task280-regevocompat/internal/model"
	"task280-regevocompat/internal/store"
)

// Record 是字段到字符串值的映射（本服务所有字段值以字符串表示）。
type Record map[string]string

// Adapter 描述读路径适配器：用 inputs 字段按 op 组合出 output 字段。
type Adapter struct {
	Output string   `json:"output"`
	Inputs []string `json:"inputs"`
	Op     string   `json:"op"` // concat_space | concat | first
}

// Register 登记某 schema 版本的读/写路径语义声明。
func Register(s *store.Store, versionID model.SchemaVersionID, kind model.SemanticKind) (*model.Semantic, error) {
	if versionID == "" {
		return nil, fmt.Errorf("%w: version required", model.ErrInvalidArgument)
	}
	if _, err := s.GetSchemaVersion(versionID); err != nil {
		return nil, err
	}
	if kind != model.SemanticRead && kind != model.SemanticWrite {
		return nil, fmt.Errorf("%w: invalid semantic kind", model.ErrInvalidArgument)
	}
	h := sha256.Sum256([]byte(string(versionID) + "|" + string(kind)))
	se := &model.Semantic{
		ID:              model.SemanticID(model.GenID("sm")),
		SchemaVersionID: versionID,
		Kind:            kind,
		Hash:            hex.EncodeToString(h[:])[:16],
		CreatedAt:       store.NowMillis(),
	}
	if err := s.SaveSemantic(se); err != nil {
		return nil, err
	}
	return se, nil
}

// List 列出全部语义声明。
func List(s *store.Store) ([]*model.Semantic, error) {
	return s.ListSemantics()
}

// Encode 写路径：按写版本字段集合，将逻辑记录编码为存储记录（仅保留写版本拥有的字段）。
func Encode(writer *model.SchemaVersion, logical Record) Record {
	out := Record{}
	for _, f := range writer.Fields {
		if v, ok := logical[f.Name]; ok {
			out[f.Name] = v
		} else {
			out[f.Name] = "" // 写版本拥有该字段，但逻辑记录未提供
		}
	}
	return out
}

// Decode 读路径：按读版本字段集合解释存储记录。
// 若某读版本字段在 stored 中缺失，尝试用 adapters 重建；仍无法得到则计入 missing。
func Decode(reader *model.SchemaVersion, stored Record, adapters []Adapter) (Record, []string) {
	decoded := Record{}
	var missing []string
	for _, f := range reader.Fields {
		if v, ok := stored[f.Name]; ok {
			decoded[f.Name] = v
			continue
		}
		// 尝试适配器重建
		rebuilt, ok := applyAdapters(f.Name, stored, adapters)
		if ok {
			decoded[f.Name] = rebuilt
			continue
		}
		missing = append(missing, f.Name)
	}
	return decoded, missing
}

// applyAdapters 用给定适配器尝试重建 output 字段。
func applyAdapters(output string, stored Record, adapters []Adapter) (string, bool) {
	for _, a := range adapters {
		if a.Output != output {
			continue
		}
		values := make([]string, 0, len(a.Inputs))
		allPresent := true
		for _, in := range a.Inputs {
			v, ok := stored[in]
			if !ok {
				allPresent = false
				break
			}
			values = append(values, v)
		}
		if !allPresent {
			continue
		}
		switch a.Op {
		case "concat_space":
			return strings.Join(values, " "), true
		case "concat":
			return strings.Join(values, ""), true
		case "first":
			if len(values) > 0 {
				return values[0], true
			}
		}
	}
	return "", false
}

// ApplyAdapterByFields 在探索阶段（无具体值）按可用字段集合判定是否能用适配器重建 output：
// 若某适配器的 inputs 全部包含于 availableFields，则返回重建后的占位串（inputs 按 op 组合）与 true。
func ApplyAdapterByFields(output string, availableFields []string, adapters []Adapter) (string, bool) {
	set := map[string]bool{}
	for _, f := range availableFields {
		set[f] = true
	}
	for _, a := range adapters {
		if a.Output != output {
			continue
		}
		allPresent := true
		for _, in := range a.Inputs {
			if !set[in] {
				allPresent = false
				break
			}
		}
		if !allPresent {
			continue
		}
		switch a.Op {
		case "concat_space":
			return strings.Join(a.Inputs, " "), true
		case "concat":
			return strings.Join(a.Inputs, ""), true
		case "first":
			if len(a.Inputs) > 0 {
				return a.Inputs[0], true
			}
		}
	}
	return "", false
}

// ParseAdapters 解析兼容窗口 rule_payload 中的适配器数组。
func ParseAdapters(payload string) ([]Adapter, error) {
	if payload == "" {
		return nil, nil
	}
	var out []Adapter
	if err := json.Unmarshal([]byte(payload), &out); err != nil {
		return nil, fmt.Errorf("parse adapters: %w", err)
	}
	return out, nil
}

// Package httpapi 提供基于标准 net/http 的 JSON API（路由前缀 /api）。
package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"task280-regevocompat/internal/model"
	"task280-regevocompat/internal/service"
)

// Handler 持有业务 Service，注册全部路由。
type Handler struct {
	svc *service.Service
}

// New 构造 Handler。
func New(svc *service.Service) *Handler { return &Handler{svc: svc} }

// Routes 注册全部 HTTP 路由（Go 1.22+ 的 method+path 通配语法）。
func (h *Handler) Routes(mux *http.ServeMux) {
	// schema 版本
	mux.HandleFunc("POST /api/schema-versions", h.handleCreateSchemaVersion)
	mux.HandleFunc("GET /api/schema-versions", h.handleListSchemaVersions)
	mux.HandleFunc("GET /api/schema-versions/{id}", h.handleGetSchemaVersion)
	// 区域
	mux.HandleFunc("POST /api/regions", h.handleCreateRegion)
	mux.HandleFunc("GET /api/regions", h.handleListRegions)
	mux.HandleFunc("GET /api/regions/{id}", h.handleGetRegion)
	mux.HandleFunc("POST /api/regions/{id}/upgrade", h.handleUpgradeRegion)
	mux.HandleFunc("POST /api/regions/{id}/set-version", h.handleSetRegionVersion)
	// 计划
	mux.HandleFunc("POST /api/plans", h.handleCreatePlan)
	mux.HandleFunc("GET /api/plans", h.handleListPlans)
	mux.HandleFunc("GET /api/plans/{id}", h.handleGetPlan)
	mux.HandleFunc("POST /api/plans/{id}/verify", h.handleVerifyPlan)
	mux.HandleFunc("POST /api/plans/{id}/explore", h.handleExplorePlan)
	mux.HandleFunc("POST /api/plans/{id}/seal", h.handleSealPlan)
	mux.HandleFunc("GET /api/plans/{id}/conflicts", h.handleListConflicts)
	// 迁移步骤
	mux.HandleFunc("POST /api/plans/{id}/steps", h.handleAddStep)
	mux.HandleFunc("GET /api/plans/{id}/steps", h.handleListSteps)
	mux.HandleFunc("POST /api/steps/{id}/advance", h.handleAdvanceStep)
	// 语义
	mux.HandleFunc("POST /api/semantics", h.handleCreateSemantic)
	mux.HandleFunc("GET /api/semantics", h.handleListSemantics)
	// 兼容窗口
	mux.HandleFunc("POST /api/plans/{id}/windows", h.handleDeclareWindow)
	mux.HandleFunc("GET /api/plans/{id}/windows", h.handleListWindows)
	mux.HandleFunc("POST /api/windows/{id}/revoke", h.handleRevokeWindow)
	// 快照
	mux.HandleFunc("POST /api/plans/{id}/snapshots", h.handlePublishSnapshot)
	mux.HandleFunc("GET /api/plans/{id}/snapshots", h.handleListSnapshots)
	mux.HandleFunc("POST /api/snapshots/{id}/supersede", h.handleSupersedeSnapshot)
	// 自检
	mux.HandleFunc("GET /api/health", h.handleHealth)
	mux.HandleFunc("POST /api/example", h.handleExample)
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func readJSON(r *http.Request, v interface{}) error {
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	return dec.Decode(v)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func classifyError(err error) (int, string) {
	switch {
	case err == model.ErrNotFound:
		return http.StatusNotFound, "NOT_FOUND"
	case errors.Is(err, model.ErrDuplicate):
		return http.StatusConflict, "CONFLICT"
	case errors.Is(err, model.ErrSealed):
		return http.StatusConflict, "SEALED"
	case errors.Is(err, model.ErrInvalidState):
		return http.StatusConflict, "INVALID_STATE"
	case errors.Is(err, model.ErrInvalidArgument), errors.Is(err, model.ErrMissingField):
		return http.StatusBadRequest, "INVALID_ARGUMENT"
	default:
		return http.StatusInternalServerError, "INTERNAL"
	}
}

func writeAPIError(w http.ResponseWriter, err error) {
	status, code := classifyError(err)
	writeJSON(w, status, map[string]string{"error": code, "message": err.Error()})
}

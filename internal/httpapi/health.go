package httpapi

import (
	"net/http"

	"task280-regevocompat/internal/service"
)

func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"service": "regevocompat",
	})
}

func (h *Handler) handleExample(w http.ResponseWriter, r *http.Request) {
	plan, err := service.SeedExample(h.svc.Store())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	conflicts, err := h.svc.Conflicts(plan.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	snaps, err := h.svc.Store().ListSnapshotsByPlan(plan.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"plan":      plan,
		"conflicts": conflicts,
		"snapshots": snaps,
		"message":   "示例场景已构建：区域 B 的 customer_name 冲突经兼容窗口消解，计划已封存并发布快照",
	})
}

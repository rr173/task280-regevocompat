package httpapi

import (
	"context"
	"net/http"

	"task280-regevocompat/internal/migration"
	"task280-regevocompat/internal/model"
)

type createPlanReq struct {
	Name              string `json:"name"`
	BaselineVersionID string `json:"baseline_version_id"`
	TargetVersionID   string `json:"target_version_id"`
}

func (h *Handler) handleCreatePlan(w http.ResponseWriter, r *http.Request) {
	var req createPlanReq
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	p, err := migration.CreatePlan(h.svc.Store(), req.Name, model.SchemaVersionID(req.BaselineVersionID), model.SchemaVersionID(req.TargetVersionID))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

func (h *Handler) handleListPlans(w http.ResponseWriter, r *http.Request) {
	ps, err := migration.List(h.svc.Store())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, ps)
}

func (h *Handler) handleGetPlan(w http.ResponseWriter, r *http.Request) {
	id := model.PlanID(r.PathValue("id"))
	p, err := h.svc.GetPlan(id)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (h *Handler) handleVerifyPlan(w http.ResponseWriter, r *http.Request) {
	id := model.PlanID(r.PathValue("id"))
	p, err := migration.Verify(h.svc.Store(), id)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (h *Handler) handleExplorePlan(w http.ResponseWriter, r *http.Request) {
	id := model.PlanID(r.PathValue("id"))
	conflicts, err := h.svc.VerifyAndExplore(context.Background(), id)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"plan_id":   string(id),
		"conflicts": conflicts,
		"count":     len(conflicts),
	})
}

func (h *Handler) handleSealPlan(w http.ResponseWriter, r *http.Request) {
	id := model.PlanID(r.PathValue("id"))
	p, err := h.svc.Seal(r.Context(), id)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

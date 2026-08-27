package httpapi

import (
	"net/http"

	"task280-regevocompat/internal/migration"
	"task280-regevocompat/internal/model"
)

type addStepReq struct {
	Ordinal        int      `json:"ordinal"`
	Kind           string   `json:"kind"`
	FromField      string   `json:"from_field"`
	ToFields       []string `json:"to_fields"`
	DualWrite      bool     `json:"dual_write"`
	StopDualWrite  bool     `json:"stop_dual_write"`
	Description    string   `json:"description"`
}

func (h *Handler) handleAddStep(w http.ResponseWriter, r *http.Request) {
	planID := model.PlanID(r.PathValue("id"))
	var req addStepReq
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	st, err := migration.AddStep(h.svc.Store(), planID, req.Ordinal, model.StepKind(req.Kind),
		req.FromField, req.ToFields, req.DualWrite, req.StopDualWrite, req.Description)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, st)
}

func (h *Handler) handleListSteps(w http.ResponseWriter, r *http.Request) {
	planID := model.PlanID(r.PathValue("id"))
	sts, err := migration.ListSteps(h.svc.Store(), planID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, sts)
}

type advanceStepReq struct {
	To string `json:"to"`
}

func (h *Handler) handleAdvanceStep(w http.ResponseWriter, r *http.Request) {
	id := model.StepID(r.PathValue("id"))
	var req advanceStepReq
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if req.To == "" {
		req.To = model.StepDualWrite
	}
	st, err := migration.Advance(h.svc.Store(), id, req.To)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

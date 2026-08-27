package httpapi

import (
	"net/http"

	"task280-regevocompat/internal/compat"
	"task280-regevocompat/internal/model"
)

func (h *Handler) handlePublishSnapshot(w http.ResponseWriter, r *http.Request) {
	planID := model.PlanID(r.PathValue("id"))
	snap, err := h.svc.Publish(r.Context(), planID)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, snap)
}

func (h *Handler) handleListSnapshots(w http.ResponseWriter, r *http.Request) {
	planID := model.PlanID(r.PathValue("id"))
	snaps, err := h.svc.ListSnapshots(planID)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, snaps)
}

type supersedeSnapshotReq struct {
	By string `json:"by"`
}

func (h *Handler) handleSupersedeSnapshot(w http.ResponseWriter, r *http.Request) {
	id := model.SnapshotID(r.PathValue("id"))
	var req supersedeSnapshotReq
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if req.By == "" {
		writeError(w, http.StatusBadRequest, model.ErrMissingField)
		return
	}
	snap, err := compat.Supersede(h.svc.Store(), id, model.SnapshotID(req.By))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, snap)
}

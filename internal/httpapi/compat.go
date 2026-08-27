package httpapi

import (
	"net/http"

	"task280-regevocompat/internal/compat"
	"task280-regevocompat/internal/model"
)

type declareWindowReq struct {
	ReaderVersionID string `json:"reader_version_id"`
	WriterVersionID string `json:"writer_version_id"`
	RuleType        string `json:"rule_type"`
	RulePayload     string `json:"rule_payload"`
}

func (h *Handler) handleDeclareWindow(w http.ResponseWriter, r *http.Request) {
	planID := model.PlanID(r.PathValue("id"))
	var req declareWindowReq
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	wnd, err := h.svc.DeclareWindow(planID,
		model.SchemaVersionID(req.ReaderVersionID), model.SchemaVersionID(req.WriterVersionID),
		model.WindowRuleType(req.RuleType), req.RulePayload)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, wnd)
}

func (h *Handler) handleListWindows(w http.ResponseWriter, r *http.Request) {
	planID := model.PlanID(r.PathValue("id"))
	wnds, err := compat.ListWindows(h.svc.Store(), planID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, wnds)
}

func (h *Handler) handleRevokeWindow(w http.ResponseWriter, r *http.Request) {
	id := model.WindowID(r.PathValue("id"))
	wnd, err := h.svc.RevokeWindow(id)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, wnd)
}

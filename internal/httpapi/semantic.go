package httpapi

import (
	"errors"
	"net/http"

	"task280-regevocompat/internal/model"
	"task280-regevocompat/internal/semantic"
)

type createSemanticReq struct {
	SchemaVersionID string `json:"schema_version_id"`
	Kind            string `json:"kind"`
}

func (h *Handler) handleCreateSemantic(w http.ResponseWriter, r *http.Request) {
	var req createSemanticReq
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	se, err := semantic.Register(h.svc.Store(), model.SchemaVersionID(req.SchemaVersionID), model.SemanticKind(req.Kind))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, se)
}

func (h *Handler) handleListSemantics(w http.ResponseWriter, r *http.Request) {
	ses, err := semantic.List(h.svc.Store())
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, model.ErrNotFound) {
			status = http.StatusNotFound
		}
		writeError(w, status, err)
		return
	}
	writeJSON(w, http.StatusOK, ses)
}

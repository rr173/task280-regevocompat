package httpapi

import (
	"errors"
	"net/http"

	"task280-regevocompat/internal/model"
	"task280-regevocompat/internal/schemaver"
)

type createSchemaReq struct {
	Tag    string        `json:"tag"`
	Fields []model.Field `json:"fields"`
}

func (h *Handler) handleCreateSchemaVersion(w http.ResponseWriter, r *http.Request) {
	var req createSchemaReq
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	v, err := h.svc.RegisterSchema(req.Tag, req.Fields)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, v)
}

func (h *Handler) handleListSchemaVersions(w http.ResponseWriter, r *http.Request) {
	vs, err := schemaver.List(h.svc.Store())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, vs)
}

func (h *Handler) handleGetSchemaVersion(w http.ResponseWriter, r *http.Request) {
	id := model.SchemaVersionID(r.PathValue("id"))
	v, err := schemaver.Get(h.svc.Store(), id)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, model.ErrNotFound) {
			status = http.StatusNotFound
		}
		writeError(w, status, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

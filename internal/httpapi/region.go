package httpapi

import (
	"errors"
	"net/http"

	"task280-regevocompat/internal/model"
	"task280-regevocompat/internal/region"
)

type createRegionReq struct {
	Name    string `json:"name"`
	Version string `json:"version_id"`
}

func (h *Handler) handleCreateRegion(w http.ResponseWriter, r *http.Request) {
	var req createRegionReq
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	rg, err := region.Register(h.svc.Store(), req.Name, model.SchemaVersionID(req.Version))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, rg)
}

func (h *Handler) handleListRegions(w http.ResponseWriter, r *http.Request) {
	rgs, err := region.List(h.svc.Store())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, rgs)
}

func (h *Handler) handleGetRegion(w http.ResponseWriter, r *http.Request) {
	id := model.RegionID(r.PathValue("id"))
	rg, err := region.Get(h.svc.Store(), id)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, model.ErrNotFound) {
			status = http.StatusNotFound
		}
		writeError(w, status, err)
		return
	}
	writeJSON(w, http.StatusOK, rg)
}

type upgradeRegionReq struct {
	Version string `json:"version_id"`
}

func (h *Handler) handleUpgradeRegion(w http.ResponseWriter, r *http.Request) {
	id := model.RegionID(r.PathValue("id"))
	var req upgradeRegionReq
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	rg, err := region.Upgrade(h.svc.Store(), id, model.SchemaVersionID(req.Version))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, rg)
}

type setVersionReq struct {
	Version string `json:"version_id"`
}

func (h *Handler) handleSetRegionVersion(w http.ResponseWriter, r *http.Request) {
	id := model.RegionID(r.PathValue("id"))
	var req setVersionReq
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	rg, err := region.SetVersion(h.svc.Store(), id, model.SchemaVersionID(req.Version))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, rg)
}

package httpapi

import (
	"errors"
	"net/http"

	"task280-regevocompat/internal/model"
)

func (h *Handler) handleListConflicts(w http.ResponseWriter, r *http.Request) {
	planID := model.PlanID(r.PathValue("id"))
	conflicts, err := h.svc.Conflicts(planID)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, model.ErrNotFound) {
			status = http.StatusNotFound
		}
		writeError(w, status, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"plan_id":   string(planID),
		"conflicts": conflicts,
		"count":     len(conflicts),
	})
}

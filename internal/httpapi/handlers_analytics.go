package httpapi

import "net/http"

func (h *Handler) TeamsSummary(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.currentUser(w, r)
	if !ok {
		return
	}

	result, err := h.analytics.TeamsSummary(r.Context(), userID)
	if err != nil {
		h.respondError(w, r, err)
		return
	}

	respondJSON(w, http.StatusOK, result)
}

func (h *Handler) TopCreators(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.currentUser(w, r)
	if !ok {
		return
	}

	result, err := h.analytics.TopCreators(r.Context(), userID)
	if err != nil {
		h.respondError(w, r, err)
		return
	}

	respondJSON(w, http.StatusOK, result)
}

func (h *Handler) InvalidAssignees(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.currentUser(w, r)
	if !ok {
		return
	}

	result, err := h.analytics.InvalidAssignees(r.Context(), userID)
	if err != nil {
		h.respondError(w, r, err)
		return
	}

	respondJSON(w, http.StatusOK, result)
}

package httpapi

import (
	"net/http"
)

type registerRequest struct {
	Email    string `json:"email"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginResponse struct {
	Token string `json:"token"`
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := decodeJSON(w, r, &req); err != nil {
		h.respondError(w, r, err)
		return
	}

	user, err := h.auth.Register(r.Context(), req.Email, req.Username, req.Password)
	if err != nil {
		h.respondError(w, r, err)
		return
	}

	respondJSON(w, http.StatusCreated, user)
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := decodeJSON(w, r, &req); err != nil {
		h.respondError(w, r, err)
		return
	}

	token, err := h.auth.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		h.respondError(w, r, err)
		return
	}

	respondJSON(w, http.StatusOK, loginResponse{Token: token})
}

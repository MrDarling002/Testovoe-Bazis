package httpapi

import (
	"net/http"

	"github.com/example/Testovoe-Bazis/internal/domain"
	"github.com/example/Testovoe-Bazis/internal/service"
)

type createTeamRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type inviteRequest struct {
	UserID int64  `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"`
}

type inviteResponse struct {
	TeamMember       domain.TeamMember `json:"team_member"`
	NotificationSent bool              `json:"notification_sent"`
}

func (h *Handler) CreateTeam(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.currentUser(w, r)
	if !ok {
		return
	}

	var req createTeamRequest
	if err := decodeJSON(w, r, &req); err != nil {
		h.respondError(w, r, err)
		return
	}

	team, err := h.teams.CreateTeam(r.Context(), userID, req.Name, req.Description)
	if err != nil {
		h.respondError(w, r, err)
		return
	}

	respondJSON(w, http.StatusCreated, team)
}

func (h *Handler) ListTeams(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.currentUser(w, r)
	if !ok {
		return
	}

	teams, err := h.teams.ListTeams(r.Context(), userID)
	if err != nil {
		h.respondError(w, r, err)
		return
	}

	respondJSON(w, http.StatusOK, teams)
}

func (h *Handler) InviteToTeam(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.currentUser(w, r)
	if !ok {
		return
	}

	teamID, err := pathID(r)
	if err != nil {
		h.respondError(w, r, err)
		return
	}

	var req inviteRequest
	if err := decodeJSON(w, r, &req); err != nil {
		h.respondError(w, r, err)
		return
	}

	result, err := h.teams.Invite(r.Context(), userID, teamID, service.InviteInput{
		UserID: req.UserID,
		Email:  req.Email,
		Role:   domain.Role(req.Role),
	})
	if err != nil {
		h.respondError(w, r, err)
		return
	}

	respondJSON(w, http.StatusCreated, inviteResponse{
		TeamMember:       result.TeamMember,
		NotificationSent: result.NotificationSent,
	})
}

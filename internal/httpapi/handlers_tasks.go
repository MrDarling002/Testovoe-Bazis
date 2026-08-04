package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/example/Testovoe-Bazis/internal/domain"
	"github.com/example/Testovoe-Bazis/internal/service"
)

type createTaskRequest struct {
	TeamID      int64  `json:"team_id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      string `json:"status"`
	AssigneeID  *int64 `json:"assignee_id"`
}

type updateTaskRequest struct {
	Title       *string         `json:"title"`
	Description *string         `json:"description"`
	Status      *string         `json:"status"`
	AssigneeID  json.RawMessage `json:"assignee_id"`
}

var jsonNull = []byte("null")

func (req updateTaskRequest) toPatch() (domain.TaskPatch, error) {
	patch := domain.TaskPatch{
		Title:       req.Title,
		Description: req.Description,
	}

	if req.Status != nil {
		status := domain.TaskStatus(*req.Status)
		patch.Status = &status
	}

	if len(req.AssigneeID) > 0 {
		patch.Assignee.Set = true

		if !bytes.Equal(bytes.TrimSpace(req.AssigneeID), jsonNull) {
			var v int64
			if err := json.Unmarshal(req.AssigneeID, &v); err != nil {
				return domain.TaskPatch{}, fmt.Errorf(
					"%w: assignee_id must be an integer or null", domain.ErrValidation,
				)
			}

			patch.Assignee.Value = &v
		}
	}

	return patch, nil
}

func (h *Handler) CreateTask(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.currentUser(w, r)
	if !ok {
		return
	}

	var req createTaskRequest
	if err := decodeJSON(w, r, &req); err != nil {
		h.respondError(w, r, err)
		return
	}

	task, err := h.tasks.Create(r.Context(), userID, service.CreateTaskInput{
		TeamID:      req.TeamID,
		Title:       req.Title,
		Description: req.Description,
		Status:      domain.TaskStatus(req.Status),
		AssigneeID:  req.AssigneeID,
	})
	if err != nil {
		h.respondError(w, r, err)
		return
	}

	respondJSON(w, http.StatusCreated, task)
}

func (h *Handler) ListTasks(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.currentUser(w, r)
	if !ok {
		return
	}

	q := r.URL.Query()

	teamID, err := queryInt64(q, "team_id")
	if err != nil {
		h.respondError(w, r, err)
		return
	}

	var assigneeID *int64

	if q.Get("assignee_id") != "" {
		parsed, err := queryInt64(q, "assignee_id")
		if err != nil {
			h.respondError(w, r, err)
			return
		}

		assigneeID = &parsed
	}

	page, err := queryInt(q, "page")
	if err != nil {
		h.respondError(w, r, err)
		return
	}

	perPage, err := queryInt(q, "per_page")
	if err != nil {
		h.respondError(w, r, err)
		return
	}

	result, err := h.tasks.List(r.Context(), userID, domain.TaskFilters{
		TeamID:     teamID,
		Status:     q.Get("status"),
		AssigneeID: assigneeID,
		Page:       page,
		PerPage:    perPage,
	})
	if err != nil {
		h.respondError(w, r, err)
		return
	}

	respondJSON(w, http.StatusOK, result)
}

func (h *Handler) UpdateTask(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.currentUser(w, r)
	if !ok {
		return
	}

	taskID, err := pathID(r)
	if err != nil {
		h.respondError(w, r, err)
		return
	}

	var req updateTaskRequest
	if err := decodeJSON(w, r, &req); err != nil {
		h.respondError(w, r, err)
		return
	}

	patch, err := req.toPatch()
	if err != nil {
		h.respondError(w, r, err)
		return
	}

	task, err := h.tasks.Update(r.Context(), userID, taskID, patch)
	if err != nil {
		h.respondError(w, r, err)
		return
	}

	respondJSON(w, http.StatusOK, task)
}

func (h *Handler) TaskHistory(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.currentUser(w, r)
	if !ok {
		return
	}

	taskID, err := pathID(r)
	if err != nil {
		h.respondError(w, r, err)
		return
	}

	q := r.URL.Query()

	page, err := queryInt(q, "page")
	if err != nil {
		h.respondError(w, r, err)
		return
	}

	perPage, err := queryInt(q, "per_page")
	if err != nil {
		h.respondError(w, r, err)
		return
	}

	history, err := h.tasks.History(r.Context(), userID, taskID, page, perPage)
	if err != nil {
		h.respondError(w, r, err)
		return
	}

	respondJSON(w, http.StatusOK, history)
}

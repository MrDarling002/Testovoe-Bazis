package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/example/Testovoe-Bazis/internal/domain"
)

type TeamGateway interface {
	GetMemberRole(ctx context.Context, teamID int64, userID int64) (domain.Role, error)
	IsMember(ctx context.Context, teamID int64, userID int64) (bool, error)
}

type TaskGateway interface {
	Get(ctx context.Context, id int64) (domain.Task, error)
	List(ctx context.Context, filters domain.TaskFilters) ([]domain.Task, int64, error)
	Create(ctx context.Context, task domain.Task) (domain.Task, error)
	Update(
		ctx context.Context,
		id int64,
		patch domain.TaskPatch,
		actorID int64,
		authorize func(task domain.Task) error,
	) (domain.Task, error)
	History(ctx context.Context, taskID int64, limit, offset int) ([]domain.TaskHistory, error)
}

type TaskCacheGateway interface {
	GetList(ctx context.Context, teamID int64, filters domain.TaskFilters) ([]byte, bool)
	SetList(ctx context.Context, teamID int64, filters domain.TaskFilters, data []byte)
	InvalidateTeam(ctx context.Context, teamID int64)
}

type CreateTaskInput struct {
	TeamID      int64
	Title       string
	Description string
	Status      domain.TaskStatus
	AssigneeID  *int64
}

type TaskService struct {
	tasks TaskGateway
	teams TeamGateway
	cache TaskCacheGateway
}

func NewTaskService(tasks TaskGateway, teams TeamGateway, cache TaskCacheGateway) *TaskService {
	return &TaskService{
		tasks: tasks,
		teams: teams,
		cache: cache,
	}
}

// memberRole resolves the actor's role in the team, translating "not a
// member" into ErrForbidden while passing infrastructure errors through
// unchanged so they surface as 500s, not 403s.
func (s *TaskService) memberRole(ctx context.Context, teamID, userID int64) (domain.Role, error) {
	role, err := s.teams.GetMemberRole(ctx, teamID, userID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return "", domain.ErrForbidden
		}

		return "", fmt.Errorf("get member role: %w", err)
	}

	return role, nil
}

func (s *TaskService) Create(ctx context.Context, actorID int64, in CreateTaskInput) (domain.Task, error) {
	in.Title = strings.TrimSpace(in.Title)

	if in.Title == "" {
		return domain.Task{}, fmt.Errorf("%w: title is required", domain.ErrValidation)
	}

	if len([]rune(in.Title)) > domain.MaxTaskTitleLength {
		return domain.Task{}, fmt.Errorf(
			"%w: title must be at most %d characters",
			domain.ErrValidation, domain.MaxTaskTitleLength,
		)
	}

	if in.Status == "" {
		in.Status = domain.TaskStatusTodo
	} else if !in.Status.Valid() {
		return domain.Task{}, fmt.Errorf("%w: unknown status %q", domain.ErrValidation, in.Status)
	}

	if _, err := s.memberRole(ctx, in.TeamID, actorID); err != nil {
		return domain.Task{}, err
	}

	if in.AssigneeID != nil {
		ok, err := s.teams.IsMember(ctx, in.TeamID, *in.AssigneeID)
		if err != nil {
			return domain.Task{}, fmt.Errorf("check assignee membership: %w", err)
		}

		if !ok {
			return domain.Task{}, fmt.Errorf("%w: assignee is not a member of the team", domain.ErrValidation)
		}
	}

	task := domain.Task{
		TeamID:      in.TeamID,
		Title:       in.Title,
		Description: in.Description,
		Status:      in.Status,
		AssigneeID:  in.AssigneeID,
		CreatedBy:   actorID,
	}

	created, err := s.tasks.Create(ctx, task)
	if err != nil {
		return domain.Task{}, err
	}

	s.cache.InvalidateTeam(ctx, created.TeamID)

	return created, nil
}

func (s *TaskService) List(ctx context.Context, actorID int64, filters domain.TaskFilters) (domain.TaskPage, error) {
	filters.Normalize()

	if err := filters.Validate(); err != nil {
		return domain.TaskPage{}, err
	}

	if _, err := s.memberRole(ctx, filters.TeamID, actorID); err != nil {
		return domain.TaskPage{}, err
	}

	if cached, ok := s.cache.GetList(ctx, filters.TeamID, filters); ok {
		var page domain.TaskPage
		if err := json.Unmarshal(cached, &page); err == nil {
			return page, nil
		}
	}

	tasks, total, err := s.tasks.List(ctx, filters)
	if err != nil {
		return domain.TaskPage{}, err
	}

	page := domain.TaskPage{
		Items:   tasks,
		Total:   total,
		Page:    filters.Page,
		PerPage: filters.PerPage,
		HasNext: int64(filters.Page*filters.PerPage) < total,
	}

	if data, err := json.Marshal(page); err == nil {
		s.cache.SetList(ctx, filters.TeamID, filters, data)
	}

	return page, nil
}

func (s *TaskService) Update(ctx context.Context, actorID int64, taskID int64, patch domain.TaskPatch) (domain.Task, error) {
	if patch.Empty() {
		return domain.Task{}, fmt.Errorf("%w: at least one field must be provided", domain.ErrValidation)
	}

	if patch.Status != nil && !patch.Status.Valid() {
		return domain.Task{}, fmt.Errorf("%w: unknown status %q", domain.ErrValidation, *patch.Status)
	}

	if patch.Title != nil {
		title := strings.TrimSpace(*patch.Title)

		if title == "" {
			return domain.Task{}, fmt.Errorf("%w: title cannot be empty", domain.ErrValidation)
		}

		if len([]rune(title)) > domain.MaxTaskTitleLength {
			return domain.Task{}, fmt.Errorf(
				"%w: title must be at most %d characters",
				domain.ErrValidation, domain.MaxTaskTitleLength,
			)
		}

		patch.Title = &title
	}

	task, err := s.tasks.Get(ctx, taskID)
	if err != nil {
		return domain.Task{}, err
	}

	role, err := s.memberRole(ctx, task.TeamID, actorID)
	if err != nil {
		return domain.Task{}, err
	}

	if patch.Assignee.Set && patch.Assignee.Value != nil {
		ok, err := s.teams.IsMember(ctx, task.TeamID, *patch.Assignee.Value)
		if err != nil {
			return domain.Task{}, fmt.Errorf("check assignee membership: %w", err)
		}

		if !ok {
			return domain.Task{}, fmt.Errorf("%w: assignee is not a member of the team", domain.ErrValidation)
		}
	}

	updated, err := s.tasks.Update(ctx, taskID, patch, actorID, func(t domain.Task) error {
		info := AccessInfo{
			TeamRole:   role,
			IsCreator:  t.CreatedBy == actorID,
			IsAssignee: t.AssigneeID != nil && *t.AssigneeID == actorID,
		}

		return CanUpdateTask(patch, info)
	})
	if err != nil {
		return domain.Task{}, err
	}

	s.cache.InvalidateTeam(ctx, updated.TeamID)

	return updated, nil
}

func (s *TaskService) History(ctx context.Context, actorID int64, taskID int64, page, perPage int) ([]domain.TaskHistory, error) {
	if page < 1 {
		page = 1
	}

	if perPage <= 0 {
		perPage = domain.DefaultPageSize
	}

	if perPage > domain.MaxPageSize {
		perPage = domain.MaxPageSize
	}

	task, err := s.tasks.Get(ctx, taskID)
	if err != nil {
		return nil, err
	}

	if _, err := s.memberRole(ctx, task.TeamID, actorID); err != nil {
		return nil, err
	}

	return s.tasks.History(ctx, taskID, perPage, (page-1)*perPage)
}

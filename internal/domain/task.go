package domain

import (
	"fmt"
	"time"
)

type TaskStatus string

const (
	TaskStatusTodo       TaskStatus = "todo"
	TaskStatusInProgress TaskStatus = "in_progress"
	TaskStatusDone       TaskStatus = "done"
)

func (s TaskStatus) Valid() bool {
	switch s {
	case TaskStatusTodo, TaskStatusInProgress, TaskStatusDone:
		return true
	default:
		return false
	}
}

const (
	MaxTaskTitleLength = 200
	DefaultPageSize    = 20
	MaxPageSize        = 100
)

type Task struct {
	ID          int64      `db:"id" json:"id"`
	TeamID      int64      `db:"team_id" json:"team_id"`
	Title       string     `db:"title" json:"title"`
	Description string     `db:"description" json:"description,omitempty"`
	Status      TaskStatus `db:"status" json:"status"`
	AssigneeID  *int64     `db:"assignee_id" json:"assignee_id"`
	CreatedBy   int64      `db:"created_by" json:"created_by"`
	CreatedAt   time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time  `db:"updated_at" json:"updated_at"`
	CompletedAt *time.Time `db:"completed_at" json:"completed_at"`
}

type TaskHistory struct {
	ID        int64     `db:"id" json:"id"`
	TaskID    int64     `db:"task_id" json:"task_id"`
	ChangedBy int64     `db:"changed_by" json:"changed_by"`
	Action    string    `db:"action" json:"action"`
	Field     *string   `db:"field" json:"field,omitempty"`
	OldValue  *string   `db:"old_value" json:"old_value,omitempty"`
	NewValue  *string   `db:"new_value" json:"new_value,omitempty"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

type TaskFilters struct {
	TeamID     int64
	Status     string
	AssigneeID *int64
	Page       int
	PerPage    int
}

func (f *TaskFilters) Normalize() {
	if f.Page < 1 {
		f.Page = 1
	}

	if f.PerPage <= 0 {
		f.PerPage = DefaultPageSize
	}

	if f.PerPage > MaxPageSize {
		f.PerPage = MaxPageSize
	}
}

func (f TaskFilters) Validate() error {
	if f.TeamID <= 0 {
		return fmt.Errorf("%w: team_id is required", ErrValidation)
	}

	if f.Status != "" && !TaskStatus(f.Status).Valid() {
		return fmt.Errorf("%w: unknown status %q", ErrValidation, f.Status)
	}

	return nil
}

type OptionalInt64 struct {
	Set   bool
	Value *int64
}

type TaskPatch struct {
	Title       *string
	Description *string
	Status      *TaskStatus
	Assignee    OptionalInt64
}

func (p TaskPatch) Empty() bool {
	return p.Title == nil && p.Description == nil && p.Status == nil && !p.Assignee.Set
}

type TaskPage struct {
	Items   []Task `json:"items"`
	Total   int64  `json:"total"`
	Page    int    `json:"page"`
	PerPage int    `json:"per_page"`
	HasNext bool   `json:"has_next"`
}

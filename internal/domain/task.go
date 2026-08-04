package domain

import "time"

type TaskStatus string

const (
	TaskStatusTodo TaskStatus = "todo"
	TaskStatusInProgress TaskStatus = "in_progress"
	TaskStatusDone TaskStatus = "done"
)

func (s TaskStatus) Valid() bool {
	switch s {
	case TaskStatusTodo, TaskStatusInProgress, TaskStatusDone:
		return true
	default:
		return false
	}
}

type Task struct {
	ID int64 `db:"id" json:"id"`
	TeamID int64 `db:"team_id" json:"team_id"`
	Title string `db:"title" json:"title"`
	Description string `db:"description" json:"description,omitempty"`
	Status TaskStatus `db:"status" json:"status"`
	AssigneeID *int64 `db:"assignee_id" json:"assignee_id"`
	CreatedBy int64 `db:"created_by" json:"created_by"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
	CompletedAt *time.Time `db:"completed_at" json:"completed_at"`
}

type TaskHistory struct {
	ID int64 `db:"id" json:"id"`
	TaskID int64 `db:"task_id" json:"task_id"`
	ChangedBy int64 `db:"changed_by" json:"changed_by"`
	Action string `db:"action" json:"action"`
	Field string `db:"field" json:"field,omitempty"`
	OldValue string `db:"old_value" json:"old_value,omitempty"`
	NewValue string `db:"new_value" json:"new_value,omitempty"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

type TaskFilters struct {
	TeamID int64
	Status string
	AssigneeID *int64
	Page int
	PerPage int
}

func (f *TaskFilters) Normalize() {
	if f.Page < 1 {
		f.Page = 1
	}

	if f.PerPage <= 0 {
		f.PerPage = 20
	}

	if f.PerPage > 100 {
		f.PerPage = 100
	}
}

type TaskPatch struct {
	Title *string `json:"title,omitempty"`
	Description *string `json:"description,omitempty"`
	Status *TaskStatus `json:"status,omitempty"`
	AssigneeID *int64 `json:"assignee_id,omitempty"` // 0 = unassign
}

type TaskPage struct {
	Items []Task `json:"items"`
	Total int64 `json:"total"`
	Page int `json:"page"`
	PerPage int `json:"per_page"`
	HasNext bool `json:"has_next"`
}
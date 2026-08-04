package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"

	"github.com/example/Testovoe-Bazis/internal/domain"
)

type TaskRepository struct {
	db *sqlx.DB
}

func NewTaskRepository(db *sqlx.DB) *TaskRepository {
	return &TaskRepository{db: db}
}

const taskColumns = `
	id,
	team_id,
	title,
	description,
	status,
	assignee_id,
	created_by,
	created_at,
	updated_at,
	completed_at
`

func (r *TaskRepository) Get(ctx context.Context, id int64) (domain.Task, error) {
	var task domain.Task

	err := r.db.GetContext(ctx, &task, `
		SELECT `+taskColumns+`
		FROM tasks
		WHERE id = ?
	`, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Task{}, domain.ErrNotFound
		}

		return domain.Task{}, fmt.Errorf("select task: %w", err)
	}

	return task, nil
}

func (r *TaskRepository) List(ctx context.Context, filters domain.TaskFilters) ([]domain.Task, int64, error) {
	filters.Normalize()

	where := []string{"team_id = ?"}
	args := []any{filters.TeamID}

	if filters.Status != "" {
		where = append(where, "status = ?")
		args = append(args, filters.Status)
	}

	if filters.AssigneeID != nil {
		where = append(where, "assignee_id = ?")
		args = append(args, *filters.AssigneeID)
	}

	whereSQL := strings.Join(where, " AND ")

	var total int64

	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM tasks WHERE %s`, whereSQL)
	if err := r.db.GetContext(ctx, &total, countQuery, args...); err != nil {
		return nil, 0, fmt.Errorf("count tasks: %w", err)
	}

	listQuery := fmt.Sprintf(`
		SELECT `+taskColumns+`
		FROM tasks
		WHERE %s
		ORDER BY id DESC
		LIMIT ? OFFSET ?
	`, whereSQL)

	listArgs := make([]any, 0, len(args)+2)
	listArgs = append(listArgs, args...)
	listArgs = append(listArgs, filters.PerPage, (filters.Page-1)*filters.PerPage)

	tasks := make([]domain.Task, 0, filters.PerPage)

	if err := r.db.SelectContext(ctx, &tasks, listQuery, listArgs...); err != nil {
		return nil, 0, fmt.Errorf("select tasks: %w", err)
	}

	return tasks, total, nil
}

func (r *TaskRepository) Create(ctx context.Context, task domain.Task) (domain.Task, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return domain.Task{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	now := time.Now().UTC().Truncate(time.Second)

	task.CreatedAt = now
	task.UpdatedAt = now

	if task.Status == domain.TaskStatusDone {
		task.CompletedAt = &now
	} else {
		task.CompletedAt = nil
	}

	res, err := tx.ExecContext(ctx, `
		INSERT INTO tasks (
			team_id,
			title,
			description,
			status,
			assignee_id,
			created_by,
			created_at,
			updated_at,
			completed_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		task.TeamID,
		task.Title,
		task.Description,
		task.Status,
		task.AssigneeID,
		task.CreatedBy,
		task.CreatedAt,
		task.UpdatedAt,
		task.CompletedAt,
	)
	if err != nil {
		return domain.Task{}, fmt.Errorf("insert task: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return domain.Task{}, fmt.Errorf("last insert id: %w", err)
	}

	task.ID = id

	_, err = tx.ExecContext(ctx, `
		INSERT INTO task_history (
			task_id,
			changed_by,
			action,
			field,
			old_value,
			new_value,
			created_at
		) VALUES (?, ?, 'created', NULL, NULL, NULL, ?)
	`, task.ID, task.CreatedBy, now)
	if err != nil {
		return domain.Task{}, fmt.Errorf("insert task history: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return domain.Task{}, fmt.Errorf("commit tx: %w", err)
	}

	return task, nil
}

func (r *TaskRepository) Update(
	ctx context.Context,
	id int64,
	patch domain.TaskPatch,
	actorID int64,
	authorize func(task domain.Task) error,
) (domain.Task, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return domain.Task{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	var task domain.Task

	err = tx.GetContext(ctx, &task, `
		SELECT `+taskColumns+`
		FROM tasks
		WHERE id = ?
		FOR UPDATE
	`, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Task{}, domain.ErrNotFound
		}

		return domain.Task{}, fmt.Errorf("select task for update: %w", err)
	}

	if err := authorize(task); err != nil {
		return domain.Task{}, err
	}

	now := time.Now().UTC().Truncate(time.Second)

	history := buildHistory(&task, patch, actorID, now)

	if len(history) == 0 {
		return task, nil
	}

	task.UpdatedAt = now

	_, err = tx.ExecContext(ctx, `
		UPDATE tasks
		SET
			title = ?,
			description = ?,
			status = ?,
			assignee_id = ?,
			completed_at = ?,
			updated_at = ?
		WHERE id = ?
	`,
		task.Title,
		task.Description,
		task.Status,
		task.AssigneeID,
		task.CompletedAt,
		task.UpdatedAt,
		task.ID,
	)
	if err != nil {
		return domain.Task{}, fmt.Errorf("update task: %w", err)
	}

	for _, h := range history {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO task_history (
				task_id,
				changed_by,
				action,
				field,
				old_value,
				new_value,
				created_at
			) VALUES (?, ?, ?, ?, ?, ?, ?)
		`,
			h.TaskID,
			h.ChangedBy,
			h.Action,
			h.Field,
			h.OldValue,
			h.NewValue,
			now,
		)
		if err != nil {
			return domain.Task{}, fmt.Errorf("insert task history: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return domain.Task{}, fmt.Errorf("commit tx: %w", err)
	}

	return task, nil
}

func buildHistory(task *domain.Task, patch domain.TaskPatch, actorID int64, now time.Time) []domain.TaskHistory {
	var history []domain.TaskHistory

	record := func(field string, oldValue, newValue *string) {
		history = append(history, domain.TaskHistory{
			TaskID:    task.ID,
			ChangedBy: actorID,
			Action:    "updated",
			Field:     strPtr(field),
			OldValue:  oldValue,
			NewValue:  newValue,
		})
	}

	if patch.Title != nil && *patch.Title != task.Title {
		record("title", strPtr(task.Title), strPtr(*patch.Title))
		task.Title = *patch.Title
	}

	if patch.Description != nil && *patch.Description != task.Description {
		record("description", strPtr(truncate(task.Description, 500)), strPtr(truncate(*patch.Description, 500)))
		task.Description = *patch.Description
	}

	if patch.Status != nil && *patch.Status != task.Status {
		record("status", strPtr(string(task.Status)), strPtr(string(*patch.Status)))

		task.Status = *patch.Status

		if task.Status == domain.TaskStatusDone {
			task.CompletedAt = &now
		} else {
			task.CompletedAt = nil
		}
	}

	if patch.Assignee.Set {
		oldValue := int64Ptr(task.AssigneeID)

		if patch.Assignee.Value == nil {
			task.AssigneeID = nil
		} else {
			v := *patch.Assignee.Value
			task.AssigneeID = &v
		}

		newValue := int64Ptr(task.AssigneeID)

		if !equalStrPtr(oldValue, newValue) {
			record("assignee_id", oldValue, newValue)
		}
	}

	return history
}

func (r *TaskRepository) History(ctx context.Context, taskID int64, limit, offset int) ([]domain.TaskHistory, error) {
	history := make([]domain.TaskHistory, 0, limit)

	err := r.db.SelectContext(ctx, &history, `
		SELECT
			id,
			task_id,
			changed_by,
			action,
			field,
			old_value,
			new_value,
			created_at
		FROM task_history
		WHERE task_id = ?
		ORDER BY id DESC
		LIMIT ? OFFSET ?
	`, taskID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("select task history: %w", err)
	}

	return history, nil
}

func strPtr(s string) *string { return &s }

func int64Ptr(v *int64) *string {
	if v == nil {
		return nil
	}

	s := strconv.FormatInt(*v, 10)

	return &s
}

func equalStrPtr(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}

	return *a == *b
}

func truncate(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}

	return string(runes[:max])
}

func IsMySQLDuplicate(err error) bool {
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) {
		return mysqlErr.Number == 1062
	}

	return false
}

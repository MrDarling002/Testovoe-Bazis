package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/example/Testovoe-Bazis/internal/domain"
	"github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

type TaskRepository struct {
	db *sqlx.DB
}

func NewTaskRepository(db *sqlx.DB) *TaskRepository {
	return &TaskRepository{db: db}
}

func (r *TaskRepository) Get(ctx context.Context, id int64) (domain.Task, error) {
	var task domain.Task

	query := `
		SELECT
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
		FROM tasks
		WHERE id = ?
	`

	err := r.db.GetContext(ctx, &task, query, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Task{}, domain.ErrNotFound
		}
		return domain.Task{}, err
	}

	return task, nil
}

func (r *TaskRepository) List(ctx context.Context, filters domain.TaskFilters) ([]domain.Task, int64, error) {
	filters.Normalize()

	where := []string{"team_id = ?"}
	args := []interface{}{filters.TeamID}

	if filters.Status != "" {
		where = append(where, "status = ?")
		args = append(args, filters.Status)
	}

	if filters.AssigneeID != nil {
		where = append(where, "assignee_id = ?")
		args = append(args, *filters.AssigneeID)
	}

	whereSQL := strings.Join(where, " AND ")

	countQuery := fmt.Sprintf(`
		SELECT COUNT(*)
		FROM tasks
		WHERE %s
	`, whereSQL)

	var total int64

	if err := r.db.GetContext(ctx, &total, countQuery, args...); err != nil {
		return nil, 0, err
	}

	listQuery := fmt.Sprintf(`
		SELECT
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
		FROM tasks
		WHERE %s
		ORDER BY id DESC
		LIMIT ? OFFSET ?
	`, whereSQL)

	listArgs := make([]interface{}, 0, len(args)+2)
	listArgs = append(listArgs, args...)
	listArgs = append(listArgs, filters.PerPage, (filters.Page-1)*filters.PerPage)

	var tasks []domain.Task

	if err := r.db.SelectContext(ctx, &tasks, listQuery, listArgs...); err != nil {
		return nil, 0, err
	}

	return tasks, total, nil
}

func (r *TaskRepository) Create(ctx context.Context, task domain.Task) (domain.Task, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return domain.Task{}, err
	}
	defer tx.Rollback()

	now := time.Now().UTC()

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
		return domain.Task{}, err
	}

	id, err := res.LastInsertId()
	if err != nil {
		return domain.Task{}, err
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
		return domain.Task{}, err
	}

	if err := tx.Commit(); err != nil {
		return domain.Task{}, err
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
		return domain.Task{}, err
	}
	defer tx.Rollback()

	var task domain.Task

	err = tx.GetContext(ctx, &task, `
		SELECT
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
		FROM tasks
		WHERE id = ?
		FOR UPDATE
	`, id)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Task{}, domain.ErrNotFound
		}
		return domain.Task{}, err
	}

	if err := authorize(task); err != nil {
		return domain.Task{}, err
	}

	now := time.Now().UTC()
	var history []domain.TaskHistory

	if patch.Title != nil && *patch.Title != task.Title {
		history = append(history, domain.TaskHistory{
			TaskID: task.ID,
			ChangedBy: actorID,
			Action: "updated",
			Field: "title",
			OldValue: task.Title,
			NewValue: *patch.Title,
		})
		task.Title = *patch.Title
	}

	if patch.Description != nil && *patch.Description != task.Description {
		oldDesc := truncate(task.Description, 500)
		newDesc := truncate(*patch.Description, 500)

		history = append(history, domain.TaskHistory{
			TaskID: task.ID,
			ChangedBy: actorID,
			Action: "updated",
			Field:"description",
			OldValue: oldDesc,
			NewValue: newDesc,
		})
		task.Description = *patch.Description
	}

	if patch.Status != nil && *patch.Status != task.Status {
		oldStatus := string(task.Status)
		newStatus := string(*patch.Status)

		history = append(history, domain.TaskHistory{
			TaskID: task.ID,
			ChangedBy: actorID,
			Action: "updated",
			Field: "status",
			OldValue: oldStatus,
			NewValue: newStatus,
		})

		task.Status = *patch.Status

		if task.Status == domain.TaskStatusDone {
			task.CompletedAt = &now
		} else {
			task.CompletedAt = nil
		}
	}

	if patch.AssigneeID != nil {
		oldAssignee := "null"
		if task.AssigneeID != nil {
			oldAssignee = strconv.FormatInt(*task.AssigneeID, 10)
		}

		if *patch.AssigneeID == 0 {
			task.AssigneeID = nil
		} else {
			v := *patch.AssigneeID
			task.AssigneeID = &v
		}

		newAssignee := "null"
		if task.AssigneeID != nil {
			newAssignee = strconv.FormatInt(*task.AssigneeID, 10)
		}

		if oldAssignee != newAssignee {
			history = append(history, domain.TaskHistory{
				TaskID: task.ID,
				ChangedBy: actorID,
				Action: "updated",
				Field: "assignee_id",
				OldValue: oldAssignee,
				NewValue: newAssignee,
			})
		}
	}

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
		return domain.Task{}, err
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
			return domain.Task{}, err
		}
	}

	if err := tx.Commit(); err != nil {
		return domain.Task{}, err
	}

	return task, nil
}

func (r *TaskRepository) History(ctx context.Context, taskID int64) ([]domain.TaskHistory, error) {
	var history []domain.TaskHistory

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
		LIMIT 100
	`, taskID)

	if err != nil {
		return nil, err
	}

	return history, nil
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
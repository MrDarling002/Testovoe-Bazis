package mysql

import (
	"context"

	"github.com/jmoiron/sqlx"
)

type TeamSummary struct {
	TeamID int64 `db:"team_id" json:"team_id"`
	TeamName string `db:"team_name" json:"team_name"`
	MembersCount int64 `db:"members_count" json:"members_count"`
	DoneLast7Days int64 `db:"done_last_7_days" json:"done_last_7_days"`
}

type TopCreator struct {
	TeamID int64 `db:"team_id" json:"team_id"`
	TeamName string `db:"team_name" json:"team_name"`
	UserID int64 `db:"user_id" json:"user_id"`
	Username string `db:"username" json:"username"`
	TasksCreated int64 `db:"tasks_created" json:"tasks_created"`
	Rank int64 `db:"rank" json:"rank"`
}

type InvalidAssigneeTask struct {
	TaskID int64 `db:"task_id" json:"task_id"`
	TaskTitle  string `db:"task_title" json:"task_title"`
	TeamID int64 `db:"team_id" json:"team_id"`
	AssigneeID int64 `db:"assignee_id" json:"assignee_id"`
}

type AnalyticsRepository struct {
	db *sqlx.DB
}

func NewAnalyticsRepository(db *sqlx.DB) *AnalyticsRepository {
	return &AnalyticsRepository{db: db}
}

func (r *AnalyticsRepository) TeamSummary(ctx context.Context) ([]TeamSummary, error) {
	var result []TeamSummary
	query := `
		SELECT
			t.id AS team_id,
			t.name AS team_name,
			COUNT(DISTINCT tm.user_id) AS members_count,
			COUNT(DISTINCT done.id) AS done_last_7_days
		FROM teams t
		LEFT JOIN team_members tm ON tm.team_id = t.id
		LEFT JOIN tasks done ON done.team_id = t.id
			AND done.status = 'done'
			AND done.completed_at >= UTC_TIMESTAMP() - INTERVAL 7 DAY
		GROUP BY t.id, t.name
		ORDER BY t.id
	`

	err := r.db.SelectContext(ctx, &result, query)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (r *AnalyticsRepository) TopCreators(ctx context.Context) ([]TopCreator, error) {
	var result []TopCreator

	query := `
		WITH created_stats AS (
			SELECT
				t.team_id,
				t.created_by AS user_id,
				COUNT(*) AS tasks_created
			FROM tasks t
			WHERE t.created_at >= UTC_TIMESTAMP() - INTERVAL 30 DAY
			GROUP BY t.team_id, t.created_by
		),
		ranked AS (
			SELECT
				team_id,
				user_id,
				tasks_created,
				ROW_NUMBER() OVER (
					PARTITION BY team_id
					ORDER BY tasks_created DESC, user_id ASC
				) AS rn
			FROM created_stats
		)
		SELECT
			r.team_id,
			teams.name AS team_name,
			r.user_id,
			users.username,
			r.tasks_created,
			r.rn AS rank
		FROM ranked r
		JOIN teams ON teams.id = r.team_id
		JOIN users ON users.id = r.user_id
		WHERE r.rn <= 3
		ORDER BY r.team_id, r.rn
	`

	err := r.db.SelectContext(ctx, &result, query)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (r *AnalyticsRepository) InvalidAssignees(ctx context.Context) ([]InvalidAssigneeTask, error) {
	var result []InvalidAssigneeTask

	query := `
		SELECT
			t.id AS task_id,
			t.title AS task_title,
			t.team_id,
			t.assignee_id
		FROM tasks t
		LEFT JOIN team_members tm ON tm.team_id = t.team_id AND tm.user_id = t.assignee_id
		WHERE t.assignee_id IS NOT NULL
			AND tm.user_id IS NULL
		ORDER BY t.id
	`

	err := r.db.SelectContext(ctx, &result, query)
	if err != nil {
		return nil, err
	}

	return result, nil
}
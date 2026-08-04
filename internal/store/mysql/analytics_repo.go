package mysql

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"

	"github.com/example/Testovoe-Bazis/internal/domain"
)

// AnalyticsRepository runs the reporting queries. Every query is scoped to the
// teams the requesting user belongs to, so no cross-team data leaks out.
type AnalyticsRepository struct {
	db *sqlx.DB
}

func NewAnalyticsRepository(db *sqlx.DB) *AnalyticsRepository {
	return &AnalyticsRepository{db: db}
}

// TeamSummary returns, for each team the user belongs to, the member count and
// the number of tasks completed in the last 7 days. Counts are computed in
// derived tables to avoid a members x tasks cartesian join.
func (r *AnalyticsRepository) TeamSummary(ctx context.Context, userID int64) ([]domain.TeamSummary, error) {
	result := make([]domain.TeamSummary, 0)

	query := `
		SELECT
			t.id AS team_id,
			t.name AS team_name,
			COALESCE(m.members_count, 0) AS members_count,
			COALESCE(d.done_count, 0) AS done_last_7_days
		FROM teams t
		JOIN team_members me ON me.team_id = t.id AND me.user_id = ?
		LEFT JOIN (
			SELECT team_id, COUNT(*) AS members_count
			FROM team_members
			GROUP BY team_id
		) m ON m.team_id = t.id
		LEFT JOIN (
			SELECT team_id, COUNT(*) AS done_count
			FROM tasks
			WHERE status = 'done'
				AND completed_at >= UTC_TIMESTAMP() - INTERVAL 7 DAY
			GROUP BY team_id
		) d ON d.team_id = t.id
		ORDER BY t.id
	`

	if err := r.db.SelectContext(ctx, &result, query, userID); err != nil {
		return nil, fmt.Errorf("select team summary: %w", err)
	}

	return result, nil
}

// TopCreators returns the top-3 task creators for the last 30 days in each
// team the user belongs to, ranked with a window function.
func (r *AnalyticsRepository) TopCreators(ctx context.Context, userID int64) ([]domain.TopCreator, error) {
	result := make([]domain.TopCreator, 0)

	query := `
		WITH created_stats AS (
			SELECT
				t.team_id,
				t.created_by AS user_id,
				COUNT(*) AS tasks_created
			FROM tasks t
			JOIN team_members me ON me.team_id = t.team_id AND me.user_id = ?
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
			r.rn AS rank_position
		FROM ranked r
		JOIN teams ON teams.id = r.team_id
		JOIN users ON users.id = r.user_id
		WHERE r.rn <= 3
		ORDER BY r.team_id, r.rn
	`

	if err := r.db.SelectContext(ctx, &result, query, userID); err != nil {
		return nil, fmt.Errorf("select top creators: %w", err)
	}

	return result, nil
}

// InvalidAssignees finds tasks (within the user's teams) whose assignee is not
// a member of the task's team — a data-integrity validation query.
func (r *AnalyticsRepository) InvalidAssignees(ctx context.Context, userID int64) ([]domain.InvalidAssigneeTask, error) {
	result := make([]domain.InvalidAssigneeTask, 0)

	query := `
		SELECT
			t.id AS task_id,
			t.title AS task_title,
			t.team_id,
			t.assignee_id
		FROM tasks t
		JOIN team_members me ON me.team_id = t.team_id AND me.user_id = ?
		LEFT JOIN team_members tm ON tm.team_id = t.team_id AND tm.user_id = t.assignee_id
		WHERE t.assignee_id IS NOT NULL
			AND tm.user_id IS NULL
		ORDER BY t.id
	`

	if err := r.db.SelectContext(ctx, &result, query, userID); err != nil {
		return nil, fmt.Errorf("select invalid assignees: %w", err)
	}

	return result, nil
}

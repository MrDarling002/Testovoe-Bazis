package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"

	"github.com/example/Testovoe-Bazis/internal/domain"
)

type TeamRepository struct {
	db *sqlx.DB
}

func NewTeamRepository(db *sqlx.DB) *TeamRepository {
	return &TeamRepository{db: db}
}

func (r *TeamRepository) Create(ctx context.Context, name string, description string, creatorID int64) (domain.Team, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return domain.Team{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // rollback after commit is a no-op

	res, err := tx.ExecContext(ctx, `
		INSERT INTO teams (name, description, created_by)
		VALUES (?, ?, ?)
	`, name, description, creatorID)
	if err != nil {
		return domain.Team{}, fmt.Errorf("insert team: %w", err)
	}

	teamID, err := res.LastInsertId()
	if err != nil {
		return domain.Team{}, fmt.Errorf("last insert id: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO team_members (team_id, user_id, role)
		VALUES (?, ?, 'owner')
	`, teamID, creatorID)
	if err != nil {
		return domain.Team{}, fmt.Errorf("insert owner membership: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return domain.Team{}, fmt.Errorf("commit tx: %w", err)
	}

	return r.Get(ctx, teamID)
}

func (r *TeamRepository) Get(ctx context.Context, id int64) (domain.Team, error) {
	var team domain.Team

	err := r.db.GetContext(ctx, &team, `
		SELECT id, name, description, created_by, created_at, updated_at
		FROM teams
		WHERE id = ?
	`, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Team{}, domain.ErrNotFound
		}

		return domain.Team{}, fmt.Errorf("select team: %w", err)
	}

	return team, nil
}

func (r *TeamRepository) ListByUser(ctx context.Context, userID int64) ([]domain.Team, error) {
	teams := make([]domain.Team, 0)

	err := r.db.SelectContext(ctx, &teams, `
		SELECT
			t.id,
			t.name,
			t.description,
			t.created_by,
			t.created_at,
			t.updated_at
		FROM teams t
		JOIN team_members tm ON tm.team_id = t.id
		WHERE tm.user_id = ?
		ORDER BY t.id DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("select teams by user: %w", err)
	}

	return teams, nil
}

// GetMemberRole returns domain.ErrNotFound when the user is not a member of
// the team; interpreting that as "forbidden" is a service-layer decision.
func (r *TeamRepository) GetMemberRole(ctx context.Context, teamID int64, userID int64) (domain.Role, error) {
	var role domain.Role

	err := r.db.GetContext(ctx, &role, `
		SELECT role
		FROM team_members
		WHERE team_id = ? AND user_id = ?
	`, teamID, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", domain.ErrNotFound
		}

		return "", fmt.Errorf("select member role: %w", err)
	}

	return role, nil
}

func (r *TeamRepository) IsMember(ctx context.Context, teamID int64, userID int64) (bool, error) {
	var exists bool

	err := r.db.GetContext(ctx, &exists, `
		SELECT EXISTS(
			SELECT 1
			FROM team_members
			WHERE team_id = ? AND user_id = ?
		)
	`, teamID, userID)
	if err != nil {
		return false, fmt.Errorf("check membership: %w", err)
	}

	return exists, nil
}

func (r *TeamRepository) AddMember(ctx context.Context, teamID int64, userID int64, invitedBy int64, role domain.Role) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO team_members (team_id, user_id, role, invited_by)
		VALUES (?, ?, ?, ?)
	`, teamID, userID, role, invitedBy)
	if err != nil {
		if IsMySQLDuplicate(err) {
			return fmt.Errorf("%w: user is already a member of the team", domain.ErrConflict)
		}

		return fmt.Errorf("insert team member: %w", err)
	}

	return nil
}

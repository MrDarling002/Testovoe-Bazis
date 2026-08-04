package mysql

import (
	"context"
	"database/sql"
	"errors"

	"github.com/example/Testovoe-Bazis/internal/domain"
	"github.com/jmoiron/sqlx"
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
		return domain.Team{}, err
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx, `
		INSERT INTO teams (name, description, created_by)
		VALUES (?, ?, ?)
	`, name, description, creatorID)

	if err != nil {
		return domain.Team{}, err
	}

	teamID, err := res.LastInsertId()
	if err != nil {
		return domain.Team{}, err
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO team_members (team_id, user_id, role)
		VALUES (?, ?, 'owner')
	`, teamID, creatorID)

	if err != nil {
		return domain.Team{}, err
	}

	if err := tx.Commit(); err != nil {
		return domain.Team{}, err
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
		return domain.Team{}, err
	}

	return team, nil
}

func (r *TeamRepository) ListByUser(ctx context.Context, userID int64) ([]domain.Team, error) {
	var teams []domain.Team
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
		return nil, err
	}

	return teams, nil
}

func (r *TeamRepository) GetMemberRole(ctx context.Context, teamID int64, userID int64) (domain.Role, error) {
	var role domain.Role

	err := r.db.GetContext(ctx, &role, `
		SELECT role
		FROM team_members
		WHERE team_id = ? AND user_id = ?
	`, teamID, userID)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", domain.ErrForbidden
		}
		return "", err
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
		return false, err
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
			return domain.ErrConflict
		}
		return err
	}

	return nil
}

package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"

	"github.com/example/Testovoe-Bazis/internal/domain"
)

type UserRepository struct {
	db *sqlx.DB
}

func NewUserRepository(db *sqlx.DB) *UserRepository {
	return &UserRepository{db: db}
}

const userColumns = `id, email, username, password_hash, created_at`

func (r *UserRepository) Create(ctx context.Context, email, username, passwordHash string) (domain.User, error) {
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO users (email, username, password_hash)
		VALUES (?, ?, ?)
	`, email, username, passwordHash)
	if err != nil {
		if IsMySQLDuplicate(err) {
			return domain.User{}, fmt.Errorf("%w: email or username is already taken", domain.ErrConflict)
		}

		return domain.User{}, fmt.Errorf("insert user: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return domain.User{}, fmt.Errorf("last insert id: %w", err)
	}

	return r.GetByID(ctx, id)
}

func (r *UserRepository) GetByID(ctx context.Context, id int64) (domain.User, error) {
	var user domain.User

	err := r.db.GetContext(ctx, &user, `
		SELECT `+userColumns+`
		FROM users
		WHERE id = ?
	`, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.User{}, domain.ErrNotFound
		}

		return domain.User{}, fmt.Errorf("select user by id: %w", err)
	}

	return user, nil
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (domain.User, error) {
	var user domain.User

	err := r.db.GetContext(ctx, &user, `
		SELECT `+userColumns+`
		FROM users
		WHERE email = ?
	`, email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.User{}, domain.ErrNotFound
		}

		return domain.User{}, fmt.Errorf("select user by email: %w", err)
	}

	return user, nil
}

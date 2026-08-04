package mysql

import (
	"context"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

type DBConfig struct {
	DSN             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

const (
	pingAttempts = 10
	pingBackoff  = 2 * time.Second
)

// NewDB opens a MySQL connection pool and verifies connectivity with a
// bounded retry loop, so the service tolerates the database still starting up.
func NewDB(ctx context.Context, cfg DBConfig) (*sqlx.DB, error) {
	db, err := sqlx.Open("mysql", cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("open mysql: %w", err)
	}

	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	var pingErr error

	for attempt := 0; attempt < pingAttempts; attempt++ {
		pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		pingErr = db.PingContext(pingCtx)
		cancel()

		if pingErr == nil {
			return db, nil
		}

		select {
		case <-ctx.Done():
			db.Close()
			return nil, fmt.Errorf("ping mysql: %w", ctx.Err())
		case <-time.After(pingBackoff):
		}
	}

	db.Close()

	return nil, fmt.Errorf("ping mysql after %d attempts: %w", pingAttempts, pingErr)
}

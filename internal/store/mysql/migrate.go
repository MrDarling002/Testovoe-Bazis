package mysql

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	migratemysql "github.com/golang-migrate/migrate/v4/database/mysql"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	"github.com/example/Testovoe-Bazis/migrations"
)

func Migrate(dsn string) error {
	db, err := sql.Open("mysql", withMultiStatements(dsn))
	if err != nil {
		return fmt.Errorf("open mysql for migrations: %w", err)
	}
	defer db.Close()

	driver, err := migratemysql.WithInstance(db, &migratemysql.Config{})
	if err != nil {
		return fmt.Errorf("init migrate driver: %w", err)
	}

	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return fmt.Errorf("init migrations source: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", src, "mysql", driver)
	if err != nil {
		return fmt.Errorf("init migrate: %w", err)
	}

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("apply migrations: %w", err)
	}

	return nil
}

func withMultiStatements(dsn string) string {
	if strings.Contains(dsn, "?") {
		return dsn + "&multiStatements=true"
	}

	return dsn + "?multiStatements=true"
}

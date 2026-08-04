package mysql

import (
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

type DBConfig struct {
	DSN string
	MaxOpenConns int
	MaxIdleConns int
	ConnMaxLifetime time.Duration
}

func NewDB(cfg struct {
	DSN string
	MaxOpenConns int
	MaxIdleConns int
	ConnMaxLifetime time.Duration
}) *sqlx.DB {
	db := sqlx.MustConnect("mysql", cfg.DSN)
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	return db
}
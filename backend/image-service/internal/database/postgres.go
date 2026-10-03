package database

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func NewPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	const op = "database.NewPool"

	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to parse dsn: %w", op, err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to connect to postgres: %w", op, err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()

		return nil, fmt.Errorf("%s: failed to ping postgres: %w", op, err)
	}

	return pool, nil
}

func NewDB(ctx context.Context, dsn string) (*sql.DB, error) {
	const op = "database.NewDB"

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to connect to postgres: %w", op, err)
	}

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()

		return nil, fmt.Errorf("%s: failed to ping postgres: %w", op, err)
	}

	return db, nil
}

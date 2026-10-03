package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/andrey1af/shop-api/backend/auth-service/internal/config"
	"github.com/andrey1af/shop-api/backend/auth-service/internal/database"
	"github.com/pressly/goose/v3"
)

func main() {
	log := slog.Default()

	if err := run(log); err != nil {
		log.Error("migration failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	const op = "migrator.run"

	cfg := config.MustLoadMigrator()

	ctx := context.Background()

	db, err := database.NewDB(ctx, cfg.Database.URL)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	defer func() { _ = db.Close() }()

	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("%s: failed to set goose dialect: %w", op, err)
	}

	if err := goose.UpContext(ctx, db, cfg.MigrationsDir); err != nil {
		return fmt.Errorf("%s: failed to apply migrations: %w", op, err)
	}

	log.Info("migrations applied successfully")

	return nil
}

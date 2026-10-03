package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/pressly/goose/v3"

	"github.com/andrey1af/shop-api/backend/api-service/internal/config"
	"github.com/andrey1af/shop-api/backend/api-service/internal/database"
)

func main() {
	logger := slog.Default()
	if err := run(logger); err != nil {
		logger.Error("migration failed", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.LoadMigratorConfig()
	if err != nil {
		return err
	}

	ctx := context.Background()

	db, err := database.NewDB(ctx, cfg.Database.URL)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("failed to set goose dialect: %w", err)
	}

	if err := goose.UpContext(ctx, db, cfg.MigrationsDir); err != nil {
		return fmt.Errorf("failed to apply migrations: %w", err)
	}

	logger.Info("migrations applied successfully")
	return nil
}

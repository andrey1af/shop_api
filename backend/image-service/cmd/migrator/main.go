package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/andrey1af/shop-api/backend/image-service/internal/config"
	"github.com/andrey1af/shop-api/backend/image-service/internal/database"
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

	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("%s: failed to set goose dialect: %w", op, err)
	}

	for i, shardURL := range cfg.Shards.URLs() {
		shard := i + 1

		if err := migrateShard(ctx, shardURL, cfg.MigrationsDir); err != nil {
			return fmt.Errorf("%s: shard %d: %w", op, shard, err)
		}

		log.Info("migrations applied successfully", slog.Int("shard", shard))
	}

	return nil
}

func migrateShard(ctx context.Context, shardURL, migrationsDir string) error {
	db, err := database.NewDB(ctx, shardURL)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	if err := goose.UpContext(ctx, db, migrationsDir); err != nil {
		return fmt.Errorf("failed to apply migrations: %w", err)
	}

	return nil
}

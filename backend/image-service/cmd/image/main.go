package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/andrey1af/shop-api/backend/image-service/internal/app"
	"github.com/andrey1af/shop-api/backend/image-service/internal/config"
	"github.com/andrey1af/shop-api/backend/image-service/internal/database"
	"github.com/andrey1af/shop-api/backend/image-service/internal/database/sqlc"
	"github.com/andrey1af/shop-api/backend/image-service/internal/repository"
)

const (
	envLocal = "local"
	envDev   = "dev"
	envProd  = "prod"
)

func main() {
	cfg := config.MustLoad()

	log := setupLogger(cfg.Env)

	log.Info("starting image service", slog.String("env", cfg.Env))

	if err := run(log, cfg); err != nil {
		log.Error("failed to run image service", slog.String("error", err.Error()))
		os.Exit(1)
	}

	log.Info("image service stopped")
}

func run(log *slog.Logger, cfg *config.Config) error {
	shardURLs := cfg.Shards.URLs()
	shards := make([]sqlc.DBTX, 0, len(shardURLs))

	for i, shardURL := range shardURLs {
		pool, err := database.NewPool(context.Background(), shardURL)
		if err != nil {
			return fmt.Errorf("connect to shard %d: %w", i+1, err)
		}
		defer pool.Close()

		shards = append(shards, pool)
	}

	imageRepository, err := repository.NewImageRepository(shards)
	if err != nil {
		return fmt.Errorf("create image repository: %w", err)
	}

	application := app.NewApp(log, cfg.GRPC, imageRepository)

	go application.MustRun()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	sign := <-stop

	log.Info("stopping image service", slog.String("signal", sign.String()))

	application.Stop()

	return nil
}

func setupLogger(env string) *slog.Logger {
	var log *slog.Logger

	switch env {
	case envLocal:
		log = slog.New(
			slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}),
		)
	case envDev:
		log = slog.New(
			slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}),
		)
	case envProd:
		log = slog.New(
			slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}),
		)
	}

	return log
}

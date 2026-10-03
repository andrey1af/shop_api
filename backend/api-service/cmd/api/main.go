package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/andrey1af/shop-api/backend/api-service/internal/app"
	authgrpc "github.com/andrey1af/shop-api/backend/api-service/internal/clients/auth/grpc"
	imagegrpc "github.com/andrey1af/shop-api/backend/api-service/internal/clients/image/grpc"
	"github.com/andrey1af/shop-api/backend/api-service/internal/config"
	"github.com/andrey1af/shop-api/backend/api-service/internal/database"
	"github.com/andrey1af/shop-api/backend/api-service/internal/repository"
	"github.com/andrey1af/shop-api/backend/api-service/internal/storage"
)

const (
	envLocal = "local"
	envDev   = "dev"
	envProd  = "prod"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		slog.Default().Error("failed to load config", slog.String("error", err.Error()))
		os.Exit(1)
	}

	log := setupLogger(cfg.Env)

	log.Info("starting api service", slog.String("env", cfg.Env))

	if err := run(log, cfg); err != nil {
		log.Error("failed to run api service", slog.String("error", err.Error()))
		os.Exit(1)
	}

	log.Info("api service stopped")
}

func run(log *slog.Logger, cfg *config.Config) error {
	pool, err := database.NewPool(context.Background(), cfg.Database.URL)
	if err != nil {
		return fmt.Errorf("connect to postgres: %w", err)
	}
	defer pool.Close()

	authClient, err := authgrpc.New(cfg.Auth.Addr, cfg.Auth.Timeout)
	if err != nil {
		return fmt.Errorf("create auth client: %w", err)
	}
	defer func() {
		if err := authClient.Close(); err != nil {
			log.Error("failed to close auth client", slog.String("error", err.Error()))
		}
	}()

	imageClient, err := imagegrpc.New(cfg.Image.Addr, cfg.Image.Timeout, cfg.Image.MaxMessageBytes)
	if err != nil {
		return fmt.Errorf("create image client: %w", err)
	}
	defer func() {
		if err := imageClient.Close(); err != nil {
			log.Error("failed to close image client", slog.String("error", err.Error()))
		}
	}()

	var imageCache repository.ImageCache
	if cfg.Redis.Addr != "" {
		redisClient := storage.NewRedisClient(cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB)
		defer func() {
			if err := redisClient.Close(); err != nil {
				log.Error("failed to close redis client", slog.String("error", err.Error()))
			}
		}()

		if err := redisClient.Ping(context.Background()).Err(); err != nil {
			log.Warn("redis is unavailable, images will be read from image service until it recovers",
				slog.String("error", err.Error()),
			)
		}

		imageCache = storage.NewImageCache(log, redisClient, cfg.Redis.ImageTTL, cfg.Redis.ImageMaxBytes)
		log.Info("image cache enabled", slog.String("addr", cfg.Redis.Addr), slog.Duration("ttl", cfg.Redis.ImageTTL))
	} else {
		log.Info("image cache disabled: REDIS_ADDR is empty")
	}

	clientRepository := repository.NewClientRepository(pool)
	supplierRepository := repository.NewSupplierRepository(pool)
	productRepository := repository.NewProductRepository(pool)
	imageRepository := repository.NewImageRepository(pool, imageClient, imageCache)

	application := app.NewApp(
		log,
		cfg.HTTP,
		cfg.Kafka,
		authClient,
		clientRepository,
		supplierRepository,
		productRepository,
		imageRepository,
		pool,
		imageClient,
	)

	go application.MustRun()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	sign := <-stop

	log.Info("stopping api service", slog.String("signal", sign.String()))

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

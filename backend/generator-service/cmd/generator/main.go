package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/andrey1af/shop-api/backend/generator-service/internal/app"
	"github.com/andrey1af/shop-api/backend/generator-service/internal/broker/kafka"
	apiclient "github.com/andrey1af/shop-api/backend/generator-service/internal/clients/api/http"
	"github.com/andrey1af/shop-api/backend/generator-service/internal/config"
	"github.com/andrey1af/shop-api/backend/generator-service/internal/usecase"
)

const (
	envLocal = "local"
	envDev   = "dev"
	envProd  = "prod"
)

func main() {
	cfg := config.MustLoad()

	log := setupLogger(cfg.Env)

	log.Info("starting generator service", slog.String("env", cfg.Env))

	apiClient, err := apiclient.New(cfg.API.BaseURL, cfg.API.Timeout, apiclient.Credentials{
		Email:       cfg.API.Email,
		Password:    cfg.API.Password,
		PhoneNumber: cfg.API.PhoneNumber,
	})
	if err != nil {
		log.Error("failed to create api client", slog.String("error", err.Error()))
		os.Exit(1)
	}

	producer := kafka.NewProducer(cfg.Kafka.Brokers, cfg.Kafka.Topic, cfg.Kafka.WriteTimeout)
	defer func() {
		if err := producer.Close(); err != nil {
			log.Error("failed to close kafka producer", slog.String("error", err.Error()))
		}
	}()

	generatorUseCase := usecase.NewGeneratorUseCase(log, apiClient, producer, usecase.GeneratorSettings{
		PriceDeltaPercent: cfg.Generator.PriceDeltaPercent,
		StockMax:          cfg.Generator.StockMax,
	})

	application := app.NewApp(log, generatorUseCase, cfg.Generator.Interval, cfg.Generator.RefreshInterval)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	application.Run(ctx)

	log.Info("generator service stopped")
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

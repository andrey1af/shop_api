package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/andrey1af/shop-api/backend/auth-service/internal/app"
	"github.com/andrey1af/shop-api/backend/auth-service/internal/config"
	"github.com/andrey1af/shop-api/backend/auth-service/internal/database"
	"github.com/andrey1af/shop-api/backend/auth-service/internal/jwt"
	"github.com/andrey1af/shop-api/backend/auth-service/internal/oauth/yandex"
	"github.com/andrey1af/shop-api/backend/auth-service/internal/repository"
	"github.com/andrey1af/shop-api/backend/auth-service/internal/usecase"
)

const (
	envLocal = "local"
	envDev   = "dev"
	envProd  = "prod"
)

func main() {
	cfg := config.MustLoad()

	log := setupLogger(cfg.Env)

	log.Info("starting auth service", slog.String("env", cfg.Env))

	pool, err := database.NewPool(context.Background(), cfg.Database.URL)
	if err != nil {
		log.Error("failed to connect to postgres", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer pool.Close()

	tokenManager := jwt.NewManager(cfg.JWT.Secret, cfg.JWT.TokenTTL)

	userRepository := repository.NewUserRepository(pool)
	oauthStateRepository := repository.NewOAuthStateRepository(pool)

	oauthProviders := setupOAuthProviders(log, cfg.OAuth)

	sessionRepository := repository.NewSessionRepository(pool)

	application := app.NewApp(
		log,
		cfg.GRPC.Port,
		tokenManager,
		userRepository,
		sessionRepository,
		oauthStateRepository,
		oauthProviders,
		cfg.OAuth.StateTTL,
		cfg.Session,
	)

	go application.MustRun()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	sign := <-stop

	log.Info("stopping auth service", slog.String("signal", sign.String()))

	application.Stop()

	log.Info("auth service stopped")
}

func setupOAuthProviders(log *slog.Logger, cfg config.OAuthConfig) map[string]usecase.OAuthProvider {
	providers := make(map[string]usecase.OAuthProvider)

	if cfg.Yandex.Enabled() {
		providers[yandex.Name] = yandex.New(
			cfg.Yandex.ClientID,
			cfg.Yandex.ClientSecret,
			cfg.Yandex.RedirectURL,
			cfg.HTTPTimeout,
		)
	}

	for name := range providers {
		log.Info("oauth provider enabled", slog.String("provider", name))
	}

	return providers
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

package app

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/andrey1af/shop-api/backend/auth-service/internal/config"
	"github.com/andrey1af/shop-api/backend/auth-service/internal/jwt"
	"github.com/andrey1af/shop-api/backend/auth-service/internal/repository"
	authgrpc "github.com/andrey1af/shop-api/backend/auth-service/internal/transport/grpc"
	"github.com/andrey1af/shop-api/backend/auth-service/internal/transport/grpc/interceptor"
	"github.com/andrey1af/shop-api/backend/auth-service/internal/usecase"
	"google.golang.org/grpc"
)

type App struct {
	log        *slog.Logger
	gRPCServer *grpc.Server
	port       string

	sessions        *usecase.Sessions
	cleanupInterval time.Duration
	stopCleanup     chan struct{}
	cleanupDone     chan struct{}
}

func NewApp(
	log *slog.Logger,
	port string,
	tokenManager *jwt.Manager,
	userRepository *repository.UserRepository,
	sessionRepository *repository.SessionRepository,
	oauthStateRepository *repository.OAuthStateRepository,
	oauthProviders map[string]usecase.OAuthProvider,
	oauthStateTTL time.Duration,
	sessionCfg config.SessionConfig,
) *App {
	gRPCServer := grpc.NewServer(grpc.ChainUnaryInterceptor(interceptor.Logging(log)))

	sessions := usecase.NewSessions(log, sessionRepository, userRepository, tokenManager, sessionCfg.RefreshTTL)
	authUseCase := usecase.NewAuthUseCase(log, userRepository, userRepository, sessions)
	oauthUseCase := usecase.NewOAuthUseCase(
		log,
		oauthProviders,
		userRepository,
		oauthStateRepository,
		sessions,
		oauthStateTTL,
	)

	authgrpc.RegisterServer(gRPCServer, authUseCase, oauthUseCase)

	return &App{
		log:             log,
		gRPCServer:      gRPCServer,
		port:            port,
		sessions:        sessions,
		cleanupInterval: sessionCfg.CleanupInterval,
		stopCleanup:     make(chan struct{}),
		cleanupDone:     make(chan struct{}),
	}
}

func (a *App) MustRun() {
	if err := a.Run(); err != nil {
		panic(err)
	}
}

func (a *App) Run() error {
	const op = "grpcapp.Run"
	log := a.log.With(
		slog.String("op", op),
		slog.String("port", a.port),
	)

	go a.runSessionCleanup()

	log.Info("starting gRPC server")

	l, err := net.Listen("tcp", net.JoinHostPort("", a.port))
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	log.Info("gRPC server started", slog.String("addr", l.Addr().String()))

	if err := a.gRPCServer.Serve(l); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}

func (a *App) Stop() {
	const op = "grpcapp.Stop"
	log := a.log.With(slog.String("op", op))

	log.Info("stopping gRPC server")

	a.gRPCServer.GracefulStop()

	log.Info("gRPC server stopped")

	close(a.stopCleanup)
	<-a.cleanupDone
}

func (a *App) runSessionCleanup() {
	defer close(a.cleanupDone)

	ticker := time.NewTicker(a.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-a.stopCleanup:
			return
		case <-ticker.C:

			_ = a.sessions.DeleteExpired(context.Background())
		}
	}
}

package app

import (
	"fmt"
	"log/slog"
	"net"

	"github.com/andrey1af/shop-api/backend/image-service/internal/config"
	"github.com/andrey1af/shop-api/backend/image-service/internal/repository"
	imagegrpc "github.com/andrey1af/shop-api/backend/image-service/internal/transport/grpc"
	"github.com/andrey1af/shop-api/backend/image-service/internal/transport/grpc/interceptor"
	"github.com/andrey1af/shop-api/backend/image-service/internal/usecase"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthv1 "google.golang.org/grpc/health/grpc_health_v1"
)

type App struct {
	log        *slog.Logger
	gRPCServer *grpc.Server
	health     *health.Server
	port       string
}

func NewApp(log *slog.Logger, cfg config.GRPCConfig, imageRepository *repository.ImageRepository) *App {
	gRPCServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(interceptor.Logging(log)),
		grpc.MaxRecvMsgSize(cfg.MaxMessageBytes),
	)

	imageUseCase := usecase.NewImageUseCase(log, imageRepository)

	imagegrpc.RegisterServer(gRPCServer, imageUseCase)

	healthServer := health.NewServer()
	healthv1.RegisterHealthServer(gRPCServer, healthServer)

	return &App{
		log:        log,
		gRPCServer: gRPCServer,
		health:     healthServer,
		port:       cfg.Port,
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

	a.health.Shutdown()
	a.gRPCServer.GracefulStop()

	log.Info("gRPC server stopped")
}

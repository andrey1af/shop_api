package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"

	"github.com/andrey1af/shop-api/backend/api-service/internal/broadcast"
	authgrpc "github.com/andrey1af/shop-api/backend/api-service/internal/clients/auth/grpc"
	"github.com/andrey1af/shop-api/backend/api-service/internal/config"
	"github.com/andrey1af/shop-api/backend/api-service/internal/repository"
	handlers "github.com/andrey1af/shop-api/backend/api-service/internal/transport/http"
	"github.com/andrey1af/shop-api/backend/api-service/internal/transport/http/middleware"
	"github.com/andrey1af/shop-api/backend/api-service/internal/transport/kafka"
	"github.com/andrey1af/shop-api/backend/api-service/internal/usecase"
)

type App struct {
	log    *slog.Logger
	server *http.Server
	cfg    config.HTTPConfig

	mu              sync.Mutex
	consumer        *kafka.Consumer
	stopConsumer    context.CancelFunc
	consumerStopped chan struct{}
	stopped         bool
}

func NewApp(
	log *slog.Logger,
	cfg config.HTTPConfig,
	kafkaCfg config.KafkaConfig,
	authClient *authgrpc.Client,
	clientRepository *repository.ClientRepository,
	supplierRepository *repository.SupplierRepository,
	productRepository *repository.ProductRepository,
	imageRepository *repository.ImageRepository,
	databasePinger handlers.Pinger,
	storagePinger handlers.Pinger,
) *App {
	authUseCase := usecase.NewAuthUseCase(log, authClient)
	clientUseCase := usecase.NewClientUseCase(log, clientRepository)
	supplierUseCase := usecase.NewSupplierUseCase(log, supplierRepository, productRepository)
	productHub := broadcast.NewProductHub(log)
	productUseCase := usecase.NewProductUseCase(log, productRepository, supplierRepository, productHub)
	imageUseCase := usecase.NewImageUseCase(log, imageRepository, productRepository)

	consumer := kafka.NewConsumer(
		log,
		kafkaCfg.Brokers,
		kafkaCfg.ProductUpdatesTopic,
		kafkaCfg.ConsumerGroupID,
		productUseCase,
	)

	healthHandler := handlers.NewHealthHandler(cfg.HealthTimeout, databasePinger, storagePinger)
	authHandler := handlers.NewAuthHandler(authUseCase, cfg.RefreshCookieSecure)
	clientHandler := handlers.NewClientHandler(clientUseCase)
	supplierHandler := handlers.NewSupplierHandler(supplierUseCase)
	productHandler := handlers.NewProductHandler(productUseCase)
	imageHandler := handlers.NewImageHandler(imageUseCase)
	productStreamHandler := handlers.NewProductStreamHandler(productHub)

	router := handlers.NewRouter(
		middleware.Auth(authUseCase),
		middleware.QueryTokenAuth(authUseCase),
		middleware.NewRateLimiter(cfg.AuthRateLimitPerMinute, cfg.AuthRateLimitBurst).Middleware,
		healthHandler,
		authHandler,
		clientHandler,
		supplierHandler,
		productHandler,
		imageHandler,
		productStreamHandler,
	)

	server := &http.Server{
		Addr:              net.JoinHostPort("", cfg.Port),
		Handler:           middleware.Logging(log)(router),
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
	}

	server.RegisterOnShutdown(productHub.Close)

	return &App{
		log:             log,
		server:          server,
		cfg:             cfg,
		consumer:        consumer,
		consumerStopped: make(chan struct{}),
	}
}

func (a *App) MustRun() {
	if err := a.Run(); err != nil {
		panic(err)
	}
}

func (a *App) Run() error {
	const op = "httpapp.Run"
	log := a.log.With(
		slog.String("op", op),
		slog.String("port", a.cfg.Port),
	)

	a.startKafkaConsumer()

	log.Info("starting http server")

	if err := a.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}

func (a *App) Stop() {
	const op = "httpapp.Stop"
	log := a.log.With(slog.String("op", op))

	defer a.stopKafkaConsumer()

	log.Info("stopping http server")

	ctx, cancel := context.WithTimeout(context.Background(), a.cfg.ShutdownTimeout)
	defer cancel()

	if err := a.server.Shutdown(ctx); err != nil {
		log.Error("failed to shutdown http server", slog.String("error", err.Error()))
		return
	}

	log.Info("http server stopped")
}

func (a *App) startKafkaConsumer() {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.stopped || a.stopConsumer != nil {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	a.stopConsumer = cancel

	go func() {
		defer close(a.consumerStopped)
		a.consumer.Run(ctx)
	}()
}

func (a *App) stopKafkaConsumer() {
	const op = "kafkaapp.Stop"
	log := a.log.With(slog.String("op", op))

	a.mu.Lock()
	a.stopped = true
	stopConsumer := a.stopConsumer
	a.mu.Unlock()

	if stopConsumer == nil {
		return
	}

	log.Info("stopping kafka consumer")

	stopConsumer()
	<-a.consumerStopped

	if err := a.consumer.Close(); err != nil {
		log.Error("failed to close kafka consumer", slog.String("error", err.Error()))
		return
	}

	log.Info("kafka consumer stopped")
}

package app

import (
	"context"
	"log/slog"
	"time"

	"github.com/andrey1af/shop-api/backend/generator-service/internal/domain"
)

type generator interface {
	RefreshProducts(ctx context.Context) error
	Tick(ctx context.Context) (domain.ProductUpdated, error)
}

type App struct {
	log             *slog.Logger
	generator       generator
	interval        time.Duration
	refreshInterval time.Duration
}

func NewApp(log *slog.Logger, generator generator, interval, refreshInterval time.Duration) *App {
	return &App{
		log:             log,
		generator:       generator,
		interval:        interval,
		refreshInterval: refreshInterval,
	}
}

func (a *App) Run(ctx context.Context) {
	const op = "app.Run"
	log := a.log.With(slog.String("op", op))

	log.Info("generator started",
		slog.Duration("interval", a.interval),
		slog.Duration("refresh_interval", a.refreshInterval),
	)

	_ = a.generator.RefreshProducts(ctx)

	tick := time.NewTicker(a.interval)
	defer tick.Stop()

	refresh := time.NewTicker(a.refreshInterval)
	defer refresh.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info("generator stopped")
			return
		case <-refresh.C:
			_ = a.generator.RefreshProducts(ctx)
		case <-tick.C:
			_, _ = a.generator.Tick(ctx)
		}
	}
}

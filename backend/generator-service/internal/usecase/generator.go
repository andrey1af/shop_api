package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"math/rand/v2"
	"sync"
	"time"
	"uuid"

	"github.com/andrey1af/shop-api/backend/generator-service/internal/domain"
	apperrors "github.com/andrey1af/shop-api/backend/generator-service/internal/errors"
)

const minPrice = 0.01

type productSource interface {
	ListProducts(ctx context.Context) ([]domain.Product, error)
}

type eventPublisher interface {
	PublishProductUpdated(ctx context.Context, event domain.ProductUpdated) error
}

type GeneratorSettings struct {
	PriceDeltaPercent float64

	StockMax int64
}

type GeneratorUseCase struct {
	log       *slog.Logger
	source    productSource
	publisher eventPublisher
	settings  GeneratorSettings

	rand  *rand.Rand
	now   func() time.Time
	newID func() uuid.UUID

	mu         sync.Mutex
	products   []domain.Product
	basePrices map[uuid.UUID]float64
}

func NewGeneratorUseCase(
	log *slog.Logger,
	source productSource,
	publisher eventPublisher,
	settings GeneratorSettings,
) *GeneratorUseCase {
	return &GeneratorUseCase{
		log:       log,
		source:    source,
		publisher: publisher,
		settings:  settings,
		rand:      rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64())),
		now:       time.Now,
		newID:     uuid.NewV7,

		basePrices: make(map[uuid.UUID]float64),
	}
}

func (uc *GeneratorUseCase) RefreshProducts(ctx context.Context) error {
	const op = "generatorUseCase.RefreshProducts"
	log := uc.log.With(slog.String("op", op))

	products, err := uc.source.ListProducts(ctx)
	if err != nil {
		log.Error("failed to list products", slog.String("error", err.Error()))

		return fmt.Errorf("%s: %w", op, err)
	}

	uc.mu.Lock()
	uc.products = products
	for _, p := range products {
		if _, ok := uc.basePrices[p.ID]; !ok {
			uc.basePrices[p.ID] = p.Price
		}
	}
	uc.mu.Unlock()

	log.Debug("products refreshed", slog.Int("count", len(products)))

	return nil
}

func (uc *GeneratorUseCase) Tick(ctx context.Context) (domain.ProductUpdated, error) {
	const op = "generatorUseCase.Tick"
	log := uc.log.With(slog.String("op", op))

	uc.mu.Lock()
	defer uc.mu.Unlock()

	if len(uc.products) == 0 {
		log.Warn("no products to update")

		return domain.ProductUpdated{}, fmt.Errorf("%s: %w", op, apperrors.ErrNoProducts)
	}

	idx := uc.rand.IntN(len(uc.products))
	product := uc.products[idx]

	event := domain.ProductUpdated{
		EventID:        uc.newID(),
		ProductID:      product.ID,
		Price:          uc.nextPrice(uc.basePrice(product)),
		AvailableStock: uc.rand.Int64N(uc.settings.StockMax + 1),
		OccurredAt:     uc.now().UTC(),
	}

	if err := uc.publisher.PublishProductUpdated(ctx, event); err != nil {
		log.Error("failed to publish product update",
			slog.String("product_id", product.ID.String()),
			slog.String("error", err.Error()),
		)

		return domain.ProductUpdated{}, fmt.Errorf("%s: %w", op, err)
	}

	uc.products[idx].Price = event.Price
	uc.products[idx].AvailableStock = event.AvailableStock

	log.Info("product update published",
		slog.String("event_id", event.EventID.String()),
		slog.String("product_id", event.ProductID.String()),
		slog.Float64("price", event.Price),
		slog.Int64("available_stock", event.AvailableStock),
	)

	return event, nil
}

func (uc *GeneratorUseCase) basePrice(product domain.Product) float64 {
	if base, ok := uc.basePrices[product.ID]; ok {
		return base
	}

	return product.Price
}

func (uc *GeneratorUseCase) nextPrice(base float64) float64 {
	delta := (uc.rand.Float64()*2 - 1) * uc.settings.PriceDeltaPercent / 100
	price := math.Round(base*(1+delta)*100) / 100

	return max(price, minPrice)
}

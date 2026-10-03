package usecase

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"math/rand/v2"
	"testing"
	"time"
	"uuid"

	"github.com/andrey1af/shop-api/backend/generator-service/internal/domain"
	apperrors "github.com/andrey1af/shop-api/backend/generator-service/internal/errors"
)

type fakeProductSource struct {
	products []domain.Product
	err      error
}

func (f *fakeProductSource) ListProducts(context.Context) ([]domain.Product, error) {
	return f.products, f.err
}

type fakeEventPublisher struct {
	err       error
	published []domain.ProductUpdated
}

func (f *fakeEventPublisher) PublishProductUpdated(_ context.Context, event domain.ProductUpdated) error {
	if f.err != nil {
		return f.err
	}
	f.published = append(f.published, event)
	return nil
}

func newTestUseCase(
	source *fakeProductSource,
	publisher *fakeEventPublisher,
	settings GeneratorSettings,
) *GeneratorUseCase {
	uc := NewGeneratorUseCase(slog.New(slog.NewTextHandler(io.Discard, nil)), source, publisher, settings)
	uc.rand = rand.New(rand.NewPCG(1, 2))
	return uc
}

func TestGeneratorUseCase_Tick_NoProducts(t *testing.T) {
	publisher := &fakeEventPublisher{}
	uc := newTestUseCase(&fakeProductSource{}, publisher, GeneratorSettings{PriceDeltaPercent: 20, StockMax: 100})

	if err := uc.RefreshProducts(context.Background()); err != nil {
		t.Fatalf("RefreshProducts() error = %v", err)
	}

	_, err := uc.Tick(context.Background())
	if !errors.Is(err, apperrors.ErrNoProducts) {
		t.Fatalf("Tick() error = %v, want %v", err, apperrors.ErrNoProducts)
	}
	if len(publisher.published) != 0 {
		t.Fatalf("published %d events, want 0", len(publisher.published))
	}
}

func TestGeneratorUseCase_Tick_PublishesEventWithinBounds(t *testing.T) {
	productID := uuid.New()
	eventID := uuid.New()
	now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)

	source := &fakeProductSource{products: []domain.Product{{ID: productID, Price: 100, AvailableStock: 5}}}
	publisher := &fakeEventPublisher{}
	settings := GeneratorSettings{PriceDeltaPercent: 20, StockMax: 10}
	uc := newTestUseCase(source, publisher, settings)
	uc.now = func() time.Time { return now }
	uc.newID = func() uuid.UUID { return eventID }

	if err := uc.RefreshProducts(context.Background()); err != nil {
		t.Fatalf("RefreshProducts() error = %v", err)
	}

	for range 1000 {
		event, err := uc.Tick(context.Background())
		if err != nil {
			t.Fatalf("Tick() error = %v", err)
		}

		if event.ProductID != productID || event.EventID != eventID || !event.OccurredAt.Equal(now) {
			t.Fatalf("Tick() event = %+v, unexpected identity fields", event)
		}
		if event.AvailableStock < 0 || event.AvailableStock > settings.StockMax {
			t.Fatalf("AvailableStock = %d, want in [0, %d]", event.AvailableStock, settings.StockMax)
		}
		if event.Price < 80 || event.Price > 120 {
			t.Fatalf("Price = %v, want within ±20%% of base price 100", event.Price)
		}
	}

	if len(publisher.published) != 1000 {
		t.Fatalf("published %d events, want 1000", len(publisher.published))
	}
}

func TestGeneratorUseCase_Tick_PriceNeverBelowMinimum(t *testing.T) {
	source := &fakeProductSource{products: []domain.Product{{ID: uuid.New(), Price: minPrice, AvailableStock: 1}}}
	uc := newTestUseCase(source, &fakeEventPublisher{}, GeneratorSettings{PriceDeltaPercent: 99, StockMax: 0})

	if err := uc.RefreshProducts(context.Background()); err != nil {
		t.Fatalf("RefreshProducts() error = %v", err)
	}

	for range 100 {
		event, err := uc.Tick(context.Background())
		if err != nil {
			t.Fatalf("Tick() error = %v", err)
		}
		if event.Price < minPrice {
			t.Fatalf("Price = %v, want >= %v", event.Price, minPrice)
		}
		if event.AvailableStock != 0 {
			t.Fatalf("AvailableStock = %d, want 0", event.AvailableStock)
		}
	}
}

func TestGeneratorUseCase_Tick_PublishErrorKeepsSnapshot(t *testing.T) {
	product := domain.Product{ID: uuid.New(), Price: 50, AvailableStock: 7}
	publishErr := errors.New("broker unavailable")

	publisher := &fakeEventPublisher{err: publishErr}
	uc := newTestUseCase(&fakeProductSource{products: []domain.Product{product}}, publisher, GeneratorSettings{
		PriceDeltaPercent: 20,
		StockMax:          100,
	})

	if err := uc.RefreshProducts(context.Background()); err != nil {
		t.Fatalf("RefreshProducts() error = %v", err)
	}

	if _, err := uc.Tick(context.Background()); !errors.Is(err, publishErr) {
		t.Fatalf("Tick() error = %v, want %v", err, publishErr)
	}
	if uc.products[0] != product {
		t.Fatalf("snapshot = %+v, want unchanged %+v", uc.products[0], product)
	}
}

func TestGeneratorUseCase_RefreshProducts_KeepsSnapshotOnError(t *testing.T) {
	product := domain.Product{ID: uuid.New(), Price: 10, AvailableStock: 1}
	source := &fakeProductSource{products: []domain.Product{product}}
	uc := newTestUseCase(source, &fakeEventPublisher{}, GeneratorSettings{PriceDeltaPercent: 20, StockMax: 100})

	if err := uc.RefreshProducts(context.Background()); err != nil {
		t.Fatalf("RefreshProducts() error = %v", err)
	}

	listErr := errors.New("api unavailable")
	source.products, source.err = nil, listErr

	if err := uc.RefreshProducts(context.Background()); !errors.Is(err, listErr) {
		t.Fatalf("RefreshProducts() error = %v, want %v", err, listErr)
	}
	if len(uc.products) != 1 || uc.products[0] != product {
		t.Fatalf("snapshot = %+v, want previous snapshot kept", uc.products)
	}
}

func TestGeneratorUseCase_PriceStaysAroundFirstSeenPrice(t *testing.T) {
	productID := uuid.New()
	source := &fakeProductSource{products: []domain.Product{{ID: productID, Price: 1000, AvailableStock: 1}}}
	uc := newTestUseCase(source, &fakeEventPublisher{}, GeneratorSettings{PriceDeltaPercent: 20, StockMax: 10})

	if err := uc.RefreshProducts(context.Background()); err != nil {
		t.Fatal(err)
	}

	for i := range 50 {
		for range 100 {
			event, err := uc.Tick(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if event.Price < 800 || event.Price > 1200 {
				t.Fatalf("round %d: price drifted to %v, want within [800, 1200]", i, event.Price)
			}
		}

		source.products = []domain.Product{{ID: productID, Price: 850, AvailableStock: 1}}
		if err := uc.RefreshProducts(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
}

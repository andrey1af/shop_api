package usecase

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
	"uuid"

	"github.com/andrey1af/shop-api/backend/api-service/internal/domain"
	apperrors "github.com/andrey1af/shop-api/backend/api-service/internal/errors"
)

type fakeProductRepository struct {
	createFunc              func(ctx context.Context, product domain.Product) (domain.Product, error)
	deleteFunc              func(ctx context.Context, productID uuid.UUID) error
	getFunc                 func(ctx context.Context, productID uuid.UUID) (domain.Product, error)
	listAvailableFunc       func(ctx context.Context, p domain.ListParams) ([]domain.Product, error)
	updateStockFunc         func(ctx context.Context, productID uuid.UUID, newStock int64) (domain.Product, error)
	updatePriceAndStockFunc func(
		ctx context.Context,
		productID uuid.UUID,
		price float64,
		newStock int64,
	) (domain.Product, error)
	existsBySupplierIDFunc func(ctx context.Context, supplierID uuid.UUID) (bool, error)

	gotCreateProduct  domain.Product
	gotDeleteID       uuid.UUID
	gotGetID          uuid.UUID
	gotUpdateStockID  uuid.UUID
	gotUpdateStockNew int64
	updateStockCalled bool
	createCalled      bool
}

func (f *fakeProductRepository) Create(ctx context.Context, product domain.Product) (domain.Product, error) {
	f.gotCreateProduct = product
	f.createCalled = true
	return f.createFunc(ctx, product)
}

func (f *fakeProductRepository) Delete(ctx context.Context, productID uuid.UUID) error {
	f.gotDeleteID = productID
	return f.deleteFunc(ctx, productID)
}

func (f *fakeProductRepository) Get(ctx context.Context, productID uuid.UUID) (domain.Product, error) {
	f.gotGetID = productID
	return f.getFunc(ctx, productID)
}

func (f *fakeProductRepository) ListAvailable(ctx context.Context, p domain.ListParams) ([]domain.Product, error) {
	return f.listAvailableFunc(ctx, p)
}

func (f *fakeProductRepository) UpdateStock(
	ctx context.Context,
	productID uuid.UUID,
	newStock int64,
) (domain.Product, error) {
	f.gotUpdateStockID = productID
	f.gotUpdateStockNew = newStock
	f.updateStockCalled = true
	return f.updateStockFunc(ctx, productID, newStock)
}

func (f *fakeProductRepository) UpdatePriceAndStock(
	ctx context.Context,
	productID uuid.UUID,
	price float64,
	newStock int64,
) (domain.Product, error) {
	return f.updatePriceAndStockFunc(ctx, productID, price, newStock)
}

func (f *fakeProductRepository) ExistsBySupplierID(ctx context.Context, supplierID uuid.UUID) (bool, error) {
	return f.existsBySupplierIDFunc(ctx, supplierID)
}

type fakeProductUpdateNotifier struct {
	notified []domain.Product
}

func (f *fakeProductUpdateNotifier) NotifyProductUpdated(product domain.Product) {
	f.notified = append(f.notified, product)
}

func newTestProductUseCase(repo ProductRepository, now time.Time) *ProductUseCase {
	uc, _ := newTestProductUseCaseWithNotifier(repo, now)
	return uc
}

func newTestProductUseCaseWithNotifier(
	repo ProductRepository,
	now time.Time,
) (*ProductUseCase, *fakeProductUpdateNotifier) {
	notifier := &fakeProductUpdateNotifier{}
	uc := NewProductUseCase(newTestLogger(), repo, &fakeSupplierRepository{
		getFunc: func(ctx context.Context, id uuid.UUID) (domain.Supplier, error) {
			return domain.Supplier{ID: id}, nil
		},
	}, notifier)
	uc.now = func() time.Time { return now }
	return uc, notifier
}

func TestProductUseCase_Create(t *testing.T) {
	t.Run("maps input, generates id and uses injected clock", func(t *testing.T) {
		fixedNow := time.Date(2026, 1, 2, 10, 0, 0, 0, time.UTC)
		supplierID := uuid.New()

		repo := &fakeProductRepository{
			createFunc: func(ctx context.Context, product domain.Product) (domain.Product, error) {
				return product, nil
			},
		}
		uc := newTestProductUseCase(repo, fixedNow)

		in := CreateProductInput{
			Name:           "Холодильник Frost 3000",
			Category:       "Крупная бытовая техника",
			Price:          74990.00,
			AvailableStock: 12,
			SupplierID:     supplierID,
		}

		got, err := uc.Create(context.Background(), in)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if got.ID == uuid.Nil() {
			t.Error("expected generated product ID, got zero value")
		}
		if got.Name != in.Name || got.Category != in.Category {
			t.Errorf("name/category not mapped correctly: got %+v", got)
		}
		if got.Price != in.Price {
			t.Errorf("price not mapped: got %v, want %v", got.Price, in.Price)
		}
		if got.AvailableStock != in.AvailableStock {
			t.Errorf("available stock not mapped: got %v, want %v", got.AvailableStock, in.AvailableStock)
		}
		if got.SupplierID != in.SupplierID {
			t.Errorf("supplier id not mapped: got %v, want %v", got.SupplierID, in.SupplierID)
		}
		if !got.LastUpdateDate.Equal(fixedNow) {
			t.Errorf("last update date = %v, want injected clock value %v", got.LastUpdateDate, fixedNow)
		}
		if got.ImageID != nil {
			t.Errorf("expected nil image id on creation, got %v", got.ImageID)
		}
	})

	t.Run("generates distinct ids across calls", func(t *testing.T) {
		repo := &fakeProductRepository{
			createFunc: func(ctx context.Context, product domain.Product) (domain.Product, error) {
				return product, nil
			},
		}
		uc := newTestProductUseCase(repo, time.Now())

		first, err := uc.Create(context.Background(), CreateProductInput{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		second, err := uc.Create(context.Background(), CreateProductInput{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if first.ID == second.ID {
			t.Error("expected different product IDs across calls")
		}
	})

	t.Run("validates supplier exists before creating product", func(t *testing.T) {
		supplierID := uuid.New()
		var gotSupplierID uuid.UUID
		supplierRepo := &fakeSupplierRepository{
			getFunc: func(ctx context.Context, id uuid.UUID) (domain.Supplier, error) {
				gotSupplierID = id
				return domain.Supplier{ID: id}, nil
			},
		}
		repo := &fakeProductRepository{
			createFunc: func(ctx context.Context, product domain.Product) (domain.Product, error) {
				return product, nil
			},
		}
		uc := NewProductUseCase(newTestLogger(), repo, supplierRepo, &fakeProductUpdateNotifier{})

		if _, err := uc.Create(context.Background(), CreateProductInput{SupplierID: supplierID}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gotSupplierID != supplierID {
			t.Errorf("supplier id checked incorrectly: got %v, want %v", gotSupplierID, supplierID)
		}
		if !repo.createCalled {
			t.Error("expected repository Create to be called after supplier check passes")
		}
	})

	t.Run("propagates ErrSupplierNotFound from supplier check without calling repository Create", func(t *testing.T) {
		supplierRepo := &fakeSupplierRepository{
			getFunc: func(ctx context.Context, id uuid.UUID) (domain.Supplier, error) {
				return domain.Supplier{}, domain.ErrSupplierNotFound
			},
		}
		repo := &fakeProductRepository{}
		uc := NewProductUseCase(newTestLogger(), repo, supplierRepo, &fakeProductUpdateNotifier{})

		got, err := uc.Create(context.Background(), CreateProductInput{})
		if !errors.Is(err, domain.ErrSupplierNotFound) {
			t.Fatalf("expected ErrSupplierNotFound, got %v", err)
		}
		if !reflect.DeepEqual(got, domain.Product{}) {
			t.Errorf("expected zero value product on error, got %+v", got)
		}
		if repo.createCalled {
			t.Error("expected repository Create not to be called when supplier is not found")
		}
	})

	t.Run("propagates generic repository error and returns zero value", func(t *testing.T) {
		repoErr := errors.New("insert failed")
		repo := &fakeProductRepository{
			createFunc: func(ctx context.Context, product domain.Product) (domain.Product, error) {
				return domain.Product{}, repoErr
			},
		}
		uc := newTestProductUseCase(repo, time.Now())

		got, err := uc.Create(context.Background(), CreateProductInput{})
		if !errors.Is(err, repoErr) {
			t.Fatalf("expected error %v, got %v", repoErr, err)
		}
		if !reflect.DeepEqual(got, domain.Product{}) {
			t.Errorf("expected zero value product on error, got %+v", got)
		}
	})

	t.Run("returns product exactly as returned by repository", func(t *testing.T) {
		stored := domain.Product{ID: uuid.New(), Name: "Stored"}
		repo := &fakeProductRepository{
			createFunc: func(ctx context.Context, product domain.Product) (domain.Product, error) {
				return stored, nil
			},
		}
		uc := newTestProductUseCase(repo, time.Now())

		got, err := uc.Create(context.Background(), CreateProductInput{Name: "Input"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !reflect.DeepEqual(got, stored) {
			t.Errorf("expected usecase to return repository result unchanged, got %+v, want %+v", got, stored)
		}
	})
}

func TestProductUseCase_Delete(t *testing.T) {
	t.Run("returns nil on success and forwards product id", func(t *testing.T) {
		productID := uuid.New()
		repo := &fakeProductRepository{
			deleteFunc: func(ctx context.Context, id uuid.UUID) error {
				return nil
			},
		}
		uc := newTestProductUseCase(repo, time.Now())

		if err := uc.Delete(context.Background(), productID); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if repo.gotDeleteID != productID {
			t.Errorf("product id passed to repository = %v, want %v", repo.gotDeleteID, productID)
		}
	})

	t.Run("propagates ErrProductNotFound", func(t *testing.T) {
		repo := &fakeProductRepository{
			deleteFunc: func(ctx context.Context, id uuid.UUID) error {
				return domain.ErrProductNotFound
			},
		}
		uc := newTestProductUseCase(repo, time.Now())

		err := uc.Delete(context.Background(), uuid.New())
		if !errors.Is(err, domain.ErrProductNotFound) {
			t.Fatalf("expected ErrProductNotFound, got %v", err)
		}
	})

	t.Run("propagates generic repository error", func(t *testing.T) {
		repoErr := errors.New("connection lost")
		repo := &fakeProductRepository{
			deleteFunc: func(ctx context.Context, id uuid.UUID) error {
				return repoErr
			},
		}
		uc := newTestProductUseCase(repo, time.Now())

		err := uc.Delete(context.Background(), uuid.New())
		if !errors.Is(err, repoErr) {
			t.Fatalf("expected %v, got %v", repoErr, err)
		}
	})
}

func TestProductUseCase_Get(t *testing.T) {
	t.Run("returns product from repository unchanged and forwards id", func(t *testing.T) {
		productID := uuid.New()
		want := domain.Product{ID: productID, Name: "Product A"}
		repo := &fakeProductRepository{
			getFunc: func(ctx context.Context, id uuid.UUID) (domain.Product, error) {
				return want, nil
			},
		}
		uc := newTestProductUseCase(repo, time.Now())

		got, err := uc.Get(context.Background(), productID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %+v, want %+v", got, want)
		}
		if repo.gotGetID != productID {
			t.Errorf("product id forwarded incorrectly: got %v, want %v", repo.gotGetID, productID)
		}
	})

	t.Run("propagates ErrProductNotFound and zero value", func(t *testing.T) {
		repo := &fakeProductRepository{
			getFunc: func(ctx context.Context, id uuid.UUID) (domain.Product, error) {
				return domain.Product{}, domain.ErrProductNotFound
			},
		}
		uc := newTestProductUseCase(repo, time.Now())

		got, err := uc.Get(context.Background(), uuid.New())
		if !errors.Is(err, domain.ErrProductNotFound) {
			t.Fatalf("expected ErrProductNotFound, got %v", err)
		}
		if !reflect.DeepEqual(got, domain.Product{}) {
			t.Errorf("expected zero value on error, got %+v", got)
		}
	})
}

func TestProductUseCase_ListAvailable(t *testing.T) {
	t.Run("returns products from repository unchanged", func(t *testing.T) {
		want := []domain.Product{{Name: "A"}, {Name: "B"}}
		repo := &fakeProductRepository{
			listAvailableFunc: func(ctx context.Context, p domain.ListParams) ([]domain.Product, error) {
				return want, nil
			},
		}
		uc := newTestProductUseCase(repo, time.Now())

		got, err := uc.ListAvailable(context.Background(), domain.ListParams{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %+v, want %+v", got, want)
		}
	})

	t.Run("returns nil slice from repository as-is when no matches", func(t *testing.T) {
		repo := &fakeProductRepository{
			listAvailableFunc: func(ctx context.Context, p domain.ListParams) ([]domain.Product, error) {
				return nil, nil
			},
		}
		uc := newTestProductUseCase(repo, time.Now())

		got, err := uc.ListAvailable(context.Background(), domain.ListParams{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != nil {
			t.Errorf("expected nil slice to be passed through, got %+v", got)
		}
	})

	t.Run("returns non-nil empty slice and error on repository failure", func(t *testing.T) {
		repoErr := errors.New("query failed")
		repo := &fakeProductRepository{
			listAvailableFunc: func(ctx context.Context, p domain.ListParams) ([]domain.Product, error) {
				return nil, repoErr
			},
		}
		uc := newTestProductUseCase(repo, time.Now())

		got, err := uc.ListAvailable(context.Background(), domain.ListParams{})
		if !errors.Is(err, repoErr) {
			t.Fatalf("expected %v, got %v", repoErr, err)
		}
		if got == nil || len(got) != 0 {
			t.Errorf("expected non-nil empty slice on error, got %+v", got)
		}
	})
}

func TestProductUseCase_ListAvailable_PassesPagination(t *testing.T) {
	limit, offset := 20, 40
	var got domain.ListParams
	repo := &fakeProductRepository{
		listAvailableFunc: func(ctx context.Context, p domain.ListParams) ([]domain.Product, error) {
			got = p
			return nil, nil
		},
	}
	uc := newTestProductUseCase(repo, time.Now())

	if _, err := uc.ListAvailable(context.Background(), domain.ListParams{Limit: &limit, Offset: &offset}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Limit == nil || *got.Limit != 20 || got.Offset == nil || *got.Offset != 40 {
		t.Errorf("pagination forwarded incorrectly: %+v", got)
	}
}

func TestProductUseCase_DecreaseStock(t *testing.T) {
	t.Run("reads current stock, computes new value in usecase and forwards it to repository", func(t *testing.T) {
		productID := uuid.New()
		updated := domain.Product{ID: productID, AvailableStock: 10}
		repo := &fakeProductRepository{
			getFunc: func(ctx context.Context, id uuid.UUID) (domain.Product, error) {
				return domain.Product{ID: productID, AvailableStock: 12}, nil
			},
			updateStockFunc: func(ctx context.Context, id uuid.UUID, newStock int64) (domain.Product, error) {
				return updated, nil
			},
		}
		uc, notifier := newTestProductUseCaseWithNotifier(repo, time.Now())

		got, err := uc.DecreaseStock(context.Background(), productID, 2)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(notifier.notified) != 1 || !reflect.DeepEqual(notifier.notified[0], updated) {
			t.Errorf("expected updated product to be notified once, got %+v", notifier.notified)
		}
		if repo.gotGetID != productID {
			t.Errorf("product id passed to Get incorrectly: got %v, want %v", repo.gotGetID, productID)
		}
		if repo.gotUpdateStockID != productID {
			t.Errorf("product id forwarded to UpdateStock incorrectly: got %v, want %v", repo.gotUpdateStockID, productID)
		}
		if repo.gotUpdateStockNew != 10 {
			t.Errorf("new stock forwarded incorrectly: got %v, want %v", repo.gotUpdateStockNew, 10)
		}
		if !reflect.DeepEqual(got, updated) {
			t.Errorf("expected usecase to return repository result unchanged, got %+v, want %+v", got, updated)
		}
	})

	t.Run("allows decreasing stock down to exactly zero", func(t *testing.T) {
		productID := uuid.New()
		repo := &fakeProductRepository{
			getFunc: func(ctx context.Context, id uuid.UUID) (domain.Product, error) {
				return domain.Product{ID: productID, AvailableStock: 5}, nil
			},
			updateStockFunc: func(ctx context.Context, id uuid.UUID, newStock int64) (domain.Product, error) {
				return domain.Product{ID: productID, AvailableStock: newStock}, nil
			},
		}
		uc := newTestProductUseCase(repo, time.Now())

		if _, err := uc.DecreaseStock(context.Background(), productID, 5); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if repo.gotUpdateStockNew != 0 {
			t.Errorf("expected new stock 0, got %v", repo.gotUpdateStockNew)
		}
	})

	t.Run("returns ErrInsufficientStock and skips UpdateStock when amount exceeds stock", func(t *testing.T) {
		repo := &fakeProductRepository{
			getFunc: func(ctx context.Context, id uuid.UUID) (domain.Product, error) {
				return domain.Product{ID: id, AvailableStock: 1}, nil
			},
		}
		uc := newTestProductUseCase(repo, time.Now())

		got, err := uc.DecreaseStock(context.Background(), uuid.New(), 100)
		if !errors.Is(err, domain.ErrInsufficientStock) {
			t.Fatalf("expected ErrInsufficientStock, got %v", err)
		}
		if !reflect.DeepEqual(got, domain.Product{}) {
			t.Errorf("expected zero value on error, got %+v", got)
		}
		if repo.updateStockCalled {
			t.Error("expected UpdateStock not to be called when stock is insufficient")
		}
	})

	t.Run("propagates ErrProductNotFound from Get without calling UpdateStock", func(t *testing.T) {
		repo := &fakeProductRepository{
			getFunc: func(ctx context.Context, id uuid.UUID) (domain.Product, error) {
				return domain.Product{}, domain.ErrProductNotFound
			},
		}
		uc := newTestProductUseCase(repo, time.Now())

		_, err := uc.DecreaseStock(context.Background(), uuid.New(), 1)
		if !errors.Is(err, domain.ErrProductNotFound) {
			t.Fatalf("expected ErrProductNotFound, got %v", err)
		}
		if repo.updateStockCalled {
			t.Error("expected UpdateStock not to be called when product is not found")
		}
	})

	t.Run("propagates generic repository error from UpdateStock and returns zero value", func(t *testing.T) {
		repoErr := errors.New("update failed")
		repo := &fakeProductRepository{
			getFunc: func(ctx context.Context, id uuid.UUID) (domain.Product, error) {
				return domain.Product{ID: id, AvailableStock: 10}, nil
			},
			updateStockFunc: func(ctx context.Context, id uuid.UUID, newStock int64) (domain.Product, error) {
				return domain.Product{}, repoErr
			},
		}
		uc := newTestProductUseCase(repo, time.Now())

		got, err := uc.DecreaseStock(context.Background(), uuid.New(), 1)
		if !errors.Is(err, repoErr) {
			t.Fatalf("expected %v, got %v", repoErr, err)
		}
		if !reflect.DeepEqual(got, domain.Product{}) {
			t.Errorf("expected zero value on error, got %+v", got)
		}
	})
}

func TestProductUseCase_ApplyUpdate(t *testing.T) {
	t.Run("forwards new price and stock to repository and returns updated product", func(t *testing.T) {
		productID := uuid.New()
		updated := domain.Product{ID: productID, Price: 99.5, AvailableStock: 7}

		var gotID uuid.UUID
		var gotPrice float64
		var gotStock int64
		repo := &fakeProductRepository{
			updatePriceAndStockFunc: func(
				ctx context.Context,
				id uuid.UUID,
				price float64,
				newStock int64,
			) (domain.Product, error) {
				gotID, gotPrice, gotStock = id, price, newStock
				return updated, nil
			},
		}
		uc, notifier := newTestProductUseCaseWithNotifier(repo, time.Now())

		got, err := uc.ApplyUpdate(context.Background(), ApplyProductUpdateInput{
			ProductID:      productID,
			Price:          99.5,
			AvailableStock: 7,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(notifier.notified) != 1 || !reflect.DeepEqual(notifier.notified[0], updated) {
			t.Errorf("expected updated product to be notified once, got %+v", notifier.notified)
		}
		if gotID != productID || gotPrice != 99.5 || gotStock != 7 {
			t.Errorf("forwarded (%v, %v, %v), want (%v, 99.5, 7)", gotID, gotPrice, gotStock, productID)
		}
		if !reflect.DeepEqual(got, updated) {
			t.Errorf("expected usecase to return repository result unchanged, got %+v, want %+v", got, updated)
		}
	})

	t.Run("allows zero stock", func(t *testing.T) {
		repo := &fakeProductRepository{
			updatePriceAndStockFunc: func(
				ctx context.Context,
				id uuid.UUID,
				price float64,
				newStock int64,
			) (domain.Product, error) {
				return domain.Product{ID: id, Price: price, AvailableStock: newStock}, nil
			},
		}
		uc := newTestProductUseCase(repo, time.Now())

		if _, err := uc.ApplyUpdate(context.Background(), ApplyProductUpdateInput{
			ProductID:      uuid.New(),
			Price:          1,
			AvailableStock: 0,
		}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	for _, tc := range []struct {
		name  string
		price float64
		stock int64
	}{
		{name: "rejects zero price", price: 0, stock: 1},
		{name: "rejects negative price", price: -1, stock: 1},
		{name: "rejects negative stock", price: 10, stock: -1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeProductRepository{}
			uc, notifier := newTestProductUseCaseWithNotifier(repo, time.Now())

			_, err := uc.ApplyUpdate(context.Background(), ApplyProductUpdateInput{
				ProductID:      uuid.New(),
				Price:          tc.price,
				AvailableStock: tc.stock,
			})
			if !errors.Is(err, apperrors.ErrInvalidProductUpdate) {
				t.Fatalf("expected ErrInvalidProductUpdate, got %v", err)
			}
			if len(notifier.notified) != 0 {
				t.Errorf("expected no notification for invalid update, got %d", len(notifier.notified))
			}
		})
	}

	t.Run("propagates ErrProductNotFound", func(t *testing.T) {
		repo := &fakeProductRepository{
			updatePriceAndStockFunc: func(
				ctx context.Context,
				id uuid.UUID,
				price float64,
				newStock int64,
			) (domain.Product, error) {
				return domain.Product{}, domain.ErrProductNotFound
			},
		}
		uc, notifier := newTestProductUseCaseWithNotifier(repo, time.Now())

		_, err := uc.ApplyUpdate(context.Background(), ApplyProductUpdateInput{
			ProductID:      uuid.New(),
			Price:          1,
			AvailableStock: 1,
		})
		if !errors.Is(err, domain.ErrProductNotFound) {
			t.Fatalf("expected ErrProductNotFound, got %v", err)
		}
		if len(notifier.notified) != 0 {
			t.Errorf("expected no notification when product is not found, got %d", len(notifier.notified))
		}
	})
}

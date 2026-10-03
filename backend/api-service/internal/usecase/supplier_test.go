package usecase

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"uuid"

	"github.com/andrey1af/shop-api/backend/api-service/internal/domain"
)

type fakeSupplierRepository struct {
	createFunc        func(ctx context.Context, supplier domain.Supplier) (domain.Supplier, error)
	deleteFunc        func(ctx context.Context, supplierID uuid.UUID) error
	getFunc           func(ctx context.Context, supplierID uuid.UUID) (domain.Supplier, error)
	listFunc          func(ctx context.Context) ([]domain.Supplier, error)
	updateAddressFunc func(ctx context.Context, supplierID uuid.UUID, address domain.Address) (domain.Supplier, error)

	gotCreateSupplier domain.Supplier
	gotDeleteID       uuid.UUID
	gotGetID          uuid.UUID
	gotUpdateID       uuid.UUID
	gotUpdateAddress  domain.Address
	deleteCalled      bool
}

func (f *fakeSupplierRepository) Create(ctx context.Context, supplier domain.Supplier) (domain.Supplier, error) {
	f.gotCreateSupplier = supplier
	return f.createFunc(ctx, supplier)
}

func (f *fakeSupplierRepository) Delete(ctx context.Context, supplierID uuid.UUID) error {
	f.gotDeleteID = supplierID
	f.deleteCalled = true
	return f.deleteFunc(ctx, supplierID)
}

func (f *fakeSupplierRepository) Get(ctx context.Context, supplierID uuid.UUID) (domain.Supplier, error) {
	f.gotGetID = supplierID
	return f.getFunc(ctx, supplierID)
}

func (f *fakeSupplierRepository) List(ctx context.Context) ([]domain.Supplier, error) {
	return f.listFunc(ctx)
}

func (f *fakeSupplierRepository) UpdateAddress(
	ctx context.Context,
	supplierID uuid.UUID,
	address domain.Address,
) (domain.Supplier, error) {
	f.gotUpdateID = supplierID
	f.gotUpdateAddress = address
	return f.updateAddressFunc(ctx, supplierID, address)
}

func newTestSupplierUseCase(repo SupplierRepository) *SupplierUseCase {
	return NewSupplierUseCase(newTestLogger(), repo, &fakeProductRepository{
		existsBySupplierIDFunc: func(ctx context.Context, supplierID uuid.UUID) (bool, error) {
			return false, nil
		},
	})
}

func TestSupplierUseCase_Create(t *testing.T) {
	t.Run("maps input and generates ids", func(t *testing.T) {
		repo := &fakeSupplierRepository{
			createFunc: func(ctx context.Context, supplier domain.Supplier) (domain.Supplier, error) {
				return supplier, nil
			},
		}
		uc := newTestSupplierUseCase(repo)

		in := CreateSupplierInput{
			Name:        "Acme Corp",
			PhoneNumber: "+1234567890",
			Address: CreateAddressInput{
				Country: "US",
				City:    "NYC",
				Street:  "5th Ave",
			},
		}

		got, err := uc.Create(context.Background(), in)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if got.ID == uuid.Nil() {
			t.Error("expected generated supplier ID, got zero value")
		}
		if got.Address.ID == uuid.Nil() {
			t.Error("expected generated address ID, got zero value")
		}
		if got.ID == got.Address.ID {
			t.Error("supplier ID and address ID must not be equal")
		}
		if got.Name != in.Name || got.PhoneNumber != in.PhoneNumber {
			t.Errorf("name/phone not mapped correctly: got %+v", got)
		}
		if got.Address.Country != in.Address.Country ||
			got.Address.City != in.Address.City ||
			got.Address.Street != in.Address.Street {
			t.Errorf("address fields not mapped: got %+v", got.Address)
		}
	})

	t.Run("generates distinct ids across calls", func(t *testing.T) {
		repo := &fakeSupplierRepository{
			createFunc: func(ctx context.Context, supplier domain.Supplier) (domain.Supplier, error) {
				return supplier, nil
			},
		}
		uc := newTestSupplierUseCase(repo)

		first, err := uc.Create(context.Background(), CreateSupplierInput{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		second, err := uc.Create(context.Background(), CreateSupplierInput{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if first.ID == second.ID {
			t.Error("expected different supplier IDs across calls")
		}
		if first.Address.ID == second.Address.ID {
			t.Error("expected different address IDs across calls")
		}
	})

	t.Run("propagates repository error and returns zero value", func(t *testing.T) {
		repoErr := errors.New("insert failed")
		repo := &fakeSupplierRepository{
			createFunc: func(ctx context.Context, supplier domain.Supplier) (domain.Supplier, error) {
				return domain.Supplier{}, repoErr
			},
		}
		uc := newTestSupplierUseCase(repo)

		got, err := uc.Create(context.Background(), CreateSupplierInput{})
		if !errors.Is(err, repoErr) {
			t.Fatalf("expected error %v, got %v", repoErr, err)
		}
		if !reflect.DeepEqual(got, domain.Supplier{}) {
			t.Errorf("expected zero value supplier on error, got %+v", got)
		}
	})

	t.Run("returns supplier exactly as returned by repository", func(t *testing.T) {
		stored := domain.Supplier{
			ID:          uuid.New(),
			Name:        "Stored",
			PhoneNumber: "+0000000000",
		}
		repo := &fakeSupplierRepository{
			createFunc: func(ctx context.Context, supplier domain.Supplier) (domain.Supplier, error) {
				return stored, nil
			},
		}
		uc := newTestSupplierUseCase(repo)

		got, err := uc.Create(context.Background(), CreateSupplierInput{Name: "Input"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !reflect.DeepEqual(got, stored) {
			t.Errorf("expected usecase to return repository result unchanged, got %+v, want %+v", got, stored)
		}
	})
}

func TestSupplierUseCase_Delete(t *testing.T) {
	t.Run("returns nil on success and forwards supplier id", func(t *testing.T) {
		supplierID := uuid.New()
		repo := &fakeSupplierRepository{
			deleteFunc: func(ctx context.Context, id uuid.UUID) error {
				return nil
			},
		}
		uc := newTestSupplierUseCase(repo)

		if err := uc.Delete(context.Background(), supplierID); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if repo.gotDeleteID != supplierID {
			t.Errorf("supplier id passed to repository = %v, want %v", repo.gotDeleteID, supplierID)
		}
	})

	t.Run("propagates ErrSupplierNotFound", func(t *testing.T) {
		repo := &fakeSupplierRepository{
			deleteFunc: func(ctx context.Context, id uuid.UUID) error {
				return domain.ErrSupplierNotFound
			},
		}
		uc := newTestSupplierUseCase(repo)

		err := uc.Delete(context.Background(), uuid.New())
		if !errors.Is(err, domain.ErrSupplierNotFound) {
			t.Fatalf("expected ErrSupplierNotFound, got %v", err)
		}
	})

	t.Run("checks product usage before deleting", func(t *testing.T) {
		supplierID := uuid.New()
		var gotSupplierID uuid.UUID
		productRepo := &fakeProductRepository{
			existsBySupplierIDFunc: func(ctx context.Context, id uuid.UUID) (bool, error) {
				gotSupplierID = id
				return false, nil
			},
		}
		repo := &fakeSupplierRepository{
			deleteFunc: func(ctx context.Context, id uuid.UUID) error {
				return nil
			},
		}
		uc := NewSupplierUseCase(newTestLogger(), repo, productRepo)

		if err := uc.Delete(context.Background(), supplierID); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gotSupplierID != supplierID {
			t.Errorf("supplier id checked incorrectly: got %v, want %v", gotSupplierID, supplierID)
		}
		if !repo.deleteCalled {
			t.Error("expected repository Delete to be called when supplier is not in use")
		}
	})

	t.Run("returns ErrSupplierInUse and skips Delete when products reference the supplier", func(t *testing.T) {
		productRepo := &fakeProductRepository{
			existsBySupplierIDFunc: func(ctx context.Context, id uuid.UUID) (bool, error) {
				return true, nil
			},
		}
		repo := &fakeSupplierRepository{}
		uc := NewSupplierUseCase(newTestLogger(), repo, productRepo)

		err := uc.Delete(context.Background(), uuid.New())
		if !errors.Is(err, domain.ErrSupplierInUse) {
			t.Fatalf("expected ErrSupplierInUse, got %v", err)
		}
		if repo.deleteCalled {
			t.Error("expected repository Delete not to be called when supplier is in use")
		}
	})

	t.Run("propagates error from product usage check without calling repository Delete", func(t *testing.T) {
		checkErr := errors.New("query failed")
		productRepo := &fakeProductRepository{
			existsBySupplierIDFunc: func(ctx context.Context, id uuid.UUID) (bool, error) {
				return false, checkErr
			},
		}
		repo := &fakeSupplierRepository{}
		uc := NewSupplierUseCase(newTestLogger(), repo, productRepo)

		err := uc.Delete(context.Background(), uuid.New())
		if !errors.Is(err, checkErr) {
			t.Fatalf("expected %v, got %v", checkErr, err)
		}
		if repo.deleteCalled {
			t.Error("expected repository Delete not to be called when usage check fails")
		}
	})

	t.Run("propagates generic repository error", func(t *testing.T) {
		repoErr := errors.New("connection lost")
		repo := &fakeSupplierRepository{
			deleteFunc: func(ctx context.Context, id uuid.UUID) error {
				return repoErr
			},
		}
		uc := newTestSupplierUseCase(repo)

		err := uc.Delete(context.Background(), uuid.New())
		if !errors.Is(err, repoErr) {
			t.Fatalf("expected %v, got %v", repoErr, err)
		}
	})
}

func TestSupplierUseCase_Get(t *testing.T) {
	t.Run("returns supplier from repository unchanged and forwards id", func(t *testing.T) {
		supplierID := uuid.New()
		want := domain.Supplier{ID: supplierID, Name: "Acme"}
		repo := &fakeSupplierRepository{
			getFunc: func(ctx context.Context, id uuid.UUID) (domain.Supplier, error) {
				return want, nil
			},
		}
		uc := newTestSupplierUseCase(repo)

		got, err := uc.Get(context.Background(), supplierID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %+v, want %+v", got, want)
		}
		if repo.gotGetID != supplierID {
			t.Errorf("supplier id forwarded incorrectly: got %v, want %v", repo.gotGetID, supplierID)
		}
	})

	t.Run("propagates ErrSupplierNotFound and zero value", func(t *testing.T) {
		repo := &fakeSupplierRepository{
			getFunc: func(ctx context.Context, id uuid.UUID) (domain.Supplier, error) {
				return domain.Supplier{}, domain.ErrSupplierNotFound
			},
		}
		uc := newTestSupplierUseCase(repo)

		got, err := uc.Get(context.Background(), uuid.New())
		if !errors.Is(err, domain.ErrSupplierNotFound) {
			t.Fatalf("expected ErrSupplierNotFound, got %v", err)
		}
		if !reflect.DeepEqual(got, domain.Supplier{}) {
			t.Errorf("expected zero value on error, got %+v", got)
		}
	})
}

func TestSupplierUseCase_List(t *testing.T) {
	t.Run("returns suppliers from repository unchanged", func(t *testing.T) {
		want := []domain.Supplier{{Name: "A"}, {Name: "B"}}
		repo := &fakeSupplierRepository{
			listFunc: func(ctx context.Context) ([]domain.Supplier, error) {
				return want, nil
			},
		}
		uc := newTestSupplierUseCase(repo)

		got, err := uc.List(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %+v, want %+v", got, want)
		}
	})

	t.Run("returns nil slice from repository as-is when no matches", func(t *testing.T) {
		repo := &fakeSupplierRepository{
			listFunc: func(ctx context.Context) ([]domain.Supplier, error) {
				return nil, nil
			},
		}
		uc := newTestSupplierUseCase(repo)

		got, err := uc.List(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != nil {
			t.Errorf("expected nil slice to be passed through, got %+v", got)
		}
	})

	t.Run("returns non-nil empty slice and error on repository failure", func(t *testing.T) {
		repoErr := errors.New("query failed")
		repo := &fakeSupplierRepository{
			listFunc: func(ctx context.Context) ([]domain.Supplier, error) {
				return nil, repoErr
			},
		}
		uc := newTestSupplierUseCase(repo)

		got, err := uc.List(context.Background())
		if !errors.Is(err, repoErr) {
			t.Fatalf("expected %v, got %v", repoErr, err)
		}
		if got == nil {
			t.Error("expected non-nil empty slice on error, got nil")
		}
		if len(got) != 0 {
			t.Errorf("expected empty slice, got %+v", got)
		}
	})
}

func TestSupplierUseCase_ChangeAddress(t *testing.T) {
	t.Run("maps input to zero-id address and forwards supplier id", func(t *testing.T) {
		supplierID := uuid.New()
		updated := domain.Supplier{
			ID: supplierID,
			Address: domain.Address{
				ID:      uuid.New(),
				Country: "US",
				City:    "NYC",
				Street:  "5th Ave",
			},
		}
		repo := &fakeSupplierRepository{
			updateAddressFunc: func(ctx context.Context, id uuid.UUID, address domain.Address) (domain.Supplier, error) {
				return updated, nil
			},
		}
		uc := newTestSupplierUseCase(repo)

		in := CreateAddressInput{Country: "US", City: "NYC", Street: "5th Ave"}
		got, err := uc.ChangeAddress(context.Background(), supplierID, in)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if repo.gotUpdateID != supplierID {
			t.Errorf("supplier id forwarded incorrectly: got %v, want %v", repo.gotUpdateID, supplierID)
		}
		if repo.gotUpdateAddress.ID != uuid.Nil() {
			t.Errorf("expected zero-value address ID sent to repository, got %v", repo.gotUpdateAddress.ID)
		}
		if repo.gotUpdateAddress.Country != in.Country ||
			repo.gotUpdateAddress.City != in.City ||
			repo.gotUpdateAddress.Street != in.Street {
			t.Errorf("address fields not mapped: got %+v", repo.gotUpdateAddress)
		}
		if !reflect.DeepEqual(got, updated) {
			t.Errorf("expected usecase to return repository result unchanged, got %+v, want %+v", got, updated)
		}
	})

	t.Run("propagates ErrSupplierNotFound", func(t *testing.T) {
		repo := &fakeSupplierRepository{
			updateAddressFunc: func(ctx context.Context, id uuid.UUID, address domain.Address) (domain.Supplier, error) {
				return domain.Supplier{}, domain.ErrSupplierNotFound
			},
		}
		uc := newTestSupplierUseCase(repo)

		_, err := uc.ChangeAddress(context.Background(), uuid.New(), CreateAddressInput{})
		if !errors.Is(err, domain.ErrSupplierNotFound) {
			t.Fatalf("expected ErrSupplierNotFound, got %v", err)
		}
	})

	t.Run("propagates generic repository error and returns zero value", func(t *testing.T) {
		repoErr := errors.New("update failed")
		repo := &fakeSupplierRepository{
			updateAddressFunc: func(ctx context.Context, id uuid.UUID, address domain.Address) (domain.Supplier, error) {
				return domain.Supplier{}, repoErr
			},
		}
		uc := newTestSupplierUseCase(repo)

		got, err := uc.ChangeAddress(context.Background(), uuid.New(), CreateAddressInput{})
		if !errors.Is(err, repoErr) {
			t.Fatalf("expected %v, got %v", repoErr, err)
		}
		if !reflect.DeepEqual(got, domain.Supplier{}) {
			t.Errorf("expected zero value on error, got %+v", got)
		}
	})
}

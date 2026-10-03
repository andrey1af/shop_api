package usecase

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
	"uuid"

	"github.com/andrey1af/shop-api/backend/api-service/internal/domain"
)

type fakeClientRepository struct {
	createFunc        func(ctx context.Context, client domain.Client) (domain.Client, error)
	deleteFunc        func(ctx context.Context, clientID uuid.UUID) error
	findFunc          func(ctx context.Context, name, surname string) ([]domain.Client, error)
	listFunc          func(ctx context.Context, p domain.ListParams) ([]domain.Client, error)
	updateAddressFunc func(ctx context.Context, clientID uuid.UUID, address domain.Address) (domain.Client, error)

	gotCreateClient   domain.Client
	gotDeleteClientID uuid.UUID
	gotFindName       string
	gotFindSurname    string
	gotListParams     domain.ListParams
	gotUpdateClientID uuid.UUID
	gotUpdateAddress  domain.Address
}

func (f *fakeClientRepository) Create(ctx context.Context, client domain.Client) (domain.Client, error) {
	f.gotCreateClient = client
	return f.createFunc(ctx, client)
}

func (f *fakeClientRepository) Delete(ctx context.Context, clientID uuid.UUID) error {
	f.gotDeleteClientID = clientID
	return f.deleteFunc(ctx, clientID)
}

func (f *fakeClientRepository) FindByNameAndSurname(
	ctx context.Context,
	name, surname string,
) ([]domain.Client, error) {
	f.gotFindName = name
	f.gotFindSurname = surname
	return f.findFunc(ctx, name, surname)
}

func (f *fakeClientRepository) List(ctx context.Context, p domain.ListParams) ([]domain.Client, error) {
	f.gotListParams = p
	return f.listFunc(ctx, p)
}

func (f *fakeClientRepository) UpdateAddress(
	ctx context.Context,
	clientID uuid.UUID,
	address domain.Address,
) (domain.Client, error) {
	f.gotUpdateClientID = clientID
	f.gotUpdateAddress = address
	return f.updateAddressFunc(ctx, clientID, address)
}

func newTestUseCase(repo ClientRepository, now time.Time) *ClientUseCase {
	uc := NewClientUseCase(newTestLogger(), repo)
	uc.now = func() time.Time { return now }
	return uc
}

func TestClientUseCase_Create(t *testing.T) {
	t.Run("maps input, generates ids and uses injected clock", func(t *testing.T) {
		fixedNow := time.Date(2026, 1, 2, 10, 0, 0, 0, time.UTC)
		birthday := time.Date(1990, 5, 4, 0, 0, 0, 0, time.UTC)

		repo := &fakeClientRepository{
			createFunc: func(ctx context.Context, client domain.Client) (domain.Client, error) {
				return client, nil
			},
		}
		uc := newTestUseCase(repo, fixedNow)

		in := CreateClientInput{
			Name:     "John",
			Surname:  "Doe",
			Birthday: birthday,
			Gender:   domain.GenderMale,
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
			t.Error("expected generated client ID, got zero value")
		}
		if got.Address.ID == uuid.Nil() {
			t.Error("expected generated address ID, got zero value")
		}
		if got.ID == got.Address.ID {
			t.Error("client ID and address ID must not be equal")
		}
		if got.Name != in.Name || got.Surname != in.Surname {
			t.Errorf("name/surname not mapped correctly: got %+v", got)
		}
		if !got.Birthday.Equal(in.Birthday) {
			t.Errorf("birthday not mapped: got %v, want %v", got.Birthday, in.Birthday)
		}
		if got.Gender != in.Gender {
			t.Errorf("gender not mapped: got %v, want %v", got.Gender, in.Gender)
		}
		if !got.RegistrationTime.Equal(fixedNow) {
			t.Errorf("registration time = %v, want injected clock value %v", got.RegistrationTime, fixedNow)
		}
		if got.Address.Country != in.Address.Country ||
			got.Address.City != in.Address.City ||
			got.Address.Street != in.Address.Street {
			t.Errorf("address fields not mapped: got %+v", got.Address)
		}
	})

	t.Run("generates distinct ids across calls", func(t *testing.T) {
		repo := &fakeClientRepository{
			createFunc: func(ctx context.Context, client domain.Client) (domain.Client, error) {
				return client, nil
			},
		}
		uc := newTestUseCase(repo, time.Now())

		first, err := uc.Create(context.Background(), CreateClientInput{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		second, err := uc.Create(context.Background(), CreateClientInput{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if first.ID == second.ID {
			t.Error("expected different client IDs across calls")
		}
		if first.Address.ID == second.Address.ID {
			t.Error("expected different address IDs across calls")
		}
	})

	t.Run("propagates repository error and returns zero value", func(t *testing.T) {
		repoErr := errors.New("insert failed")
		repo := &fakeClientRepository{
			createFunc: func(ctx context.Context, client domain.Client) (domain.Client, error) {
				return domain.Client{}, repoErr
			},
		}
		uc := newTestUseCase(repo, time.Now())

		got, err := uc.Create(context.Background(), CreateClientInput{})
		if !errors.Is(err, repoErr) {
			t.Fatalf("expected error %v, got %v", repoErr, err)
		}
		if !reflect.DeepEqual(got, domain.Client{}) {
			t.Errorf("expected zero value client on error, got %+v", got)
		}
	})

	t.Run("returns client exactly as returned by repository", func(t *testing.T) {
		stored := domain.Client{
			ID:      uuid.New(),
			Name:    "Stored",
			Surname: "Value",
		}
		repo := &fakeClientRepository{
			createFunc: func(ctx context.Context, client domain.Client) (domain.Client, error) {
				return stored, nil
			},
		}
		uc := newTestUseCase(repo, time.Now())

		got, err := uc.Create(context.Background(), CreateClientInput{Name: "Input", Surname: "Value"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !reflect.DeepEqual(got, stored) {
			t.Errorf("expected usecase to return repository result unchanged, got %+v, want %+v", got, stored)
		}
	})
}

func TestClientUseCase_Delete(t *testing.T) {
	t.Run("returns nil on success and forwards client id", func(t *testing.T) {
		clientID := uuid.New()
		repo := &fakeClientRepository{
			deleteFunc: func(ctx context.Context, id uuid.UUID) error {
				return nil
			},
		}
		uc := newTestUseCase(repo, time.Now())

		if err := uc.Delete(context.Background(), clientID); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if repo.gotDeleteClientID != clientID {
			t.Errorf("client id passed to repository = %v, want %v", repo.gotDeleteClientID, clientID)
		}
	})

	t.Run("propagates ErrClientNotFound", func(t *testing.T) {
		repo := &fakeClientRepository{
			deleteFunc: func(ctx context.Context, id uuid.UUID) error {
				return domain.ErrClientNotFound
			},
		}
		uc := newTestUseCase(repo, time.Now())

		err := uc.Delete(context.Background(), uuid.New())
		if !errors.Is(err, domain.ErrClientNotFound) {
			t.Fatalf("expected ErrClientNotFound, got %v", err)
		}
	})

	t.Run("propagates generic repository error", func(t *testing.T) {
		repoErr := errors.New("connection lost")
		repo := &fakeClientRepository{
			deleteFunc: func(ctx context.Context, id uuid.UUID) error {
				return repoErr
			},
		}
		uc := newTestUseCase(repo, time.Now())

		err := uc.Delete(context.Background(), uuid.New())
		if !errors.Is(err, repoErr) {
			t.Fatalf("expected %v, got %v", repoErr, err)
		}
	})
}

func TestClientUseCase_FindByNameAndSurname(t *testing.T) {
	t.Run("returns clients from repository unchanged", func(t *testing.T) {
		want := []domain.Client{{Name: "A"}, {Name: "B"}}
		repo := &fakeClientRepository{
			findFunc: func(ctx context.Context, name, surname string) ([]domain.Client, error) {
				return want, nil
			},
		}
		uc := newTestUseCase(repo, time.Now())

		got, err := uc.FindByNameAndSurname(context.Background(), "John", "Doe")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %+v, want %+v", got, want)
		}
		if repo.gotFindName != "John" || repo.gotFindSurname != "Doe" {
			t.Errorf("name/surname forwarded incorrectly: got (%q, %q)", repo.gotFindName, repo.gotFindSurname)
		}
	})

	t.Run("returns nil slice from repository as-is when no matches", func(t *testing.T) {
		repo := &fakeClientRepository{
			findFunc: func(ctx context.Context, name, surname string) ([]domain.Client, error) {
				return nil, nil
			},
		}
		uc := newTestUseCase(repo, time.Now())

		got, err := uc.FindByNameAndSurname(context.Background(), "John", "Doe")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != nil {
			t.Errorf("expected nil slice to be passed through, got %+v", got)
		}
	})

	t.Run("returns non-nil empty slice and error on repository failure", func(t *testing.T) {
		repoErr := errors.New("query failed")
		repo := &fakeClientRepository{
			findFunc: func(ctx context.Context, name, surname string) ([]domain.Client, error) {
				return nil, repoErr
			},
		}
		uc := newTestUseCase(repo, time.Now())

		got, err := uc.FindByNameAndSurname(context.Background(), "John", "Doe")
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

func TestClientUseCase_List(t *testing.T) {
	t.Run("forwards params and returns repository result unchanged", func(t *testing.T) {
		limit, offset := 10, 5
		params := domain.ListParams{Limit: &limit, Offset: &offset}
		want := []domain.Client{{Name: "A"}}
		repo := &fakeClientRepository{
			listFunc: func(ctx context.Context, p domain.ListParams) ([]domain.Client, error) {
				return want, nil
			},
		}
		uc := newTestUseCase(repo, time.Now())

		got, err := uc.List(context.Background(), params)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %+v, want %+v", got, want)
		}
		if repo.gotListParams.Limit != params.Limit || repo.gotListParams.Offset != params.Offset {
			t.Errorf("params not forwarded unchanged: got %+v", repo.gotListParams)
		}
	})

	t.Run("forwards nil limit and offset as-is", func(t *testing.T) {
		var captured domain.ListParams
		repo := &fakeClientRepository{
			listFunc: func(ctx context.Context, p domain.ListParams) ([]domain.Client, error) {
				captured = p
				return nil, nil
			},
		}
		uc := newTestUseCase(repo, time.Now())

		if _, err := uc.List(context.Background(), domain.ListParams{}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if captured.Limit != nil || captured.Offset != nil {
			t.Errorf("expected nil Limit/Offset to be forwarded as-is, got %+v", captured)
		}
	})

	t.Run("returns non-nil empty slice and error on repository failure", func(t *testing.T) {
		repoErr := errors.New("query failed")
		repo := &fakeClientRepository{
			listFunc: func(ctx context.Context, p domain.ListParams) ([]domain.Client, error) {
				return nil, repoErr
			},
		}
		uc := newTestUseCase(repo, time.Now())

		got, err := uc.List(context.Background(), domain.ListParams{})
		if !errors.Is(err, repoErr) {
			t.Fatalf("expected %v, got %v", repoErr, err)
		}
		if got == nil || len(got) != 0 {
			t.Errorf("expected non-nil empty slice on error, got %+v", got)
		}
	})
}

func TestClientUseCase_ChangeAddress(t *testing.T) {
	t.Run("maps input to zero-id address and forwards client id", func(t *testing.T) {
		clientID := uuid.New()
		updated := domain.Client{
			ID: clientID,
			Address: domain.Address{
				ID:      uuid.New(),
				Country: "US",
				City:    "NYC",
				Street:  "5th Ave",
			},
		}
		repo := &fakeClientRepository{
			updateAddressFunc: func(ctx context.Context, id uuid.UUID, address domain.Address) (domain.Client, error) {
				return updated, nil
			},
		}
		uc := newTestUseCase(repo, time.Now())

		in := CreateAddressInput{Country: "US", City: "NYC", Street: "5th Ave"}
		got, err := uc.ChangeAddress(context.Background(), clientID, in)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if repo.gotUpdateClientID != clientID {
			t.Errorf("client id forwarded incorrectly: got %v, want %v", repo.gotUpdateClientID, clientID)
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

	t.Run("propagates ErrClientNotFound", func(t *testing.T) {
		repo := &fakeClientRepository{
			updateAddressFunc: func(ctx context.Context, id uuid.UUID, address domain.Address) (domain.Client, error) {
				return domain.Client{}, domain.ErrClientNotFound
			},
		}
		uc := newTestUseCase(repo, time.Now())

		_, err := uc.ChangeAddress(context.Background(), uuid.New(), CreateAddressInput{})
		if !errors.Is(err, domain.ErrClientNotFound) {
			t.Fatalf("expected ErrClientNotFound, got %v", err)
		}
	})

	t.Run("propagates generic repository error and returns zero value", func(t *testing.T) {
		repoErr := errors.New("update failed")
		repo := &fakeClientRepository{
			updateAddressFunc: func(ctx context.Context, id uuid.UUID, address domain.Address) (domain.Client, error) {
				return domain.Client{}, repoErr
			},
		}
		uc := newTestUseCase(repo, time.Now())

		got, err := uc.ChangeAddress(context.Background(), uuid.New(), CreateAddressInput{})
		if !errors.Is(err, repoErr) {
			t.Fatalf("expected %v, got %v", repoErr, err)
		}
		if !reflect.DeepEqual(got, domain.Client{}) {
			t.Errorf("expected zero value on error, got %+v", got)
		}
	})
}

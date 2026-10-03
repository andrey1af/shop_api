package usecase

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"uuid"

	"github.com/andrey1af/shop-api/backend/api-service/internal/domain"
)

type fakeImageRepository struct {
	createFunc         func(ctx context.Context, image domain.Image) (domain.Image, error)
	getByIDFunc        func(ctx context.Context, imageID uuid.UUID) (domain.Image, error)
	getByProductIDFunc func(ctx context.Context, productID uuid.UUID) (domain.Image, error)
	replaceFunc        func(ctx context.Context, imageID uuid.UUID, data []byte, contentType string) (domain.Image, error)
	deleteFunc         func(ctx context.Context, imageID uuid.UUID) error

	gotCreateImage    domain.Image
	gotGetByID        uuid.UUID
	gotGetByProductID uuid.UUID
	gotReplaceID      uuid.UUID
	gotReplaceData    []byte
	gotReplaceType    string
	gotDeleteID       uuid.UUID
	createCalled      bool
}

func (f *fakeImageRepository) Create(ctx context.Context, image domain.Image) (domain.Image, error) {
	f.gotCreateImage = image
	f.createCalled = true
	return f.createFunc(ctx, image)
}

func (f *fakeImageRepository) GetByID(ctx context.Context, imageID uuid.UUID) (domain.Image, error) {
	f.gotGetByID = imageID
	return f.getByIDFunc(ctx, imageID)
}

func (f *fakeImageRepository) GetByProductID(ctx context.Context, productID uuid.UUID) (domain.Image, error) {
	f.gotGetByProductID = productID
	return f.getByProductIDFunc(ctx, productID)
}

func (f *fakeImageRepository) Replace(
	ctx context.Context,
	imageID uuid.UUID,
	data []byte,
	contentType string,
) (domain.Image, error) {
	f.gotReplaceID = imageID
	f.gotReplaceData = data
	f.gotReplaceType = contentType
	return f.replaceFunc(ctx, imageID, data, contentType)
}

func (f *fakeImageRepository) Delete(ctx context.Context, imageID uuid.UUID) error {
	f.gotDeleteID = imageID
	return f.deleteFunc(ctx, imageID)
}

func newTestImageUseCase(repo ImageRepository) *ImageUseCase {
	return NewImageUseCase(newTestLogger(), repo, &fakeProductRepository{
		getFunc: func(ctx context.Context, id uuid.UUID) (domain.Product, error) {
			return domain.Product{ID: id}, nil
		},
	})
}

func TestImageUseCase_Create(t *testing.T) {
	t.Run("maps input and generates id", func(t *testing.T) {
		productID := uuid.New()
		repo := &fakeImageRepository{
			createFunc: func(ctx context.Context, image domain.Image) (domain.Image, error) {
				return image, nil
			},
		}
		uc := newTestImageUseCase(repo)

		in := CreateImageInput{
			ProductID:   productID,
			Data:        []byte{0x1, 0x2, 0x3},
			ContentType: "image/png",
		}

		got, err := uc.Create(context.Background(), in)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if got.ID == uuid.Nil() {
			t.Error("expected generated image ID, got zero value")
		}
		if got.ProductID != in.ProductID {
			t.Errorf("product id not mapped: got %v, want %v", got.ProductID, in.ProductID)
		}
		if !reflect.DeepEqual(got.Data, in.Data) {
			t.Errorf("data not mapped: got %v, want %v", got.Data, in.Data)
		}
		if got.ContentType != in.ContentType {
			t.Errorf("content type not mapped: got %v, want %v", got.ContentType, in.ContentType)
		}
	})

	t.Run("generates distinct ids across calls", func(t *testing.T) {
		repo := &fakeImageRepository{
			createFunc: func(ctx context.Context, image domain.Image) (domain.Image, error) {
				return image, nil
			},
		}
		uc := newTestImageUseCase(repo)

		first, err := uc.Create(context.Background(), CreateImageInput{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		second, err := uc.Create(context.Background(), CreateImageInput{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if first.ID == second.ID {
			t.Error("expected different image IDs across calls")
		}
	})

	t.Run("propagates ErrProductNotFound from product check without calling repository Create", func(t *testing.T) {
		productRepo := &fakeProductRepository{
			getFunc: func(ctx context.Context, id uuid.UUID) (domain.Product, error) {
				return domain.Product{}, domain.ErrProductNotFound
			},
		}
		repo := &fakeImageRepository{}
		uc := NewImageUseCase(newTestLogger(), repo, productRepo)

		got, err := uc.Create(context.Background(), CreateImageInput{})
		if !errors.Is(err, domain.ErrProductNotFound) {
			t.Fatalf("expected ErrProductNotFound, got %v", err)
		}
		if !reflect.DeepEqual(got, domain.Image{}) {
			t.Errorf("expected zero value on error, got %+v", got)
		}
		if repo.createCalled {
			t.Error("expected repository Create not to be called when product is not found")
		}
	})

	t.Run("returns ErrImageAlreadyExists and skips Create when product already has an image", func(t *testing.T) {
		existingImageID := uuid.New()
		productRepo := &fakeProductRepository{
			getFunc: func(ctx context.Context, id uuid.UUID) (domain.Product, error) {
				return domain.Product{ID: id, ImageID: &existingImageID}, nil
			},
		}
		repo := &fakeImageRepository{}
		uc := NewImageUseCase(newTestLogger(), repo, productRepo)

		_, err := uc.Create(context.Background(), CreateImageInput{})
		if !errors.Is(err, domain.ErrImageAlreadyExists) {
			t.Fatalf("expected ErrImageAlreadyExists, got %v", err)
		}
		if repo.createCalled {
			t.Error("expected repository Create not to be called when product already has an image")
		}
	})

	t.Run("propagates generic repository error and returns zero value", func(t *testing.T) {
		repoErr := errors.New("insert failed")
		repo := &fakeImageRepository{
			createFunc: func(ctx context.Context, image domain.Image) (domain.Image, error) {
				return domain.Image{}, repoErr
			},
		}
		uc := newTestImageUseCase(repo)

		got, err := uc.Create(context.Background(), CreateImageInput{})
		if !errors.Is(err, repoErr) {
			t.Fatalf("expected %v, got %v", repoErr, err)
		}
		if !reflect.DeepEqual(got, domain.Image{}) {
			t.Errorf("expected zero value on error, got %+v", got)
		}
	})

	t.Run("returns image exactly as returned by repository", func(t *testing.T) {
		stored := domain.Image{ID: uuid.New(), ProductID: uuid.New(), Data: []byte{0xAA}}
		repo := &fakeImageRepository{
			createFunc: func(ctx context.Context, image domain.Image) (domain.Image, error) {
				return stored, nil
			},
		}
		uc := newTestImageUseCase(repo)

		got, err := uc.Create(context.Background(), CreateImageInput{Data: []byte{0xBB}})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !reflect.DeepEqual(got, stored) {
			t.Errorf("expected usecase to return repository result unchanged, got %+v, want %+v", got, stored)
		}
	})
}

func TestImageUseCase_GetByID(t *testing.T) {
	t.Run("returns image from repository unchanged and forwards id", func(t *testing.T) {
		imageID := uuid.New()
		want := domain.Image{ID: imageID, Data: []byte{0x1}}
		repo := &fakeImageRepository{
			getByIDFunc: func(ctx context.Context, id uuid.UUID) (domain.Image, error) {
				return want, nil
			},
		}
		uc := newTestImageUseCase(repo)

		got, err := uc.GetByID(context.Background(), imageID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %+v, want %+v", got, want)
		}
		if repo.gotGetByID != imageID {
			t.Errorf("image id forwarded incorrectly: got %v, want %v", repo.gotGetByID, imageID)
		}
	})

	t.Run("propagates ErrImageNotFound and zero value", func(t *testing.T) {
		repo := &fakeImageRepository{
			getByIDFunc: func(ctx context.Context, id uuid.UUID) (domain.Image, error) {
				return domain.Image{}, domain.ErrImageNotFound
			},
		}
		uc := newTestImageUseCase(repo)

		got, err := uc.GetByID(context.Background(), uuid.New())
		if !errors.Is(err, domain.ErrImageNotFound) {
			t.Fatalf("expected ErrImageNotFound, got %v", err)
		}
		if !reflect.DeepEqual(got, domain.Image{}) {
			t.Errorf("expected zero value on error, got %+v", got)
		}
	})
}

func TestImageUseCase_GetByProductID(t *testing.T) {
	t.Run("returns image from repository unchanged and forwards product id", func(t *testing.T) {
		productID := uuid.New()
		want := domain.Image{ProductID: productID, Data: []byte{0x2}}
		repo := &fakeImageRepository{
			getByProductIDFunc: func(ctx context.Context, id uuid.UUID) (domain.Image, error) {
				return want, nil
			},
		}
		uc := newTestImageUseCase(repo)

		got, err := uc.GetByProductID(context.Background(), productID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %+v, want %+v", got, want)
		}
		if repo.gotGetByProductID != productID {
			t.Errorf("product id forwarded incorrectly: got %v, want %v", repo.gotGetByProductID, productID)
		}
	})

	t.Run("propagates ErrImageNotFound", func(t *testing.T) {
		repo := &fakeImageRepository{
			getByProductIDFunc: func(ctx context.Context, id uuid.UUID) (domain.Image, error) {
				return domain.Image{}, domain.ErrImageNotFound
			},
		}
		uc := newTestImageUseCase(repo)

		_, err := uc.GetByProductID(context.Background(), uuid.New())
		if !errors.Is(err, domain.ErrImageNotFound) {
			t.Fatalf("expected ErrImageNotFound, got %v", err)
		}
	})
}

func TestImageUseCase_Replace(t *testing.T) {
	t.Run("forwards image id, data and content type", func(t *testing.T) {
		imageID := uuid.New()
		data := []byte{0x9, 0x8}
		contentType := "image/jpeg"
		updated := domain.Image{ID: imageID, Data: data, ContentType: contentType}
		repo := &fakeImageRepository{
			replaceFunc: func(ctx context.Context, id uuid.UUID, d []byte, ct string) (domain.Image, error) {
				return updated, nil
			},
		}
		uc := newTestImageUseCase(repo)

		got, err := uc.Replace(context.Background(), imageID, data, contentType)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if repo.gotReplaceID != imageID {
			t.Errorf("image id forwarded incorrectly: got %v, want %v", repo.gotReplaceID, imageID)
		}
		if !reflect.DeepEqual(repo.gotReplaceData, data) {
			t.Errorf("data forwarded incorrectly: got %v, want %v", repo.gotReplaceData, data)
		}
		if repo.gotReplaceType != contentType {
			t.Errorf("content type forwarded incorrectly: got %v, want %v", repo.gotReplaceType, contentType)
		}
		if !reflect.DeepEqual(got, updated) {
			t.Errorf("expected usecase to return repository result unchanged, got %+v, want %+v", got, updated)
		}
	})

	t.Run("propagates ErrImageNotFound and returns zero value", func(t *testing.T) {
		repo := &fakeImageRepository{
			replaceFunc: func(ctx context.Context, id uuid.UUID, d []byte, ct string) (domain.Image, error) {
				return domain.Image{}, domain.ErrImageNotFound
			},
		}
		uc := newTestImageUseCase(repo)

		got, err := uc.Replace(context.Background(), uuid.New(), nil, "")
		if !errors.Is(err, domain.ErrImageNotFound) {
			t.Fatalf("expected ErrImageNotFound, got %v", err)
		}
		if !reflect.DeepEqual(got, domain.Image{}) {
			t.Errorf("expected zero value on error, got %+v", got)
		}
	})

	t.Run("propagates generic repository error", func(t *testing.T) {
		repoErr := errors.New("update failed")
		repo := &fakeImageRepository{
			replaceFunc: func(ctx context.Context, id uuid.UUID, d []byte, ct string) (domain.Image, error) {
				return domain.Image{}, repoErr
			},
		}
		uc := newTestImageUseCase(repo)

		_, err := uc.Replace(context.Background(), uuid.New(), nil, "")
		if !errors.Is(err, repoErr) {
			t.Fatalf("expected %v, got %v", repoErr, err)
		}
	})
}

func TestImageUseCase_Delete(t *testing.T) {
	t.Run("returns nil on success and forwards image id", func(t *testing.T) {
		imageID := uuid.New()
		repo := &fakeImageRepository{
			deleteFunc: func(ctx context.Context, id uuid.UUID) error {
				return nil
			},
		}
		uc := newTestImageUseCase(repo)

		if err := uc.Delete(context.Background(), imageID); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if repo.gotDeleteID != imageID {
			t.Errorf("image id passed to repository = %v, want %v", repo.gotDeleteID, imageID)
		}
	})

	t.Run("propagates ErrImageNotFound", func(t *testing.T) {
		repo := &fakeImageRepository{
			deleteFunc: func(ctx context.Context, id uuid.UUID) error {
				return domain.ErrImageNotFound
			},
		}
		uc := newTestImageUseCase(repo)

		err := uc.Delete(context.Background(), uuid.New())
		if !errors.Is(err, domain.ErrImageNotFound) {
			t.Fatalf("expected ErrImageNotFound, got %v", err)
		}
	})

	t.Run("propagates generic repository error", func(t *testing.T) {
		repoErr := errors.New("connection lost")
		repo := &fakeImageRepository{
			deleteFunc: func(ctx context.Context, id uuid.UUID) error {
				return repoErr
			},
		}
		uc := newTestImageUseCase(repo)

		err := uc.Delete(context.Background(), uuid.New())
		if !errors.Is(err, repoErr) {
			t.Fatalf("expected %v, got %v", repoErr, err)
		}
	})
}

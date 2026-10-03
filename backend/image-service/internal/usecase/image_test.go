package usecase

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"uuid"

	"github.com/andrey1af/shop-api/backend/image-service/internal/domain"
	apperrors "github.com/andrey1af/shop-api/backend/image-service/internal/errors"
)

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type memoryImageStore struct {
	mu     sync.Mutex
	images map[uuid.UUID]domain.Image
	err    error
}

func newMemoryImageStore() *memoryImageStore {
	return &memoryImageStore{images: map[uuid.UUID]domain.Image{}}
}

func (m *memoryImageStore) Create(_ context.Context, image domain.Image) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return m.err
	}
	if _, ok := m.images[image.ID]; ok {
		return apperrors.ErrImageAlreadyExists
	}
	m.images[image.ID] = image
	return nil
}

func (m *memoryImageStore) Get(_ context.Context, id uuid.UUID) (domain.Image, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return domain.Image{}, m.err
	}
	image, ok := m.images[id]
	if !ok {
		return domain.Image{}, apperrors.ErrImageNotFound
	}
	return image, nil
}

func (m *memoryImageStore) Update(_ context.Context, image domain.Image) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return m.err
	}
	if _, ok := m.images[image.ID]; !ok {
		return apperrors.ErrImageNotFound
	}
	m.images[image.ID] = image
	return nil
}

func (m *memoryImageStore) Delete(_ context.Context, id uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return m.err
	}
	if _, ok := m.images[id]; !ok {
		return apperrors.ErrImageNotFound
	}
	delete(m.images, id)
	return nil
}

func TestImageUseCase_CreateAndGet(t *testing.T) {
	store := newMemoryImageStore()
	uc := NewImageUseCase(newTestLogger(), store)
	id := uuid.New()

	if err := uc.Create(context.Background(), CreateImageInput{
		ID:          id,
		Data:        []byte{1, 2, 3},
		ContentType: "image/png",
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	got, err := uc.Get(context.Background(), id)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.ID != id || !bytes.Equal(got.Data, []byte{1, 2, 3}) || got.ContentType != "image/png" {
		t.Errorf("Get() = %+v, want id %s, data [1 2 3], content type image/png", got, id)
	}
}

func TestImageUseCase_Create(t *testing.T) {
	t.Run("rejects empty data", func(t *testing.T) {
		store := newMemoryImageStore()
		uc := NewImageUseCase(newTestLogger(), store)

		err := uc.Create(context.Background(), CreateImageInput{ID: uuid.New(), ContentType: "image/png"})
		if !errors.Is(err, apperrors.ErrEmptyImageData) {
			t.Fatalf("Create() error = %v, want %v", err, apperrors.ErrEmptyImageData)
		}
		if len(store.images) != 0 {
			t.Errorf("store has %d images, want 0", len(store.images))
		}
	})

	t.Run("defaults content type", func(t *testing.T) {
		store := newMemoryImageStore()
		uc := NewImageUseCase(newTestLogger(), store)
		id := uuid.New()

		if err := uc.Create(context.Background(), CreateImageInput{ID: id, Data: []byte{1}}); err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		if got := store.images[id].ContentType; got != defaultContentType {
			t.Errorf("content type = %q, want %q", got, defaultContentType)
		}
	})

	t.Run("returns already exists", func(t *testing.T) {
		store := newMemoryImageStore()
		uc := NewImageUseCase(newTestLogger(), store)
		id := uuid.New()
		in := CreateImageInput{ID: id, Data: []byte{1}}

		if err := uc.Create(context.Background(), in); err != nil {
			t.Fatalf("first Create() error = %v", err)
		}
		if err := uc.Create(context.Background(), in); !errors.Is(err, apperrors.ErrImageAlreadyExists) {
			t.Fatalf("second Create() error = %v, want %v", err, apperrors.ErrImageAlreadyExists)
		}
	})

	t.Run("propagates repository error", func(t *testing.T) {
		repoErr := errors.New("db down")
		store := newMemoryImageStore()
		store.err = repoErr
		uc := NewImageUseCase(newTestLogger(), store)

		if err := uc.Create(context.Background(), CreateImageInput{
			ID:   uuid.New(),
			Data: []byte{1},
		}); !errors.Is(err, repoErr) {
			t.Fatalf("Create() error = %v, want %v", err, repoErr)
		}
	})
}

func TestImageUseCase_Get_NotFound(t *testing.T) {
	uc := NewImageUseCase(newTestLogger(), newMemoryImageStore())

	if _, err := uc.Get(context.Background(), uuid.New()); !errors.Is(err, apperrors.ErrImageNotFound) {
		t.Fatalf("Get() error = %v, want %v", err, apperrors.ErrImageNotFound)
	}
}

func TestImageUseCase_Replace(t *testing.T) {
	t.Run("replaces data and content type", func(t *testing.T) {
		store := newMemoryImageStore()
		uc := NewImageUseCase(newTestLogger(), store)
		id := uuid.New()
		store.images[id] = domain.Image{ID: id, Data: []byte{1}, ContentType: "image/png"}

		if err := uc.Replace(context.Background(), ReplaceImageInput{
			ID:          id,
			Data:        []byte{2, 3},
			ContentType: "image/jpeg",
		}); err != nil {
			t.Fatalf("Replace() error = %v", err)
		}

		got := store.images[id]
		if !bytes.Equal(got.Data, []byte{2, 3}) || got.ContentType != "image/jpeg" {
			t.Errorf("stored image = %+v, want data [2 3], content type image/jpeg", got)
		}
	})

	t.Run("rejects empty data", func(t *testing.T) {
		store := newMemoryImageStore()
		uc := NewImageUseCase(newTestLogger(), store)
		id := uuid.New()
		store.images[id] = domain.Image{ID: id, Data: []byte{1}, ContentType: "image/png"}

		if err := uc.Replace(context.Background(), ReplaceImageInput{ID: id}); !errors.Is(err, apperrors.ErrEmptyImageData) {
			t.Fatalf("Replace() error = %v, want %v", err, apperrors.ErrEmptyImageData)
		}
		if got := store.images[id]; !bytes.Equal(got.Data, []byte{1}) {
			t.Errorf("stored data = %v, want unchanged [1]", got.Data)
		}
	})

	t.Run("returns not found", func(t *testing.T) {
		uc := NewImageUseCase(newTestLogger(), newMemoryImageStore())

		err := uc.Replace(context.Background(), ReplaceImageInput{ID: uuid.New(), Data: []byte{1}})
		if !errors.Is(err, apperrors.ErrImageNotFound) {
			t.Fatalf("Replace() error = %v, want %v", err, apperrors.ErrImageNotFound)
		}
	})
}

func TestImageUseCase_Delete(t *testing.T) {
	t.Run("deletes image", func(t *testing.T) {
		store := newMemoryImageStore()
		uc := NewImageUseCase(newTestLogger(), store)
		id := uuid.New()
		store.images[id] = domain.Image{ID: id, Data: []byte{1}}

		if err := uc.Delete(context.Background(), id); err != nil {
			t.Fatalf("Delete() error = %v", err)
		}
		if _, ok := store.images[id]; ok {
			t.Error("image still exists after Delete()")
		}
	})

	t.Run("returns not found", func(t *testing.T) {
		uc := NewImageUseCase(newTestLogger(), newMemoryImageStore())

		if err := uc.Delete(context.Background(), uuid.New()); !errors.Is(err, apperrors.ErrImageNotFound) {
			t.Fatalf("Delete() error = %v, want %v", err, apperrors.ErrImageNotFound)
		}
	})
}

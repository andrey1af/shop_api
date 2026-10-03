package repository

import (
	"context"
	"errors"
	"fmt"

	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/andrey1af/shop-api/backend/api-service/internal/database/sqlc"
	"github.com/andrey1af/shop-api/backend/api-service/internal/domain"
)

type ImageCache interface {
	Get(ctx context.Context, imageID uuid.UUID) (data []byte, contentType string, ok bool)
	Set(ctx context.Context, imageID uuid.UUID, data []byte, contentType string)
	Delete(ctx context.Context, imageID uuid.UUID)
}

type ImageStorage interface {
	Create(ctx context.Context, imageID uuid.UUID, data []byte, contentType string) error
	Get(ctx context.Context, imageID uuid.UUID) (data []byte, contentType string, err error)
	Replace(ctx context.Context, imageID uuid.UUID, data []byte, contentType string) error
	Delete(ctx context.Context, imageID uuid.UUID) error
}

type ImageRepository struct {
	queries *sqlc.Queries
	storage ImageStorage
	cache   ImageCache
}

func NewImageRepository(db sqlc.DBTX, storage ImageStorage, cache ImageCache) *ImageRepository {
	if cache == nil {
		cache = nopImageCache{}
	}

	return &ImageRepository{
		queries: sqlc.New(db),
		storage: storage,
		cache:   cache,
	}
}

func (r *ImageRepository) Create(ctx context.Context, image domain.Image) (domain.Image, error) {
	const op = "repository.ImageRepository.Create"

	if err := r.storage.Create(ctx, image.ID, image.Data, image.ContentType); err != nil {
		return domain.Image{}, fmt.Errorf("%s: %w", op, err)
	}

	if _, err := r.queries.LinkImageToProduct(ctx, sqlc.LinkImageToProductParams{
		ProductID: image.ProductID,
		ImageID:   &image.ID,
	}); err != nil {
		r.removeStored(ctx, image.ID)

		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Image{}, fmt.Errorf("%s: %w", op, domain.ErrProductNotFound)
		}

		return domain.Image{}, fmt.Errorf("%s: %w", op, err)
	}

	return image, nil
}

func (r *ImageRepository) GetByID(ctx context.Context, imageID uuid.UUID) (domain.Image, error) {
	const op = "repository.ImageRepository.GetByID"

	productID, err := r.queries.GetImageProductID(ctx, &imageID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Image{}, fmt.Errorf("%s: %w", op, domain.ErrImageNotFound)
		}

		return domain.Image{}, fmt.Errorf("%s: %w", op, err)
	}

	data, contentType, err := r.load(ctx, imageID)
	if err != nil {
		return domain.Image{}, fmt.Errorf("%s: %w", op, err)
	}

	return domain.Image{ID: imageID, ProductID: productID, Data: data, ContentType: contentType}, nil
}

func (r *ImageRepository) GetByProductID(ctx context.Context, productID uuid.UUID) (domain.Image, error) {
	const op = "repository.ImageRepository.GetByProductID"

	imageID, err := r.queries.GetImageIDByProductID(ctx, productID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Image{}, fmt.Errorf("%s: %w", op, domain.ErrImageNotFound)
		}

		return domain.Image{}, fmt.Errorf("%s: %w", op, err)
	}

	data, contentType, err := r.load(ctx, *imageID)
	if err != nil {
		return domain.Image{}, fmt.Errorf("%s: %w", op, err)
	}

	return domain.Image{ID: *imageID, ProductID: productID, Data: data, ContentType: contentType}, nil
}

func (r *ImageRepository) Replace(
	ctx context.Context,
	imageID uuid.UUID,
	data []byte,
	contentType string,
) (domain.Image, error) {
	const op = "repository.ImageRepository.Replace"

	productID, err := r.queries.GetImageProductID(ctx, &imageID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Image{}, fmt.Errorf("%s: %w", op, domain.ErrImageNotFound)
		}

		return domain.Image{}, fmt.Errorf("%s: %w", op, err)
	}

	if err := r.storage.Replace(ctx, imageID, data, contentType); err != nil {
		return domain.Image{}, fmt.Errorf("%s: %w", op, err)
	}

	r.cache.Delete(ctx, imageID)

	return domain.Image{ID: imageID, ProductID: productID, Data: data, ContentType: contentType}, nil
}

func (r *ImageRepository) Delete(ctx context.Context, imageID uuid.UUID) error {
	const op = "repository.ImageRepository.Delete"

	if _, err := r.queries.UnlinkImage(ctx, &imageID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("%s: %w", op, domain.ErrImageNotFound)
		}

		return fmt.Errorf("%s: %w", op, err)
	}

	r.cache.Delete(ctx, imageID)

	if err := r.storage.Delete(ctx, imageID); err != nil && !errors.Is(err, domain.ErrImageNotFound) {
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}

func (r *ImageRepository) load(ctx context.Context, imageID uuid.UUID) ([]byte, string, error) {
	if data, contentType, ok := r.cache.Get(ctx, imageID); ok {
		return data, contentType, nil
	}

	data, contentType, err := r.storage.Get(ctx, imageID)
	if err != nil {
		return nil, "", err
	}

	r.cache.Set(ctx, imageID, data, contentType)

	return data, contentType, nil
}

func (r *ImageRepository) removeStored(ctx context.Context, imageID uuid.UUID) {
	_ = r.storage.Delete(ctx, imageID)
}

type nopImageCache struct{}

func (nopImageCache) Get(context.Context, uuid.UUID) ([]byte, string, bool) { return nil, "", false }

func (nopImageCache) Set(context.Context, uuid.UUID, []byte, string) {}

func (nopImageCache) Delete(context.Context, uuid.UUID) {}

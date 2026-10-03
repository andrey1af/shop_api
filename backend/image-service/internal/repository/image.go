package repository

import (
	"context"
	"errors"
	"fmt"
	"uuid"

	"github.com/andrey1af/shop-api/backend/image-service/internal/database/sqlc"
	"github.com/andrey1af/shop-api/backend/image-service/internal/domain"
	apperrors "github.com/andrey1af/shop-api/backend/image-service/internal/errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const uniqueViolation = "23505"

type ImageRepository struct {
	shards []*sqlc.Queries
}

func NewImageRepository(shards []sqlc.DBTX) (*ImageRepository, error) {
	const op = "repository.NewImageRepository"

	if len(shards) != ShardCount {
		return nil, fmt.Errorf("%s: expected %d shards, got %d", op, ShardCount, len(shards))
	}

	queries := make([]*sqlc.Queries, 0, len(shards))
	for _, db := range shards {
		queries = append(queries, sqlc.New(db))
	}

	return &ImageRepository{shards: queries}, nil
}

func (r *ImageRepository) Create(ctx context.Context, image domain.Image) error {
	const op = "repository.ImageRepository.Create"

	if err := r.shard(image.ID).CreateImage(ctx, sqlc.CreateImageParams{
		ID:          image.ID,
		Data:        image.Data,
		ContentType: image.ContentType,
	}); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
			return fmt.Errorf("%s: %w", op, apperrors.ErrImageAlreadyExists)
		}

		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}

func (r *ImageRepository) Get(ctx context.Context, id uuid.UUID) (domain.Image, error) {
	const op = "repository.ImageRepository.Get"

	image, err := r.shard(id).GetImage(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Image{}, fmt.Errorf("%s: %w", op, apperrors.ErrImageNotFound)
		}

		return domain.Image{}, fmt.Errorf("%s: %w", op, err)
	}

	return domain.Image{
		ID:          image.ID,
		Data:        image.Data,
		ContentType: image.ContentType,
	}, nil
}

func (r *ImageRepository) Update(ctx context.Context, image domain.Image) error {
	const op = "repository.ImageRepository.Update"

	if _, err := r.shard(image.ID).UpdateImage(ctx, sqlc.UpdateImageParams{
		ID:          image.ID,
		Data:        image.Data,
		ContentType: image.ContentType,
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("%s: %w", op, apperrors.ErrImageNotFound)
		}

		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}

func (r *ImageRepository) Delete(ctx context.Context, id uuid.UUID) error {
	const op = "repository.ImageRepository.Delete"

	if _, err := r.shard(id).DeleteImage(ctx, id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("%s: %w", op, apperrors.ErrImageNotFound)
		}

		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}

func (r *ImageRepository) shard(id uuid.UUID) *sqlc.Queries {
	return r.shards[shardIndex(id)]
}

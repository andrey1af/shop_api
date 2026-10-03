package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"uuid"

	"github.com/andrey1af/shop-api/backend/image-service/internal/domain"
	apperrors "github.com/andrey1af/shop-api/backend/image-service/internal/errors"
)

const defaultContentType = "application/octet-stream"

type ImageRepository interface {
	Create(ctx context.Context, image domain.Image) error
	Get(ctx context.Context, id uuid.UUID) (domain.Image, error)
	Update(ctx context.Context, image domain.Image) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type CreateImageInput struct {
	ID          uuid.UUID
	Data        []byte
	ContentType string
}

type ReplaceImageInput struct {
	ID          uuid.UUID
	Data        []byte
	ContentType string
}

type ImageOutput struct {
	ID          uuid.UUID
	Data        []byte
	ContentType string
}

type ImageUseCase struct {
	log    *slog.Logger
	images ImageRepository
}

func NewImageUseCase(log *slog.Logger, images ImageRepository) *ImageUseCase {
	return &ImageUseCase{
		log:    log,
		images: images,
	}
}

func (uc *ImageUseCase) Create(ctx context.Context, in CreateImageInput) error {
	const op = "imageUseCase.Create"
	log := uc.log.With(slog.String("op", op), slog.String("image_id", in.ID.String()))

	log.Info("create image")

	image, err := newImage(in.ID, in.Data, in.ContentType)
	if err != nil {
		log.Warn("invalid image", slog.String("error", err.Error()))

		return fmt.Errorf("%s: %w", op, err)
	}

	if err := uc.images.Create(ctx, image); err != nil {
		if errors.Is(err, apperrors.ErrImageAlreadyExists) {
			log.Warn("image already exists")

			return fmt.Errorf("%s: %w", op, err)
		}

		log.Error("failed to create image", slog.String("error", err.Error()))

		return fmt.Errorf("%s: %w", op, err)
	}

	log.Info("image created")

	return nil
}

func (uc *ImageUseCase) Get(ctx context.Context, id uuid.UUID) (ImageOutput, error) {
	const op = "imageUseCase.Get"
	log := uc.log.With(slog.String("op", op), slog.String("image_id", id.String()))

	image, err := uc.images.Get(ctx, id)
	if err != nil {
		if errors.Is(err, apperrors.ErrImageNotFound) {
			log.Warn("image not found")

			return ImageOutput{}, fmt.Errorf("%s: %w", op, err)
		}

		log.Error("failed to get image", slog.String("error", err.Error()))

		return ImageOutput{}, fmt.Errorf("%s: %w", op, err)
	}

	return ImageOutput{
		ID:          image.ID,
		Data:        image.Data,
		ContentType: image.ContentType,
	}, nil
}

func (uc *ImageUseCase) Replace(ctx context.Context, in ReplaceImageInput) error {
	const op = "imageUseCase.Replace"
	log := uc.log.With(slog.String("op", op), slog.String("image_id", in.ID.String()))

	log.Info("replace image")

	image, err := newImage(in.ID, in.Data, in.ContentType)
	if err != nil {
		log.Warn("invalid image", slog.String("error", err.Error()))

		return fmt.Errorf("%s: %w", op, err)
	}

	if err := uc.images.Update(ctx, image); err != nil {
		if errors.Is(err, apperrors.ErrImageNotFound) {
			log.Warn("image not found")

			return fmt.Errorf("%s: %w", op, err)
		}

		log.Error("failed to replace image", slog.String("error", err.Error()))

		return fmt.Errorf("%s: %w", op, err)
	}

	log.Info("image replaced")

	return nil
}

func (uc *ImageUseCase) Delete(ctx context.Context, id uuid.UUID) error {
	const op = "imageUseCase.Delete"
	log := uc.log.With(slog.String("op", op), slog.String("image_id", id.String()))

	log.Info("delete image")

	if err := uc.images.Delete(ctx, id); err != nil {
		if errors.Is(err, apperrors.ErrImageNotFound) {
			log.Warn("image not found")

			return fmt.Errorf("%s: %w", op, err)
		}

		log.Error("failed to delete image", slog.String("error", err.Error()))

		return fmt.Errorf("%s: %w", op, err)
	}

	log.Info("image deleted")

	return nil
}

func newImage(id uuid.UUID, data []byte, contentType string) (domain.Image, error) {
	if len(data) == 0 {
		return domain.Image{}, apperrors.ErrEmptyImageData
	}
	if contentType == "" {
		contentType = defaultContentType
	}

	return domain.Image{
		ID:          id,
		Data:        data,
		ContentType: contentType,
	}, nil
}

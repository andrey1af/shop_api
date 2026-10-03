package usecase

import (
	"context"
	"errors"
	"log/slog"

	"uuid"

	"github.com/andrey1af/shop-api/backend/api-service/internal/domain"
)

type CreateImageInput struct {
	ProductID   uuid.UUID
	Data        []byte
	ContentType string
}

type ImageRepository interface {
	Create(ctx context.Context, image domain.Image) (domain.Image, error)
	GetByID(ctx context.Context, imageID uuid.UUID) (domain.Image, error)
	GetByProductID(ctx context.Context, productID uuid.UUID) (domain.Image, error)
	Replace(ctx context.Context, imageID uuid.UUID, data []byte, contentType string) (domain.Image, error)
	Delete(ctx context.Context, imageID uuid.UUID) error
}

type ImageUseCase struct {
	log         *slog.Logger
	imageRepo   ImageRepository
	productRepo ProductRepository
}

func NewImageUseCase(log *slog.Logger, imageRepo ImageRepository, productRepo ProductRepository) *ImageUseCase {
	return &ImageUseCase{
		log:         log,
		imageRepo:   imageRepo,
		productRepo: productRepo,
	}
}

func (uc *ImageUseCase) Create(ctx context.Context, in CreateImageInput) (domain.Image, error) {
	const op = "imageUseCase.Create"
	log := uc.log.With(slog.String("op", op))

	log.Info("upload image", slog.String("product_id", in.ProductID.String()))

	product, err := uc.productRepo.Get(ctx, in.ProductID)
	if err != nil {
		if errors.Is(err, domain.ErrProductNotFound) {
			log.Warn("product not found", slog.String("product_id", in.ProductID.String()))

			return domain.Image{}, err
		}

		log.Error("failed to get product", slog.String("error", err.Error()))

		return domain.Image{}, err
	}
	if product.ImageID != nil {
		log.Warn("product already has an image", slog.String("product_id", in.ProductID.String()))

		return domain.Image{}, domain.ErrImageAlreadyExists
	}

	image := domain.Image{
		ID:          uuid.New(),
		ProductID:   in.ProductID,
		Data:        in.Data,
		ContentType: in.ContentType,
	}

	createdImage, err := uc.imageRepo.Create(ctx, image)
	if err != nil {
		log.Error("failed to create image", slog.String("error", err.Error()))

		return domain.Image{}, err
	}

	log.Info("image uploaded", slog.String("image_id", createdImage.ID.String()))

	return createdImage, nil
}

func (uc *ImageUseCase) GetByID(ctx context.Context, imageID uuid.UUID) (domain.Image, error) {
	const op = "imageUseCase.GetByID"
	log := uc.log.With(slog.String("op", op))

	log.Info("get image", slog.String("image_id", imageID.String()))

	image, err := uc.imageRepo.GetByID(ctx, imageID)
	if err != nil {
		if errors.Is(err, domain.ErrImageNotFound) {
			log.Warn("image not found", slog.String("image_id", imageID.String()))

			return domain.Image{}, err
		}

		log.Error("failed to get image", slog.String("error", err.Error()))

		return domain.Image{}, err
	}

	return image, nil
}

func (uc *ImageUseCase) GetByProductID(ctx context.Context, productID uuid.UUID) (domain.Image, error) {
	const op = "imageUseCase.GetByProductID"
	log := uc.log.With(slog.String("op", op))

	log.Info("get image", slog.String("product_id", productID.String()))

	image, err := uc.imageRepo.GetByProductID(ctx, productID)
	if err != nil {
		if errors.Is(err, domain.ErrImageNotFound) {
			log.Warn("image not found", slog.String("product_id", productID.String()))

			return domain.Image{}, err
		}

		log.Error("failed to get image", slog.String("error", err.Error()))

		return domain.Image{}, err
	}

	return image, nil
}

func (uc *ImageUseCase) Replace(
	ctx context.Context,
	imageID uuid.UUID,
	data []byte,
	contentType string,
) (domain.Image, error) {
	const op = "imageUseCase.Replace"
	log := uc.log.With(slog.String("op", op))

	log.Info("replace image", slog.String("image_id", imageID.String()))

	updatedImage, err := uc.imageRepo.Replace(ctx, imageID, data, contentType)
	if err != nil {
		if errors.Is(err, domain.ErrImageNotFound) {
			log.Warn("image not found", slog.String("image_id", imageID.String()))

			return domain.Image{}, err
		}

		log.Error("failed to replace image", slog.String("error", err.Error()))

		return domain.Image{}, err
	}

	return updatedImage, nil
}

func (uc *ImageUseCase) Delete(ctx context.Context, imageID uuid.UUID) error {
	const op = "imageUseCase.Delete"
	log := uc.log.With(slog.String("op", op))

	log.Info("delete image", slog.String("image_id", imageID.String()))

	if err := uc.imageRepo.Delete(ctx, imageID); err != nil {
		if errors.Is(err, domain.ErrImageNotFound) {
			log.Warn("image not found", slog.String("image_id", imageID.String()))

			return err
		}

		log.Error("failed to delete image", slog.String("error", err.Error()))

		return err
	}

	return nil
}

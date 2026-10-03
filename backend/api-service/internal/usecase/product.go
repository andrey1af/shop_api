package usecase

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"uuid"

	"github.com/andrey1af/shop-api/backend/api-service/internal/domain"
	apperrors "github.com/andrey1af/shop-api/backend/api-service/internal/errors"
)

type CreateProductInput struct {
	Name           string
	Category       string
	Price          float64
	AvailableStock int64
	SupplierID     uuid.UUID
}

type ApplyProductUpdateInput struct {
	ProductID      uuid.UUID
	Price          float64
	AvailableStock int64
}

type ProductRepository interface {
	Create(ctx context.Context, product domain.Product) (domain.Product, error)
	Delete(ctx context.Context, productID uuid.UUID) error
	Get(ctx context.Context, productID uuid.UUID) (domain.Product, error)
	ListAvailable(ctx context.Context, p domain.ListParams) ([]domain.Product, error)
	UpdateStock(ctx context.Context, productID uuid.UUID, newStock int64) (domain.Product, error)
	UpdatePriceAndStock(ctx context.Context, productID uuid.UUID, price float64, newStock int64) (domain.Product, error)
	ExistsBySupplierID(ctx context.Context, supplierID uuid.UUID) (bool, error)
}

type ProductUpdateNotifier interface {
	NotifyProductUpdated(product domain.Product)
}

type ProductUseCase struct {
	log          *slog.Logger
	productRepo  ProductRepository
	supplierRepo SupplierRepository
	notifier     ProductUpdateNotifier
	now          func() time.Time
}

func NewProductUseCase(
	log *slog.Logger,
	productRepo ProductRepository,
	supplierRepo SupplierRepository,
	notifier ProductUpdateNotifier,
) *ProductUseCase {
	return &ProductUseCase{
		log:          log,
		productRepo:  productRepo,
		supplierRepo: supplierRepo,
		notifier:     notifier,
		now:          time.Now,
	}
}

func (uc *ProductUseCase) Create(ctx context.Context, in CreateProductInput) (domain.Product, error) {
	const op = "productUseCase.Create"
	log := uc.log.With(slog.String("op", op))

	log.Info("create product")

	if _, err := uc.supplierRepo.Get(ctx, in.SupplierID); err != nil {
		if errors.Is(err, domain.ErrSupplierNotFound) {
			log.Warn("supplier not found", slog.String("supplier_id", in.SupplierID.String()))

			return domain.Product{}, err
		}

		log.Error("failed to get supplier", slog.String("error", err.Error()))

		return domain.Product{}, err
	}

	product := domain.Product{
		ID:             uuid.New(),
		Name:           in.Name,
		Category:       in.Category,
		Price:          in.Price,
		AvailableStock: in.AvailableStock,
		LastUpdateDate: uc.now(),
		SupplierID:     in.SupplierID,
	}

	createdProduct, err := uc.productRepo.Create(ctx, product)
	if err != nil {
		log.Error("failed to create product", slog.String("error", err.Error()))

		return domain.Product{}, err
	}

	log.Info("product created", slog.String("product_id", createdProduct.ID.String()))

	return createdProduct, nil
}

func (uc *ProductUseCase) Delete(ctx context.Context, productID uuid.UUID) error {
	const op = "productUseCase.Delete"
	log := uc.log.With(slog.String("op", op))

	log.Info("delete product", slog.String("product_id", productID.String()))

	if err := uc.productRepo.Delete(ctx, productID); err != nil {
		if errors.Is(err, domain.ErrProductNotFound) {
			log.Warn("product not found", slog.String("product_id", productID.String()))

			return err
		}

		log.Error("failed to delete product", slog.String("error", err.Error()))

		return err
	}

	return nil
}

func (uc *ProductUseCase) Get(ctx context.Context, productID uuid.UUID) (domain.Product, error) {
	const op = "productUseCase.Get"
	log := uc.log.With(slog.String("op", op))

	log.Info("get product", slog.String("product_id", productID.String()))

	product, err := uc.productRepo.Get(ctx, productID)
	if err != nil {
		if errors.Is(err, domain.ErrProductNotFound) {
			log.Warn("product not found", slog.String("product_id", productID.String()))

			return domain.Product{}, err
		}

		log.Error("failed to get product", slog.String("error", err.Error()))

		return domain.Product{}, err
	}

	return product, nil
}

func (uc *ProductUseCase) ListAvailable(ctx context.Context, p domain.ListParams) ([]domain.Product, error) {
	const op = "productUseCase.ListAvailable"
	log := uc.log.With(slog.String("op", op))

	log.Info("list available products")

	products, err := uc.productRepo.ListAvailable(ctx, p)
	if err != nil {
		log.Error("failed to list available products", slog.String("error", err.Error()))

		return []domain.Product{}, err
	}
	return products, nil
}

func (uc *ProductUseCase) DecreaseStock(
	ctx context.Context,
	productID uuid.UUID,
	decreaseBy int64,
) (domain.Product, error) {
	const op = "productUseCase.DecreaseStock"
	log := uc.log.With(slog.String("op", op))

	log.Info("decrease product stock", slog.String("product_id", productID.String()))

	product, err := uc.productRepo.Get(ctx, productID)
	if err != nil {
		if errors.Is(err, domain.ErrProductNotFound) {
			log.Warn("product not found", slog.String("product_id", productID.String()))

			return domain.Product{}, err
		}

		log.Error("failed to get product", slog.String("error", err.Error()))

		return domain.Product{}, err
	}

	if decreaseBy > product.AvailableStock {
		log.Warn("insufficient stock", slog.String("product_id", productID.String()))

		return domain.Product{}, domain.ErrInsufficientStock
	}

	updatedProduct, err := uc.productRepo.UpdateStock(ctx, productID, product.AvailableStock-decreaseBy)
	if err != nil {
		log.Error("failed to update product stock", slog.String("error", err.Error()))

		return domain.Product{}, err
	}

	uc.notifier.NotifyProductUpdated(updatedProduct)

	return updatedProduct, nil
}

func (uc *ProductUseCase) ApplyUpdate(ctx context.Context, in ApplyProductUpdateInput) (domain.Product, error) {
	const op = "productUseCase.ApplyUpdate"
	log := uc.log.With(slog.String("op", op))

	log.Info("apply product update", slog.String("product_id", in.ProductID.String()))

	if in.Price <= 0 || in.AvailableStock < 0 {
		log.Warn("invalid product update",
			slog.String("product_id", in.ProductID.String()),
			slog.Float64("price", in.Price),
			slog.Int64("available_stock", in.AvailableStock),
		)

		return domain.Product{}, apperrors.ErrInvalidProductUpdate
	}

	updatedProduct, err := uc.productRepo.UpdatePriceAndStock(ctx, in.ProductID, in.Price, in.AvailableStock)
	if err != nil {
		if errors.Is(err, domain.ErrProductNotFound) {
			log.Warn("product not found", slog.String("product_id", in.ProductID.String()))

			return domain.Product{}, err
		}

		log.Error("failed to update product price and stock", slog.String("error", err.Error()))

		return domain.Product{}, err
	}

	uc.notifier.NotifyProductUpdated(updatedProduct)

	return updatedProduct, nil
}

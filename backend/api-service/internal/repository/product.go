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

type ProductRepository struct {
	queries *sqlc.Queries
}

func NewProductRepository(db sqlc.DBTX) *ProductRepository {
	return &ProductRepository{queries: sqlc.New(db)}
}

func (r *ProductRepository) Create(ctx context.Context, product domain.Product) (domain.Product, error) {
	const op = "repository.ProductRepository.Create"

	err := r.queries.CreateProduct(ctx, sqlc.CreateProductParams{
		ProductID:      product.ID,
		Name:           product.Name,
		Category:       product.Category,
		Price:          product.Price,
		AvailableStock: product.AvailableStock,
		LastUpdateDate: product.LastUpdateDate,
		SupplierID:     product.SupplierID,
	})
	if err != nil {
		return domain.Product{}, fmt.Errorf("%s: %w", op, err)
	}

	return product, nil
}

func (r *ProductRepository) Delete(ctx context.Context, productID uuid.UUID) error {
	const op = "repository.ProductRepository.Delete"

	if _, err := r.queries.DeleteProduct(ctx, productID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("%s: %w", op, domain.ErrProductNotFound)
		}

		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}

func (r *ProductRepository) Get(ctx context.Context, productID uuid.UUID) (domain.Product, error) {
	const op = "repository.ProductRepository.Get"

	product, err := r.queries.GetProduct(ctx, productID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Product{}, fmt.Errorf("%s: %w", op, domain.ErrProductNotFound)
		}

		return domain.Product{}, fmt.Errorf("%s: %w", op, err)
	}

	return toDomainProduct(product), nil
}

func (r *ProductRepository) ListAvailable(ctx context.Context, p domain.ListParams) ([]domain.Product, error) {
	const op = "repository.ProductRepository.ListAvailable"

	products, err := r.queries.ListAvailableProducts(ctx, sqlc.ListAvailableProductsParams{
		Limit:  nullableInt64(p.Limit),
		Offset: nullableInt64(p.Offset),
	})
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	result := make([]domain.Product, 0, len(products))
	for _, product := range products {
		result = append(result, toDomainProduct(product))
	}

	return result, nil
}

func (r *ProductRepository) UpdateStock(
	ctx context.Context,
	productID uuid.UUID,
	newStock int64,
) (domain.Product, error) {
	const op = "repository.ProductRepository.UpdateStock"

	product, err := r.queries.UpdateProductStock(ctx, sqlc.UpdateProductStockParams{
		ProductID:      productID,
		AvailableStock: newStock,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Product{}, fmt.Errorf("%s: %w", op, domain.ErrProductNotFound)
		}

		return domain.Product{}, fmt.Errorf("%s: %w", op, err)
	}

	return toDomainProduct(product), nil
}

func (r *ProductRepository) UpdatePriceAndStock(
	ctx context.Context,
	productID uuid.UUID,
	price float64,
	newStock int64,
) (domain.Product, error) {
	const op = "repository.ProductRepository.UpdatePriceAndStock"

	product, err := r.queries.UpdateProductPriceAndStock(ctx, sqlc.UpdateProductPriceAndStockParams{
		ProductID:      productID,
		Price:          price,
		AvailableStock: newStock,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Product{}, fmt.Errorf("%s: %w", op, domain.ErrProductNotFound)
		}

		return domain.Product{}, fmt.Errorf("%s: %w", op, err)
	}

	return toDomainProduct(product), nil
}

func (r *ProductRepository) ExistsBySupplierID(ctx context.Context, supplierID uuid.UUID) (bool, error) {
	const op = "repository.ProductRepository.ExistsBySupplierID"

	exists, err := r.queries.ProductExistsBySupplierID(ctx, supplierID)
	if err != nil {
		return false, fmt.Errorf("%s: %w", op, err)
	}

	return exists, nil
}

func toDomainProduct(product sqlc.Product) domain.Product {
	return domain.Product{
		ID:             product.ID,
		Name:           product.Name,
		Category:       product.Category,
		Price:          product.Price,
		AvailableStock: product.AvailableStock,
		LastUpdateDate: product.LastUpdateDate,
		SupplierID:     product.SupplierID,
		ImageID:        product.ImageID,
	}
}

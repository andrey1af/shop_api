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

type SupplierRepository struct {
	queries *sqlc.Queries
}

func NewSupplierRepository(db sqlc.DBTX) *SupplierRepository {
	return &SupplierRepository{queries: sqlc.New(db)}
}

func (r *SupplierRepository) Create(ctx context.Context, supplier domain.Supplier) (domain.Supplier, error) {
	const op = "repository.SupplierRepository.Create"

	err := r.queries.CreateSupplier(ctx, sqlc.CreateSupplierParams{
		SupplierID:  supplier.ID,
		Name:        supplier.Name,
		PhoneNumber: supplier.PhoneNumber,
		AddressID:   supplier.Address.ID,
		Country:     supplier.Address.Country,
		City:        supplier.Address.City,
		Street:      supplier.Address.Street,
	})
	if err != nil {
		return domain.Supplier{}, fmt.Errorf("%s: %w", op, err)
	}

	return supplier, nil
}

func (r *SupplierRepository) Delete(ctx context.Context, supplierID uuid.UUID) error {
	const op = "repository.SupplierRepository.Delete"

	if _, err := r.queries.DeleteSupplier(ctx, supplierID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("%s: %w", op, domain.ErrSupplierNotFound)
		}

		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}

func (r *SupplierRepository) Get(ctx context.Context, supplierID uuid.UUID) (domain.Supplier, error) {
	const op = "repository.SupplierRepository.Get"

	row, err := r.queries.GetSupplier(ctx, supplierID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Supplier{}, fmt.Errorf("%s: %w", op, domain.ErrSupplierNotFound)
		}

		return domain.Supplier{}, fmt.Errorf("%s: %w", op, err)
	}

	return toDomainSupplier(row.Supplier, row.Address), nil
}

func (r *SupplierRepository) List(ctx context.Context) ([]domain.Supplier, error) {
	const op = "repository.SupplierRepository.List"

	rows, err := r.queries.ListSuppliers(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	suppliers := make([]domain.Supplier, 0, len(rows))
	for _, row := range rows {
		suppliers = append(suppliers, toDomainSupplier(row.Supplier, row.Address))
	}

	return suppliers, nil
}

func (r *SupplierRepository) UpdateAddress(
	ctx context.Context,
	supplierID uuid.UUID,
	address domain.Address,
) (domain.Supplier, error) {
	const op = "repository.SupplierRepository.UpdateAddress"

	row, err := r.queries.UpdateSupplierAddress(ctx, sqlc.UpdateSupplierAddressParams{
		SupplierID: supplierID,
		Country:    address.Country,
		City:       address.City,
		Street:     address.Street,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Supplier{}, fmt.Errorf("%s: %w", op, domain.ErrSupplierNotFound)
		}

		return domain.Supplier{}, fmt.Errorf("%s: %w", op, err)
	}

	return toDomainSupplier(row.Supplier, row.Address), nil
}

func toDomainSupplier(supplier sqlc.Supplier, address sqlc.Address) domain.Supplier {
	return domain.Supplier{
		ID:          supplier.ID,
		Name:        supplier.Name,
		PhoneNumber: supplier.PhoneNumber,
		Address:     toDomainAddress(address),
	}
}

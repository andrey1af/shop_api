package usecase

import (
	"context"
	"errors"
	"log/slog"

	"uuid"

	"github.com/andrey1af/shop-api/backend/api-service/internal/domain"
)

type CreateSupplierInput struct {
	Name        string
	PhoneNumber string
	Address     CreateAddressInput
}

type SupplierRepository interface {
	Create(ctx context.Context, supplier domain.Supplier) (domain.Supplier, error)
	Delete(ctx context.Context, supplierID uuid.UUID) error
	Get(ctx context.Context, supplierID uuid.UUID) (domain.Supplier, error)
	List(ctx context.Context) ([]domain.Supplier, error)
	UpdateAddress(ctx context.Context, supplierID uuid.UUID, address domain.Address) (domain.Supplier, error)
}

type SupplierUseCase struct {
	log          *slog.Logger
	supplierRepo SupplierRepository
	productRepo  ProductRepository
}

func NewSupplierUseCase(
	log *slog.Logger,
	supplierRepo SupplierRepository,
	productRepo ProductRepository,
) *SupplierUseCase {
	return &SupplierUseCase{
		log:          log,
		supplierRepo: supplierRepo,
		productRepo:  productRepo,
	}
}

func (uc *SupplierUseCase) Create(ctx context.Context, in CreateSupplierInput) (domain.Supplier, error) {
	const op = "supplierUseCase.Create"
	log := uc.log.With(slog.String("op", op))

	log.Info("create supplier")

	supplier := domain.Supplier{
		ID:          uuid.New(),
		Name:        in.Name,
		PhoneNumber: in.PhoneNumber,
		Address: domain.Address{
			ID:      uuid.New(),
			Country: in.Address.Country,
			City:    in.Address.City,
			Street:  in.Address.Street,
		},
	}

	createdSupplier, err := uc.supplierRepo.Create(ctx, supplier)
	if err != nil {
		log.Error("failed to create supplier", slog.String("error", err.Error()))

		return domain.Supplier{}, err
	}

	log.Info("supplier created", slog.String("supplier_id", createdSupplier.ID.String()))

	return createdSupplier, nil
}

func (uc *SupplierUseCase) Delete(ctx context.Context, supplierID uuid.UUID) error {
	const op = "supplierUseCase.Delete"
	log := uc.log.With(slog.String("op", op))

	log.Info("delete supplier", slog.String("supplier_id", supplierID.String()))

	inUse, err := uc.productRepo.ExistsBySupplierID(ctx, supplierID)
	if err != nil {
		log.Error("failed to check supplier usage", slog.String("error", err.Error()))

		return err
	}
	if inUse {
		log.Warn("supplier is used by products", slog.String("supplier_id", supplierID.String()))

		return domain.ErrSupplierInUse
	}

	if err := uc.supplierRepo.Delete(ctx, supplierID); err != nil {
		if errors.Is(err, domain.ErrSupplierNotFound) {
			log.Warn("supplier not found", slog.String("supplier_id", supplierID.String()))

			return err
		}

		log.Error("failed to delete supplier", slog.String("error", err.Error()))

		return err
	}

	return nil
}

func (uc *SupplierUseCase) Get(ctx context.Context, supplierID uuid.UUID) (domain.Supplier, error) {
	const op = "supplierUseCase.Get"
	log := uc.log.With(slog.String("op", op))

	log.Info("get supplier", slog.String("supplier_id", supplierID.String()))

	supplier, err := uc.supplierRepo.Get(ctx, supplierID)
	if err != nil {
		if errors.Is(err, domain.ErrSupplierNotFound) {
			log.Warn("supplier not found", slog.String("supplier_id", supplierID.String()))

			return domain.Supplier{}, err
		}

		log.Error("failed to get supplier", slog.String("error", err.Error()))

		return domain.Supplier{}, err
	}

	return supplier, nil
}

func (uc *SupplierUseCase) List(ctx context.Context) ([]domain.Supplier, error) {
	const op = "supplierUseCase.List"
	log := uc.log.With(slog.String("op", op))

	log.Info("list suppliers")

	suppliers, err := uc.supplierRepo.List(ctx)
	if err != nil {
		log.Error("failed to list suppliers", slog.String("error", err.Error()))

		return []domain.Supplier{}, err
	}
	return suppliers, nil
}

func (uc *SupplierUseCase) ChangeAddress(
	ctx context.Context,
	supplierID uuid.UUID,
	in CreateAddressInput,
) (domain.Supplier, error) {
	const op = "supplierUseCase.ChangeAddress"
	log := uc.log.With(slog.String("op", op))

	log.Info("change supplier address", slog.String("supplier_id", supplierID.String()))

	address := domain.Address{
		Country: in.Country,
		City:    in.City,
		Street:  in.Street,
	}

	updatedSupplier, err := uc.supplierRepo.UpdateAddress(ctx, supplierID, address)
	if err != nil {
		if errors.Is(err, domain.ErrSupplierNotFound) {
			log.Warn("supplier not found", slog.String("supplier_id", supplierID.String()))

			return domain.Supplier{}, err
		}

		log.Error("failed to change supplier address", slog.String("error", err.Error()))

		return domain.Supplier{}, err
	}

	return updatedSupplier, nil
}

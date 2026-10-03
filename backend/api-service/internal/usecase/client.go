package usecase

import (
	"context"
	"errors"
	"log/slog"
	"time"
	"uuid"

	"github.com/andrey1af/shop-api/backend/api-service/internal/domain"
)

type CreateAddressInput struct {
	Country string
	City    string
	Street  string
}

type CreateClientInput struct {
	Name     string
	Surname  string
	Birthday time.Time
	Gender   domain.GenderType
	Address  CreateAddressInput
}

type ClientRepository interface {
	Create(ctx context.Context, client domain.Client) (domain.Client, error)
	Delete(ctx context.Context, clientID uuid.UUID) error
	FindByNameAndSurname(ctx context.Context, name, surname string) ([]domain.Client, error)
	List(ctx context.Context, p domain.ListParams) ([]domain.Client, error)
	UpdateAddress(ctx context.Context, clientID uuid.UUID, address domain.Address) (domain.Client, error)
}

type ClientUseCase struct {
	log        *slog.Logger
	clientRepo ClientRepository
	now        func() time.Time
}

func NewClientUseCase(log *slog.Logger, clientRepo ClientRepository) *ClientUseCase {
	return &ClientUseCase{
		log:        log,
		clientRepo: clientRepo,
		now:        time.Now,
	}
}

func (uc *ClientUseCase) Create(ctx context.Context, in CreateClientInput) (domain.Client, error) {
	const op = "clientUseCase.Create"
	log := uc.log.With(slog.String("op", op))

	log.Info("create client")

	registrationTime := uc.now()

	client := domain.Client{
		ID:               uuid.New(),
		Name:             in.Name,
		Surname:          in.Surname,
		Birthday:         in.Birthday,
		Gender:           in.Gender,
		RegistrationTime: registrationTime,
		Address: domain.Address{
			ID:      uuid.New(),
			Country: in.Address.Country,
			City:    in.Address.City,
			Street:  in.Address.Street,
		},
	}

	createdClient, err := uc.clientRepo.Create(ctx, client)
	if err != nil {
		log.Error("failed to create client", slog.String("error", err.Error()))

		return domain.Client{}, err
	}

	log.Info("client created", slog.String("client_id", createdClient.ID.String()))

	return createdClient, nil
}

func (uc *ClientUseCase) Delete(ctx context.Context, clientID uuid.UUID) error {
	const op = "clientUseCase.Delete"
	log := uc.log.With(slog.String("op", op))

	log.Info("delete client", slog.String("client_id", clientID.String()))

	if err := uc.clientRepo.Delete(ctx, clientID); err != nil {
		if errors.Is(err, domain.ErrClientNotFound) {
			log.Warn("client not found", slog.String("client_id", clientID.String()))

			return err
		}

		log.Error("failed to delete client", slog.String("error", err.Error()))

		return err
	}

	return nil
}

func (uc *ClientUseCase) FindByNameAndSurname(ctx context.Context, name, surname string) ([]domain.Client, error) {
	const op = "clientUseCase.FindByNameAndSurname"
	log := uc.log.With(slog.String("op", op))

	log.Info("find clients by name and surname")

	clients, err := uc.clientRepo.FindByNameAndSurname(ctx, name, surname)
	if err != nil {
		log.Error("failed to find clients", slog.String("error", err.Error()))

		return []domain.Client{}, err
	}
	return clients, nil
}

func (uc *ClientUseCase) List(ctx context.Context, p domain.ListParams) ([]domain.Client, error) {
	const op = "clientUseCase.List"
	log := uc.log.With(slog.String("op", op))

	log.Info("list clients")

	clients, err := uc.clientRepo.List(ctx, p)
	if err != nil {
		log.Error("failed to list clients", slog.String("error", err.Error()))

		return []domain.Client{}, err
	}
	return clients, nil
}

func (uc *ClientUseCase) ChangeAddress(
	ctx context.Context,
	clientID uuid.UUID,
	in CreateAddressInput,
) (domain.Client, error) {
	const op = "clientUseCase.ChangeAddress"
	log := uc.log.With(slog.String("op", op))

	log.Info("change client address", slog.String("client_id", clientID.String()))

	address := domain.Address{
		Country: in.Country,
		City:    in.City,
		Street:  in.Street,
	}

	updatedClient, err := uc.clientRepo.UpdateAddress(ctx, clientID, address)
	if err != nil {
		if errors.Is(err, domain.ErrClientNotFound) {
			log.Warn("client not found", slog.String("client_id", clientID.String()))

			return domain.Client{}, err
		}

		log.Error("failed to change client address", slog.String("error", err.Error()))

		return domain.Client{}, err
	}

	return updatedClient, nil
}

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

type ClientRepository struct {
	queries *sqlc.Queries
}

func NewClientRepository(db sqlc.DBTX) *ClientRepository {
	return &ClientRepository{queries: sqlc.New(db)}
}

func (r *ClientRepository) Create(ctx context.Context, client domain.Client) (domain.Client, error) {
	const op = "repository.ClientRepository.Create"

	err := r.queries.CreateClient(ctx, sqlc.CreateClientParams{
		ClientID:         client.ID,
		ClientName:       client.Name,
		ClientSurname:    client.Surname,
		Birthday:         client.Birthday,
		Gender:           string(client.Gender),
		RegistrationDate: client.RegistrationTime,
		AddressID:        client.Address.ID,
		Country:          client.Address.Country,
		City:             client.Address.City,
		Street:           client.Address.Street,
	})
	if err != nil {
		return domain.Client{}, fmt.Errorf("%s: %w", op, err)
	}

	return client, nil
}

func (r *ClientRepository) Delete(ctx context.Context, clientID uuid.UUID) error {
	const op = "repository.ClientRepository.Delete"

	if _, err := r.queries.DeleteClient(ctx, clientID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("%s: %w", op, domain.ErrClientNotFound)
		}

		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}

func (r *ClientRepository) FindByNameAndSurname(ctx context.Context, name, surname string) ([]domain.Client, error) {
	const op = "repository.ClientRepository.FindByNameAndSurname"

	rows, err := r.queries.FindClientsByNameAndSurname(ctx, sqlc.FindClientsByNameAndSurnameParams{
		ClientName:    name,
		ClientSurname: surname,
	})
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	clients := make([]domain.Client, 0, len(rows))
	for _, row := range rows {
		clients = append(clients, toDomainClient(row.Client, row.Address))
	}

	return clients, nil
}

func (r *ClientRepository) List(ctx context.Context, p domain.ListParams) ([]domain.Client, error) {
	const op = "repository.ClientRepository.List"

	rows, err := r.queries.ListClients(ctx, sqlc.ListClientsParams{
		Limit:  nullableInt64(p.Limit),
		Offset: nullableInt64(p.Offset),
	})
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	clients := make([]domain.Client, 0, len(rows))
	for _, row := range rows {
		clients = append(clients, toDomainClient(row.Client, row.Address))
	}

	return clients, nil
}

func (r *ClientRepository) UpdateAddress(
	ctx context.Context,
	clientID uuid.UUID,
	address domain.Address,
) (domain.Client, error) {
	const op = "repository.ClientRepository.UpdateAddress"

	row, err := r.queries.UpdateClientAddress(ctx, sqlc.UpdateClientAddressParams{
		ClientID: clientID,
		Country:  address.Country,
		City:     address.City,
		Street:   address.Street,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Client{}, fmt.Errorf("%s: %w", op, domain.ErrClientNotFound)
		}

		return domain.Client{}, fmt.Errorf("%s: %w", op, err)
	}

	return toDomainClient(row.Client, row.Address), nil
}

func nullableInt64(value *int) *int64 {
	if value == nil {
		return nil
	}

	converted := int64(*value)

	return &converted
}

func toDomainClient(client sqlc.Client, address sqlc.Address) domain.Client {
	return domain.Client{
		ID:               client.ID,
		Name:             client.ClientName,
		Surname:          client.ClientSurname,
		Birthday:         client.Birthday,
		Gender:           domain.GenderType(client.Gender),
		RegistrationTime: client.RegistrationDate,
		Address:          toDomainAddress(address),
	}
}

func toDomainAddress(address sqlc.Address) domain.Address {
	return domain.Address{
		ID:      address.ID,
		Country: address.Country,
		City:    address.City,
		Street:  address.Street,
	}
}

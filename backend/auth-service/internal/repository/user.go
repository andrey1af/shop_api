package repository

import (
	"context"
	"errors"
	"fmt"
	"uuid"

	"github.com/andrey1af/shop-api/backend/auth-service/internal/database/sqlc"
	"github.com/andrey1af/shop-api/backend/auth-service/internal/domain"
	apperrors "github.com/andrey1af/shop-api/backend/auth-service/internal/errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

const uniqueViolation = "23505"

type UserRepository struct {
	queries *sqlc.Queries
}

func NewUserRepository(db sqlc.DBTX) *UserRepository {
	return &UserRepository{queries: sqlc.New(db)}
}

func (r *UserRepository) SaveUser(ctx context.Context, user domain.User) (domain.User, error) {
	const op = "repository.UserRepository.SaveUser"

	saved, err := r.queries.SaveUser(ctx, sqlc.SaveUserParams{
		ID:           user.ID,
		Name:         user.Name,
		Surname:      user.Surname,
		Email:        user.Email,
		PhoneNumber:  toNullableText(user.PhoneNumber),
		PasswordHash: user.PasswordHash,
		CreatedAt:    user.CreatedAt,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
			return domain.User{}, fmt.Errorf("%s: %w", op, apperrors.ErrUserAlreadyExists)
		}

		return domain.User{}, fmt.Errorf("%s: %w", op, err)
	}

	return toDomainUser(saved), nil
}

func (r *UserRepository) UserByID(ctx context.Context, id uuid.UUID) (domain.User, error) {
	const op = "repository.UserRepository.UserByID"

	user, err := r.queries.UserByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.User{}, fmt.Errorf("%s: %w", op, apperrors.ErrUserNotFound)
		}

		return domain.User{}, fmt.Errorf("%s: %w", op, err)
	}

	return toDomainUser(user), nil
}

func (r *UserRepository) UserByEmail(ctx context.Context, email string) (domain.User, error) {
	const op = "repository.UserRepository.UserByEmail"

	user, err := r.queries.UserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.User{}, fmt.Errorf("%s: %w", op, apperrors.ErrUserNotFound)
		}

		return domain.User{}, fmt.Errorf("%s: %w", op, err)
	}

	return toDomainUser(user), nil
}

func (r *UserRepository) UpdatePassword(ctx context.Context, userID uuid.UUID, passwordHash []byte) error {
	const op = "repository.UserRepository.UpdatePassword"

	affected, err := r.queries.UpdatePassword(ctx, sqlc.UpdatePasswordParams{
		ID:           userID,
		PasswordHash: passwordHash,
	})
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	if affected == 0 {
		return fmt.Errorf("%s: %w", op, apperrors.ErrUserNotFound)
	}

	return nil
}

func (r *UserRepository) UserByIdentity(ctx context.Context, provider, providerUserID string) (domain.User, error) {
	const op = "repository.UserRepository.UserByIdentity"

	row, err := r.queries.UserByIdentity(ctx, sqlc.UserByIdentityParams{
		Provider:       provider,
		ProviderUserID: providerUserID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.User{}, fmt.Errorf("%s: %w", op, apperrors.ErrUserNotFound)
		}

		return domain.User{}, fmt.Errorf("%s: %w", op, err)
	}

	return toDomainUser(row.User), nil
}

func (r *UserRepository) SaveUserWithIdentity(
	ctx context.Context,
	user domain.User,
	identity domain.UserIdentity,
) (domain.User, error) {
	const op = "repository.UserRepository.SaveUserWithIdentity"

	saved, err := r.queries.SaveUserWithIdentity(ctx, sqlc.SaveUserWithIdentityParams{
		ID:             user.ID,
		Name:           user.Name,
		Surname:        user.Surname,
		Email:          user.Email,
		PhoneNumber:    toNullableText(user.PhoneNumber),
		PasswordHash:   user.PasswordHash,
		CreatedAt:      user.CreatedAt,
		IdentityID:     identity.ID,
		Provider:       identity.Provider,
		ProviderUserID: identity.ProviderUserID,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
			return domain.User{}, fmt.Errorf("%s: %w", op, apperrors.ErrUserAlreadyExists)
		}

		return domain.User{}, fmt.Errorf("%s: %w", op, err)
	}

	return toDomainUser(sqlc.User(saved)), nil
}

func (r *UserRepository) SaveIdentity(ctx context.Context, identity domain.UserIdentity) error {
	const op = "repository.UserRepository.SaveIdentity"

	err := r.queries.SaveIdentity(ctx, sqlc.SaveIdentityParams{
		ID:             identity.ID,
		UserID:         identity.UserID,
		Provider:       identity.Provider,
		ProviderUserID: identity.ProviderUserID,
		Email:          identity.Email,
		CreatedAt:      identity.CreatedAt,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
			return fmt.Errorf("%s: %w", op, apperrors.ErrUserAlreadyExists)
		}

		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}

func toNullableText(value string) pgtype.Text {
	return pgtype.Text{String: value, Valid: value != ""}
}

func toDomainUser(user sqlc.User) domain.User {
	return domain.User{
		ID:           user.ID,
		Name:         user.Name,
		Surname:      user.Surname,
		Email:        user.Email,
		PhoneNumber:  user.PhoneNumber.String,
		PasswordHash: user.PasswordHash,
		CreatedAt:    user.CreatedAt,
	}
}

package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/andrey1af/shop-api/backend/auth-service/internal/database/sqlc"
	"github.com/andrey1af/shop-api/backend/auth-service/internal/domain"
	apperrors "github.com/andrey1af/shop-api/backend/auth-service/internal/errors"
	"github.com/jackc/pgx/v5"
)

type OAuthStateRepository struct {
	queries *sqlc.Queries
}

func NewOAuthStateRepository(db sqlc.DBTX) *OAuthStateRepository {
	return &OAuthStateRepository{queries: sqlc.New(db)}
}

func (r *OAuthStateRepository) SaveState(ctx context.Context, state domain.OAuthState) error {
	const op = "repository.OAuthStateRepository.SaveState"

	err := r.queries.SaveOAuthState(ctx, sqlc.SaveOAuthStateParams{
		State:        state.State,
		Provider:     state.Provider,
		CodeVerifier: state.CodeVerifier,
		ExpiresAt:    state.ExpiresAt,
	})
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}

func (r *OAuthStateRepository) ConsumeState(ctx context.Context, state string) (domain.OAuthState, error) {
	const op = "repository.OAuthStateRepository.ConsumeState"

	consumed, err := r.queries.ConsumeOAuthState(ctx, state)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.OAuthState{}, fmt.Errorf("%s: %w", op, apperrors.ErrInvalidOAuthState)
		}

		return domain.OAuthState{}, fmt.Errorf("%s: %w", op, err)
	}

	return domain.OAuthState{
		State:        consumed.State,
		Provider:     consumed.Provider,
		CodeVerifier: consumed.CodeVerifier,
		ExpiresAt:    consumed.ExpiresAt,
	}, nil
}

func (r *OAuthStateRepository) DeleteExpiredStates(ctx context.Context, now time.Time) error {
	const op = "repository.OAuthStateRepository.DeleteExpiredStates"

	if err := r.queries.DeleteExpiredOAuthStates(ctx, now); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}

package repository

import (
	"context"
	"errors"
	"fmt"
	"time"
	"uuid"

	"github.com/andrey1af/shop-api/backend/auth-service/internal/database/sqlc"
	"github.com/andrey1af/shop-api/backend/auth-service/internal/domain"
	apperrors "github.com/andrey1af/shop-api/backend/auth-service/internal/errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SessionRepository struct {
	pool    *pgxpool.Pool
	queries *sqlc.Queries
}

func NewSessionRepository(pool *pgxpool.Pool) *SessionRepository {
	return &SessionRepository{pool: pool, queries: sqlc.New(pool)}
}

func (r *SessionRepository) CreateSession(
	ctx context.Context,
	session domain.Session,
	token domain.RefreshToken,
) error {
	const op = "repository.SessionRepository.CreateSession"

	err := r.inTx(ctx, func(q *sqlc.Queries) error {
		if err := q.CreateSession(ctx, sqlc.CreateSessionParams{
			ID:        session.ID,
			UserID:    session.UserID,
			CreatedAt: session.CreatedAt,
			ExpiresAt: session.ExpiresAt,
		}); err != nil {
			return err
		}

		return q.CreateRefreshToken(ctx, toCreateRefreshTokenParams(token))
	})
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}

func (r *SessionRepository) SessionByID(ctx context.Context, id uuid.UUID) (domain.Session, error) {
	const op = "repository.SessionRepository.SessionByID"

	session, err := r.queries.SessionByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Session{}, fmt.Errorf("%s: %w", op, apperrors.ErrSessionNotFound)
		}

		return domain.Session{}, fmt.Errorf("%s: %w", op, err)
	}

	return domain.Session{
		ID:        session.ID,
		UserID:    session.UserID,
		CreatedAt: session.CreatedAt,
		ExpiresAt: session.ExpiresAt,
		RevokedAt: session.RevokedAt,
	}, nil
}

func (r *SessionRepository) RefreshTokenByHash(ctx context.Context, hash []byte) (domain.RefreshToken, error) {
	const op = "repository.SessionRepository.RefreshTokenByHash"

	token, err := r.queries.RefreshTokenByHash(ctx, hash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.RefreshToken{}, fmt.Errorf("%s: %w", op, apperrors.ErrRefreshTokenNotFound)
		}

		return domain.RefreshToken{}, fmt.Errorf("%s: %w", op, err)
	}

	return domain.RefreshToken{
		ID:        token.ID,
		SessionID: token.SessionID,
		TokenHash: token.TokenHash,
		CreatedAt: token.CreatedAt,
		ExpiresAt: token.ExpiresAt,
		UsedAt:    token.UsedAt,
	}, nil
}

func (r *SessionRepository) RotateRefreshToken(
	ctx context.Context,
	usedTokenID uuid.UUID,
	usedAt time.Time,
	next domain.RefreshToken,
) error {
	const op = "repository.SessionRepository.RotateRefreshToken"

	err := r.inTx(ctx, func(q *sqlc.Queries) error {
		marked, err := q.MarkRefreshTokenUsed(ctx, sqlc.MarkRefreshTokenUsedParams{ID: usedTokenID, UsedAt: &usedAt})
		if err != nil {
			return err
		}
		if marked == 0 {
			return apperrors.ErrRefreshTokenReused
		}

		if err := q.CreateRefreshToken(ctx, toCreateRefreshTokenParams(next)); err != nil {
			return err
		}

		return q.ExtendSession(ctx, sqlc.ExtendSessionParams{ID: next.SessionID, ExpiresAt: next.ExpiresAt})
	})
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}

func (r *SessionRepository) RevokeSession(ctx context.Context, id uuid.UUID, revokedAt time.Time) error {
	const op = "repository.SessionRepository.RevokeSession"

	if err := r.queries.RevokeSession(ctx, sqlc.RevokeSessionParams{ID: id, RevokedAt: &revokedAt}); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}

func (r *SessionRepository) RevokeUserSessions(ctx context.Context, userID uuid.UUID, revokedAt time.Time) error {
	const op = "repository.SessionRepository.RevokeUserSessions"

	if err := r.queries.RevokeUserSessions(ctx, sqlc.RevokeUserSessionsParams{
		UserID:    userID,
		RevokedAt: &revokedAt,
	}); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}

func (r *SessionRepository) RevokeUserSessionsExcept(
	ctx context.Context,
	userID, keepSessionID uuid.UUID,
	revokedAt time.Time,
) error {
	const op = "repository.SessionRepository.RevokeUserSessionsExcept"

	if err := r.queries.RevokeUserSessionsExcept(ctx, sqlc.RevokeUserSessionsExceptParams{
		UserID:        userID,
		KeepSessionID: keepSessionID,
		RevokedAt:     &revokedAt,
	}); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}

func (r *SessionRepository) DeleteExpired(
	ctx context.Context,
	now, usedBefore time.Time,
) (sessions, tokens int64, err error) {
	const op = "repository.SessionRepository.DeleteExpired"

	sessions, err = r.queries.DeleteExpiredSessions(ctx, now)
	if err != nil {
		return 0, 0, fmt.Errorf("%s: %w", op, err)
	}

	tokens, err = r.queries.DeleteStaleRefreshTokens(ctx, sqlc.DeleteStaleRefreshTokensParams{
		Now:        now,
		UsedBefore: &usedBefore,
	})
	if err != nil {
		return 0, 0, fmt.Errorf("%s: %w", op, err)
	}

	return sessions, tokens, nil
}

func (r *SessionRepository) inTx(ctx context.Context, fn func(q *sqlc.Queries) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := fn(r.queries.WithTx(tx)); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func toCreateRefreshTokenParams(token domain.RefreshToken) sqlc.CreateRefreshTokenParams {
	return sqlc.CreateRefreshTokenParams{
		ID:        token.ID,
		SessionID: token.SessionID,
		TokenHash: token.TokenHash,
		CreatedAt: token.CreatedAt,
		ExpiresAt: token.ExpiresAt,
	}
}

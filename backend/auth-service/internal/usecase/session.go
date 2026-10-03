package usecase

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"time"
	"uuid"

	"github.com/andrey1af/shop-api/backend/auth-service/internal/domain"
	apperrors "github.com/andrey1af/shop-api/backend/auth-service/internal/errors"
	"github.com/andrey1af/shop-api/backend/auth-service/internal/jwt"
)

const (
	refreshTokenBytes = 32

	usedRefreshTokenRetention = 7 * 24 * time.Hour
)

type SessionStore interface {
	CreateSession(ctx context.Context, session domain.Session, token domain.RefreshToken) error
	SessionByID(ctx context.Context, id uuid.UUID) (domain.Session, error)
	RefreshTokenByHash(ctx context.Context, hash []byte) (domain.RefreshToken, error)
	RotateRefreshToken(ctx context.Context, usedTokenID uuid.UUID, usedAt time.Time, next domain.RefreshToken) error
	RevokeSession(ctx context.Context, id uuid.UUID, revokedAt time.Time) error
	RevokeUserSessions(ctx context.Context, userID uuid.UUID, revokedAt time.Time) error
	RevokeUserSessionsExcept(ctx context.Context, userID, keepSessionID uuid.UUID, revokedAt time.Time) error
	DeleteExpired(ctx context.Context, now, usedBefore time.Time) (sessions, tokens int64, err error)
}

type TokenManager interface {
	NewAccessToken(user domain.User, sessionID uuid.UUID) (jwt.Token, error)
	Parse(accessToken string) (jwt.Claims, error)
}

type Sessions struct {
	log          *slog.Logger
	store        SessionStore
	userProvider UserProvider
	tokenManager TokenManager
	refreshTTL   time.Duration
	now          func() time.Time
}

func NewSessions(
	log *slog.Logger,
	store SessionStore,
	userProvider UserProvider,
	tokenManager TokenManager,
	refreshTTL time.Duration,
) *Sessions {
	return &Sessions{
		log:          log,
		store:        store,
		userProvider: userProvider,
		tokenManager: tokenManager,
		refreshTTL:   refreshTTL,
		now:          time.Now,
	}
}

func (s *Sessions) Start(ctx context.Context, user domain.User) (Token, error) {
	const op = "sessions.Start"

	now := s.now()
	session := domain.Session{
		ID:        uuid.New(),
		UserID:    user.ID,
		CreatedAt: now,
		ExpiresAt: now.Add(s.refreshTTL),
	}

	plainRefresh, refresh, err := s.newRefreshToken(session.ID, now)
	if err != nil {
		return Token{}, fmt.Errorf("%s: %w", op, err)
	}

	if err := s.store.CreateSession(ctx, session, refresh); err != nil {
		return Token{}, fmt.Errorf("%s: %w", op, err)
	}

	access, err := s.tokenManager.NewAccessToken(user, session.ID)
	if err != nil {
		return Token{}, fmt.Errorf("%s: %w", op, err)
	}

	return Token{
		AccessToken:      access.AccessToken,
		ExpiresAt:        access.ExpiresAt,
		RefreshToken:     plainRefresh,
		RefreshExpiresAt: refresh.ExpiresAt,
	}, nil
}

func (s *Sessions) Refresh(ctx context.Context, refreshToken string) (Token, error) {
	const op = "sessions.Refresh"
	log := s.log.With(slog.String("op", op))

	now := s.now()

	stored, err := s.store.RefreshTokenByHash(ctx, hashRefreshToken(refreshToken))
	if err != nil {
		if errors.Is(err, apperrors.ErrRefreshTokenNotFound) {
			log.Warn("unknown refresh token")

			return Token{}, fmt.Errorf("%s: %w", op, apperrors.ErrInvalidToken)
		}

		return Token{}, fmt.Errorf("%s: %w", op, err)
	}

	log = log.With(slog.String("session_id", stored.SessionID.String()))

	if stored.UsedAt != nil {
		s.revokeReusedSession(ctx, log, stored.SessionID, now)

		return Token{}, fmt.Errorf("%s: %w", op, apperrors.ErrInvalidToken)
	}

	if !now.Before(stored.ExpiresAt) {
		log.Warn("refresh token expired")

		return Token{}, fmt.Errorf("%s: %w", op, apperrors.ErrInvalidToken)
	}

	session, err := s.activeSession(ctx, stored.SessionID, now)
	if err != nil {
		log.Warn("refresh for inactive session", slog.String("error", err.Error()))

		return Token{}, fmt.Errorf("%s: %w", op, err)
	}

	user, err := s.userProvider.UserByID(ctx, session.UserID)
	if err != nil {
		return Token{}, fmt.Errorf("%s: %w", op, err)
	}

	plainRefresh, next, err := s.newRefreshToken(session.ID, now)
	if err != nil {
		return Token{}, fmt.Errorf("%s: %w", op, err)
	}

	if err := s.store.RotateRefreshToken(ctx, stored.ID, now, next); err != nil {

		if errors.Is(err, apperrors.ErrRefreshTokenReused) {
			s.revokeReusedSession(ctx, log, session.ID, now)

			return Token{}, fmt.Errorf("%s: %w", op, apperrors.ErrInvalidToken)
		}

		return Token{}, fmt.Errorf("%s: %w", op, err)
	}

	access, err := s.tokenManager.NewAccessToken(user, session.ID)
	if err != nil {
		return Token{}, fmt.Errorf("%s: %w", op, err)
	}

	return Token{
		AccessToken:      access.AccessToken,
		ExpiresAt:        access.ExpiresAt,
		RefreshToken:     plainRefresh,
		RefreshExpiresAt: next.ExpiresAt,
	}, nil
}

func (s *Sessions) Validate(ctx context.Context, accessToken string) (TokenClaims, error) {
	const op = "sessions.Validate"

	claims, err := s.tokenManager.Parse(accessToken)
	if err != nil {
		return TokenClaims{}, fmt.Errorf("%s: %w", op, err)
	}

	if _, err := s.activeSession(ctx, claims.SessionID, s.now()); err != nil {
		return TokenClaims{}, fmt.Errorf("%s: %w", op, err)
	}

	return TokenClaims{
		UserID:    claims.UserID,
		Email:     claims.Email,
		SessionID: claims.SessionID,
		ExpiresAt: claims.ExpiresAt,
	}, nil
}

func (s *Sessions) AccessToken(user domain.User, sessionID uuid.UUID) (Token, error) {
	const op = "sessions.AccessToken"

	access, err := s.tokenManager.NewAccessToken(user, sessionID)
	if err != nil {
		return Token{}, fmt.Errorf("%s: %w", op, err)
	}

	return Token{AccessToken: access.AccessToken, ExpiresAt: access.ExpiresAt}, nil
}

func (s *Sessions) RevokeByRefreshToken(ctx context.Context, refreshToken string) error {
	const op = "sessions.RevokeByRefreshToken"

	stored, err := s.store.RefreshTokenByHash(ctx, hashRefreshToken(refreshToken))
	if err != nil {
		if errors.Is(err, apperrors.ErrRefreshTokenNotFound) {
			return fmt.Errorf("%s: %w", op, apperrors.ErrInvalidToken)
		}

		return fmt.Errorf("%s: %w", op, err)
	}

	if err := s.store.RevokeSession(ctx, stored.SessionID, s.now()); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}

func (s *Sessions) RevokeByAccessToken(ctx context.Context, accessToken string) error {
	const op = "sessions.RevokeByAccessToken"

	claims, err := s.tokenManager.Parse(accessToken)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	if err := s.store.RevokeSession(ctx, claims.SessionID, s.now()); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}

func (s *Sessions) RevokeAll(ctx context.Context, userID uuid.UUID) error {
	const op = "sessions.RevokeAll"

	if err := s.store.RevokeUserSessions(ctx, userID, s.now()); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}

func (s *Sessions) RevokeAllExcept(ctx context.Context, userID, keepSessionID uuid.UUID) error {
	const op = "sessions.RevokeAllExcept"

	if err := s.store.RevokeUserSessionsExcept(ctx, userID, keepSessionID, s.now()); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}

func (s *Sessions) DeleteExpired(ctx context.Context) error {
	const op = "sessions.DeleteExpired"
	log := s.log.With(slog.String("op", op))

	now := s.now()

	sessions, tokens, err := s.store.DeleteExpired(ctx, now, now.Add(-usedRefreshTokenRetention))
	if err != nil {
		log.Error("failed to delete expired sessions", slog.String("error", err.Error()))

		return fmt.Errorf("%s: %w", op, err)
	}

	if sessions > 0 || tokens > 0 {
		log.Info("expired sessions deleted", slog.Int64("sessions", sessions), slog.Int64("refresh_tokens", tokens))
	}

	return nil
}

func (s *Sessions) activeSession(ctx context.Context, id uuid.UUID, now time.Time) (domain.Session, error) {
	session, err := s.store.SessionByID(ctx, id)
	if err != nil {
		if errors.Is(err, apperrors.ErrSessionNotFound) {
			return domain.Session{}, apperrors.ErrInvalidToken
		}

		return domain.Session{}, err
	}

	if !session.Active(now) {
		return domain.Session{}, apperrors.ErrInvalidToken
	}

	return session, nil
}

func (s *Sessions) revokeReusedSession(ctx context.Context, log *slog.Logger, sessionID uuid.UUID, now time.Time) {
	log.Warn("refresh token reuse detected, revoking session")

	if err := s.store.RevokeSession(ctx, sessionID, now); err != nil {
		log.Error("failed to revoke session after refresh token reuse", slog.String("error", err.Error()))
	}
}

func (s *Sessions) newRefreshToken(sessionID uuid.UUID, now time.Time) (string, domain.RefreshToken, error) {
	buf := make([]byte, refreshTokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", domain.RefreshToken{}, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	plain := base64.RawURLEncoding.EncodeToString(buf)

	return plain, domain.RefreshToken{
		ID:        uuid.New(),
		SessionID: sessionID,
		TokenHash: hashRefreshToken(plain),
		CreatedAt: now,
		ExpiresAt: now.Add(s.refreshTTL),
	}, nil
}

func hashRefreshToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

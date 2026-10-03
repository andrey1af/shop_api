package jwt

import (
	"fmt"
	"time"
	"uuid"

	"github.com/andrey1af/shop-api/backend/auth-service/internal/domain"
	"github.com/andrey1af/shop-api/backend/auth-service/internal/errors"
	"github.com/golang-jwt/jwt/v5"
)

type Token struct {
	AccessToken string
	ExpiresAt   time.Time
}

type Claims struct {
	UserID    uuid.UUID
	Email     string
	SessionID uuid.UUID
	ExpiresAt time.Time
}

type Manager struct {
	secret   []byte
	tokenTTL time.Duration
}

func NewManager(secret string, tokenTTL time.Duration) *Manager {
	return &Manager{
		secret:   []byte(secret),
		tokenTTL: tokenTTL,
	}
}

func (m *Manager) NewAccessToken(user domain.User, sessionID uuid.UUID) (Token, error) {
	const op = "jwt.Manager.NewAccessToken"

	now := time.Now()
	expiresAt := now.Add(m.tokenTTL)

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"id":    user.ID.String(),
		"email": user.Email,
		"sid":   sessionID.String(),
		"jti":   uuid.New().String(),
		"iat":   now.Unix(),
		"exp":   expiresAt.Unix(),
	})

	accessToken, err := token.SignedString(m.secret)
	if err != nil {
		return Token{}, fmt.Errorf("%s: %w", op, err)
	}

	return Token{
		AccessToken: accessToken,
		ExpiresAt:   expiresAt,
	}, nil
}

func (m *Manager) Parse(accessToken string) (Claims, error) {
	const op = "jwt.Manager.Parse"

	token, err := jwt.Parse(
		accessToken,
		func(*jwt.Token) (any, error) { return m.secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return Claims{}, fmt.Errorf("%s: %w: %w", op, errors.ErrInvalidToken, err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return Claims{}, fmt.Errorf("%s: %w: unexpected claims type %T", op, errors.ErrInvalidToken, token.Claims)
	}

	id, ok := claims["id"].(string)
	if !ok {
		return Claims{}, fmt.Errorf("%s: %w: claim id is missing or not a string", op, errors.ErrInvalidToken)
	}

	userID, err := uuid.Parse(id)
	if err != nil {
		return Claims{}, fmt.Errorf("%s: %w: %w", op, errors.ErrInvalidToken, err)
	}

	email, ok := claims["email"].(string)
	if !ok {
		return Claims{}, fmt.Errorf("%s: %w: claim email is missing or not a string", op, errors.ErrInvalidToken)
	}

	sid, ok := claims["sid"].(string)
	if !ok {
		return Claims{}, fmt.Errorf("%s: %w: claim sid is missing or not a string", op, errors.ErrInvalidToken)
	}

	sessionID, err := uuid.Parse(sid)
	if err != nil {
		return Claims{}, fmt.Errorf("%s: %w: %w", op, errors.ErrInvalidToken, err)
	}

	expiresAt, err := claims.GetExpirationTime()
	if err != nil {
		return Claims{}, fmt.Errorf("%s: %w: %w", op, errors.ErrInvalidToken, err)
	}
	if expiresAt == nil {
		return Claims{}, fmt.Errorf("%s: %w: claim exp is missing", op, errors.ErrInvalidToken)
	}

	return Claims{
		UserID:    userID,
		Email:     email,
		SessionID: sessionID,
		ExpiresAt: expiresAt.Time,
	}, nil
}

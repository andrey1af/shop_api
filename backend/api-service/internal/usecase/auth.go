package usecase

import (
	"context"
	"errors"
	"log/slog"
	"time"
	"uuid"

	apperrors "github.com/andrey1af/shop-api/backend/api-service/internal/errors"
)

type RegisterUserInput struct {
	Name        string
	Surname     string
	Email       string
	PhoneNumber string
	Password    string
}

type Token struct {
	AccessToken      string
	ExpiresAt        time.Time
	RefreshToken     string
	RefreshExpiresAt time.Time
}

type RegisterUserOutput struct {
	UserID uuid.UUID
	Token  Token
}

type LoginInput struct {
	Email    string
	Password string
}

type LoginOutput struct {
	UserID uuid.UUID
	Token  Token
}

type ChangePasswordInput struct {
	AccessToken string
	OldPassword string
	NewPassword string
}

type TokenClaims struct {
	UserID    uuid.UUID
	Email     string
	SessionID uuid.UUID
	ExpiresAt time.Time
}

type LogoutInput struct {
	RefreshToken string
	AccessToken  string
}

type OAuthCallbackInput struct {
	Code  string
	State string
}

type OAuthLoginOutput struct {
	UserID uuid.UUID
	Email  string
	Token  Token
}

type AuthProvider interface {
	Register(ctx context.Context, in RegisterUserInput) (RegisterUserOutput, error)
	Login(ctx context.Context, in LoginInput) (LoginOutput, error)
	ChangePassword(ctx context.Context, in ChangePasswordInput) (Token, error)
	ResetPassword(ctx context.Context, email string) error
	ValidateToken(ctx context.Context, accessToken string) (TokenClaims, error)
	Refresh(ctx context.Context, refreshToken string) (Token, error)
	Logout(ctx context.Context, in LogoutInput) error
	LogoutAll(ctx context.Context, accessToken string) error
	OAuthStart(ctx context.Context, provider string) (string, error)
	OAuthCallback(ctx context.Context, in OAuthCallbackInput) (OAuthLoginOutput, error)
}

type AuthUseCase struct {
	log          *slog.Logger
	authProvider AuthProvider
}

func NewAuthUseCase(log *slog.Logger, authProvider AuthProvider) *AuthUseCase {
	return &AuthUseCase{
		log:          log,
		authProvider: authProvider,
	}
}

func (uc *AuthUseCase) Register(ctx context.Context, in RegisterUserInput) (RegisterUserOutput, error) {
	const op = "authUseCase.Register"
	log := uc.log.With(slog.String("op", op))

	log.Info("register user")

	out, err := uc.authProvider.Register(ctx, in)
	if err != nil {
		log.Error("failed to register user", slog.String("error", err.Error()))

		return RegisterUserOutput{}, err
	}

	log.Info("user registered", slog.String("user_id", out.UserID.String()))

	return out, nil
}

func (uc *AuthUseCase) Login(ctx context.Context, in LoginInput) (LoginOutput, error) {
	const op = "authUseCase.Login"
	log := uc.log.With(slog.String("op", op))

	log.Info("login user")

	out, err := uc.authProvider.Login(ctx, in)
	if err != nil {
		if errors.Is(err, apperrors.ErrInvalidCredentials) {
			log.Warn("invalid credentials")

			return LoginOutput{}, err
		}

		log.Error("failed to login user", slog.String("error", err.Error()))

		return LoginOutput{}, err
	}

	log.Info("user logged in", slog.String("user_id", out.UserID.String()))

	return out, nil
}

func (uc *AuthUseCase) ChangePassword(ctx context.Context, in ChangePasswordInput) (Token, error) {
	const op = "authUseCase.ChangePassword"
	log := uc.log.With(slog.String("op", op))

	log.Info("change user password")

	token, err := uc.authProvider.ChangePassword(ctx, in)
	if err != nil {
		if errors.Is(err, apperrors.ErrInvalidCredentials) || errors.Is(err, apperrors.ErrUserNotFound) {
			log.Warn("failed to change user password", slog.String("error", err.Error()))

			return Token{}, err
		}

		log.Error("failed to change user password", slog.String("error", err.Error()))

		return Token{}, err
	}

	return token, nil
}

func (uc *AuthUseCase) ResetPassword(ctx context.Context, email string) error {
	const op = "authUseCase.ResetPassword"
	log := uc.log.With(slog.String("op", op))

	log.Info("reset user password")

	if err := uc.authProvider.ResetPassword(ctx, email); err != nil {
		if errors.Is(err, apperrors.ErrUserNotFound) {
			log.Warn("user not found")

			return nil
		}

		log.Error("failed to reset user password", slog.String("error", err.Error()))

		return err
	}

	return nil
}

func (uc *AuthUseCase) ValidateToken(ctx context.Context, accessToken string) (TokenClaims, error) {
	const op = "authUseCase.ValidateToken"
	log := uc.log.With(slog.String("op", op))

	claims, err := uc.authProvider.ValidateToken(ctx, accessToken)
	if err != nil {
		if errors.Is(err, apperrors.ErrInvalidToken) {
			log.Warn("invalid access token")

			return TokenClaims{}, err
		}

		log.Error("failed to validate access token", slog.String("error", err.Error()))

		return TokenClaims{}, err
	}

	return claims, nil
}

func (uc *AuthUseCase) OAuthStart(ctx context.Context, provider string) (string, error) {
	const op = "authUseCase.OAuthStart"
	log := uc.log.With(slog.String("op", op), slog.String("provider", provider))

	log.Info("start oauth login")

	authURL, err := uc.authProvider.OAuthStart(ctx, provider)
	if err != nil {
		if errors.Is(err, apperrors.ErrInvalidAuthRequest) {
			log.Warn("unsupported oauth provider")

			return "", err
		}

		log.Error("failed to start oauth login", slog.String("error", err.Error()))

		return "", err
	}

	return authURL, nil
}

func (uc *AuthUseCase) OAuthCallback(ctx context.Context, in OAuthCallbackInput) (OAuthLoginOutput, error) {
	const op = "authUseCase.OAuthCallback"
	log := uc.log.With(slog.String("op", op))

	log.Info("finish oauth login")

	out, err := uc.authProvider.OAuthCallback(ctx, in)
	if err != nil {
		if errors.Is(err, apperrors.ErrOAuthFailed) ||
			errors.Is(err, apperrors.ErrUserAlreadyExists) ||
			errors.Is(err, apperrors.ErrInvalidAuthRequest) {
			log.Warn("oauth login rejected", slog.String("error", err.Error()))

			return OAuthLoginOutput{}, err
		}

		log.Error("failed to finish oauth login", slog.String("error", err.Error()))

		return OAuthLoginOutput{}, err
	}

	log.Info("user logged in via oauth", slog.String("user_id", out.UserID.String()))

	return out, nil
}

func (uc *AuthUseCase) Refresh(ctx context.Context, refreshToken string) (Token, error) {
	const op = "authUseCase.Refresh"
	log := uc.log.With(slog.String("op", op))

	token, err := uc.authProvider.Refresh(ctx, refreshToken)
	if err != nil {
		if errors.Is(err, apperrors.ErrInvalidToken) {
			log.Warn("invalid refresh token")

			return Token{}, err
		}

		log.Error("failed to refresh tokens", slog.String("error", err.Error()))

		return Token{}, err
	}

	return token, nil
}

func (uc *AuthUseCase) Logout(ctx context.Context, in LogoutInput) error {
	const op = "authUseCase.Logout"
	log := uc.log.With(slog.String("op", op))

	log.Info("logout")

	if err := uc.authProvider.Logout(ctx, in); err != nil {
		if errors.Is(err, apperrors.ErrInvalidToken) {
			log.Warn("logout with invalid token")

			return err
		}

		log.Error("failed to logout", slog.String("error", err.Error()))

		return err
	}

	return nil
}

func (uc *AuthUseCase) LogoutAll(ctx context.Context, accessToken string) error {
	const op = "authUseCase.LogoutAll"
	log := uc.log.With(slog.String("op", op))

	log.Info("logout from all sessions")

	if err := uc.authProvider.LogoutAll(ctx, accessToken); err != nil {
		if errors.Is(err, apperrors.ErrInvalidToken) {
			log.Warn("logout all with invalid token")

			return err
		}

		log.Error("failed to logout from all sessions", slog.String("error", err.Error()))

		return err
	}

	return nil
}

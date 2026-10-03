package usecase

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"time"
	"unicode/utf8"
	"uuid"

	"github.com/andrey1af/shop-api/backend/auth-service/internal/domain"
	apperrors "github.com/andrey1af/shop-api/backend/auth-service/internal/errors"
	"golang.org/x/crypto/bcrypt"
)

type Token struct {
	AccessToken      string
	ExpiresAt        time.Time
	RefreshToken     string
	RefreshExpiresAt time.Time
}

type TokenClaims struct {
	UserID    uuid.UUID
	Email     string
	SessionID uuid.UUID
	ExpiresAt time.Time
}

type RegisterUserInput struct {
	Name        string
	Surname     string
	Email       string
	PhoneNumber string
	Password    string
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

type LogoutInput struct {
	RefreshToken string
	AccessToken  string
}

type UserProvider interface {
	UserByID(ctx context.Context, id uuid.UUID) (domain.User, error)
	UserByEmail(ctx context.Context, email string) (domain.User, error)
}

type UserSaver interface {
	SaveUser(ctx context.Context, user domain.User) (domain.User, error)
	UpdatePassword(ctx context.Context, userID uuid.UUID, passwordHash []byte) error
}

type AuthUseCase struct {
	log          *slog.Logger
	userProvider UserProvider
	userSaver    UserSaver
	sessions     *Sessions
}

func NewAuthUseCase(log *slog.Logger, userProvider UserProvider, userSaver UserSaver, sessions *Sessions) *AuthUseCase {
	return &AuthUseCase{
		log:          log,
		userProvider: userProvider,
		userSaver:    userSaver,
		sessions:     sessions,
	}
}

func (uc *AuthUseCase) Register(ctx context.Context, in RegisterUserInput) (RegisterUserOutput, error) {
	const op = "authUseCase.RegisterNewUser"
	log := uc.log.With(slog.String("op", op))

	log.Info("register new user")

	if err := checkPasswordPolicy(in.Password); err != nil {
		log.Warn("weak password")

		return RegisterUserOutput{}, fmt.Errorf("%s: %w", op, err)
	}

	pass, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Error("failed to generate password hash", slog.String("error", err.Error()))

		return RegisterUserOutput{}, fmt.Errorf("%s: %w", op, err)
	}

	user := domain.User{
		ID:           uuid.New(),
		Name:         in.Name,
		Surname:      in.Surname,
		Email:        in.Email,
		PhoneNumber:  in.PhoneNumber,
		PasswordHash: pass,
		CreatedAt:    time.Now(),
	}

	savedUser, err := uc.userSaver.SaveUser(ctx, user)
	if err != nil {
		if errors.Is(err, apperrors.ErrUserAlreadyExists) {
			log.Warn("user already exists")

			return RegisterUserOutput{}, fmt.Errorf("%s: %w", op, err)
		}

		log.Error("failed to save user", slog.String("error", err.Error()))

		return RegisterUserOutput{}, fmt.Errorf("%s: %w", op, err)
	}

	token, err := uc.sessions.Start(ctx, savedUser)
	if err != nil {
		log.Error("failed to start session", slog.String("error", err.Error()))

		return RegisterUserOutput{}, fmt.Errorf("%s: %w", op, err)
	}

	log.Info("user registered", slog.String("user_id", savedUser.ID.String()))

	return RegisterUserOutput{
		UserID: savedUser.ID,
		Token:  token,
	}, nil
}

func (uc *AuthUseCase) Login(ctx context.Context, in LoginInput) (LoginOutput, error) {
	const op = "authUseCase.Login"
	log := uc.log.With(slog.String("op", op))

	log.Info("login user")

	user, err := uc.userProvider.UserByEmail(ctx, in.Email)
	if err != nil {
		if errors.Is(err, apperrors.ErrUserNotFound) {
			log.Warn("user not found")

			return LoginOutput{}, fmt.Errorf("%s: %w", op, apperrors.ErrInvalidCredentials)
		}

		log.Error("failed to get user", slog.String("error", err.Error()))

		return LoginOutput{}, fmt.Errorf("%s: %w", op, err)
	}

	if err := bcrypt.CompareHashAndPassword(user.PasswordHash, []byte(in.Password)); err != nil {
		log.Warn("invalid password", slog.String("user_id", user.ID.String()))

		return LoginOutput{}, fmt.Errorf("%s: %w", op, apperrors.ErrInvalidCredentials)
	}

	token, err := uc.sessions.Start(ctx, user)
	if err != nil {
		log.Error("failed to start session", slog.String("error", err.Error()))

		return LoginOutput{}, fmt.Errorf("%s: %w", op, err)
	}

	log.Info("user logged in", slog.String("user_id", user.ID.String()))

	return LoginOutput{
		UserID: user.ID,
		Token:  token,
	}, nil
}

func (uc *AuthUseCase) ChangePassword(ctx context.Context, in ChangePasswordInput) (Token, error) {
	const op = "authUseCase.ChangePassword"
	log := uc.log.With(slog.String("op", op))

	log.Info("change user password")

	claims, err := uc.sessions.Validate(ctx, in.AccessToken)
	if err != nil {
		log.Warn("invalid access token", slog.String("error", err.Error()))

		return Token{}, fmt.Errorf("%s: %w", op, err)
	}

	user, err := uc.userProvider.UserByID(ctx, claims.UserID)
	if err != nil {
		if errors.Is(err, apperrors.ErrUserNotFound) {
			log.Warn("user not found", slog.String("user_id", claims.UserID.String()))

			return Token{}, fmt.Errorf("%s: %w", op, err)
		}

		log.Error("failed to get user", slog.String("error", err.Error()))

		return Token{}, fmt.Errorf("%s: %w", op, err)
	}

	if err := bcrypt.CompareHashAndPassword(user.PasswordHash, []byte(in.OldPassword)); err != nil {
		log.Warn("invalid old password", slog.String("user_id", user.ID.String()))

		return Token{}, fmt.Errorf("%s: %w", op, apperrors.ErrInvalidCredentials)
	}

	if err := checkPasswordPolicy(in.NewPassword); err != nil {
		log.Warn("weak new password", slog.String("user_id", user.ID.String()))

		return Token{}, fmt.Errorf("%s: %w", op, err)
	}

	pass, err := bcrypt.GenerateFromPassword([]byte(in.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		log.Error("failed to generate password hash", slog.String("error", err.Error()))

		return Token{}, fmt.Errorf("%s: %w", op, err)
	}

	if err := uc.userSaver.UpdatePassword(ctx, user.ID, pass); err != nil {
		log.Error("failed to update password", slog.String("error", err.Error()))

		return Token{}, fmt.Errorf("%s: %w", op, err)
	}

	if err := uc.sessions.RevokeAllExcept(ctx, user.ID, claims.SessionID); err != nil {
		log.Error("failed to revoke other sessions", slog.String("error", err.Error()))

		return Token{}, fmt.Errorf("%s: %w", op, err)
	}

	token, err := uc.sessions.AccessToken(user, claims.SessionID)
	if err != nil {
		log.Error("failed to generate access token", slog.String("error", err.Error()))

		return Token{}, fmt.Errorf("%s: %w", op, err)
	}

	log.Info("user password changed", slog.String("user_id", user.ID.String()))

	return token, nil
}

func (uc *AuthUseCase) ResetPassword(ctx context.Context, email string) error {
	const op = "authUseCase.ResetPassword"
	const temporaryPasswordBytes = 12

	log := uc.log.With(slog.String("op", op))

	log.Info("reset user password")

	user, err := uc.userProvider.UserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, apperrors.ErrUserNotFound) {
			log.Warn("password reset requested for unknown email")

			return nil
		}

		log.Error("failed to get user", slog.String("error", err.Error()))

		return fmt.Errorf("%s: %w", op, err)
	}

	buf := make([]byte, temporaryPasswordBytes)
	if _, err := rand.Read(buf); err != nil {
		log.Error("failed to generate temporary password", slog.String("error", err.Error()))

		return fmt.Errorf("%s: %w", op, err)
	}

	temporaryPassword := base64.RawURLEncoding.EncodeToString(buf)

	pass, err := bcrypt.GenerateFromPassword([]byte(temporaryPassword), bcrypt.DefaultCost)
	if err != nil {
		log.Error("failed to generate password hash", slog.String("error", err.Error()))

		return fmt.Errorf("%s: %w", op, err)
	}

	if err := uc.userSaver.UpdatePassword(ctx, user.ID, pass); err != nil {
		log.Error("failed to update password", slog.String("error", err.Error()))

		return fmt.Errorf("%s: %w", op, err)
	}

	if err := uc.sessions.RevokeAll(ctx, user.ID); err != nil {
		log.Error("failed to revoke user sessions", slog.String("error", err.Error()))

		return fmt.Errorf("%s: %w", op, err)
	}

	log.Warn("temporary password generated",
		slog.String("user_id", user.ID.String()),
		slog.String("temporary_password", temporaryPassword),
	)

	return nil
}

func (uc *AuthUseCase) ValidateToken(ctx context.Context, accessToken string) (TokenClaims, error) {
	const op = "authUseCase.ValidateToken"
	log := uc.log.With(slog.String("op", op))

	claims, err := uc.sessions.Validate(ctx, accessToken)
	if err != nil {
		if errors.Is(err, apperrors.ErrInvalidToken) {
			log.Warn("invalid access token", slog.String("error", err.Error()))
		} else {
			log.Error("failed to validate access token", slog.String("error", err.Error()))
		}

		return TokenClaims{}, fmt.Errorf("%s: %w", op, err)
	}

	return claims, nil
}

func (uc *AuthUseCase) Refresh(ctx context.Context, refreshToken string) (Token, error) {
	const op = "authUseCase.Refresh"
	log := uc.log.With(slog.String("op", op))

	token, err := uc.sessions.Refresh(ctx, refreshToken)
	if err != nil {
		if !errors.Is(err, apperrors.ErrInvalidToken) {
			log.Error("failed to refresh tokens", slog.String("error", err.Error()))
		}

		return Token{}, fmt.Errorf("%s: %w", op, err)
	}

	return token, nil
}

func (uc *AuthUseCase) Logout(ctx context.Context, in LogoutInput) error {
	const op = "authUseCase.Logout"
	log := uc.log.With(slog.String("op", op))

	var err error
	switch {
	case in.RefreshToken != "":
		err = uc.sessions.RevokeByRefreshToken(ctx, in.RefreshToken)
	case in.AccessToken != "":
		err = uc.sessions.RevokeByAccessToken(ctx, in.AccessToken)
	default:
		err = apperrors.ErrInvalidToken
	}
	if err != nil {
		if errors.Is(err, apperrors.ErrInvalidToken) {
			log.Warn("logout with invalid token")
		} else {
			log.Error("failed to revoke session", slog.String("error", err.Error()))
		}

		return fmt.Errorf("%s: %w", op, err)
	}

	log.Info("session revoked")

	return nil
}

func (uc *AuthUseCase) LogoutAll(ctx context.Context, accessToken string) error {
	const op = "authUseCase.LogoutAll"
	log := uc.log.With(slog.String("op", op))

	claims, err := uc.sessions.Validate(ctx, accessToken)
	if err != nil {
		log.Warn("invalid access token", slog.String("error", err.Error()))

		return fmt.Errorf("%s: %w", op, err)
	}

	if err := uc.sessions.RevokeAll(ctx, claims.UserID); err != nil {
		log.Error("failed to revoke user sessions", slog.String("error", err.Error()))

		return fmt.Errorf("%s: %w", op, err)
	}

	log.Info("all user sessions revoked", slog.String("user_id", claims.UserID.String()))

	return nil
}

const MinPasswordLength = 8

func checkPasswordPolicy(password string) error {
	if utf8.RuneCountInString(password) < MinPasswordLength {
		return apperrors.ErrWeakPassword
	}

	return nil
}

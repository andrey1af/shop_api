package usecase

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"time"
	"uuid"

	"github.com/andrey1af/shop-api/backend/auth-service/internal/domain"
	apperrors "github.com/andrey1af/shop-api/backend/auth-service/internal/errors"
	"github.com/andrey1af/shop-api/backend/auth-service/pkg/validation"
)

const oauthRandomBytes = 32

type OAuthLoginOutput struct {
	UserID uuid.UUID
	Email  string
	Token  Token
}

type OAuthProvider interface {
	AuthCodeURL(state, codeVerifier string) string
	Exchange(ctx context.Context, code, codeVerifier string) (domain.OAuthProfile, error)
}

type OAuthUserStore interface {
	UserByIdentity(ctx context.Context, provider, providerUserID string) (domain.User, error)
	UserByEmail(ctx context.Context, email string) (domain.User, error)
	SaveUserWithIdentity(ctx context.Context, user domain.User, identity domain.UserIdentity) (domain.User, error)
	SaveIdentity(ctx context.Context, identity domain.UserIdentity) error
}

type OAuthStateStore interface {
	SaveState(ctx context.Context, state domain.OAuthState) error
	ConsumeState(ctx context.Context, state string) (domain.OAuthState, error)
	DeleteExpiredStates(ctx context.Context, now time.Time) error
}

type OAuthUseCase struct {
	log        *slog.Logger
	providers  map[string]OAuthProvider
	userStore  OAuthUserStore
	stateStore OAuthStateStore
	sessions   *Sessions
	stateTTL   time.Duration
	now        func() time.Time
}

func NewOAuthUseCase(
	log *slog.Logger,
	providers map[string]OAuthProvider,
	userStore OAuthUserStore,
	stateStore OAuthStateStore,
	sessions *Sessions,
	stateTTL time.Duration,
) *OAuthUseCase {
	return &OAuthUseCase{
		log:        log,
		providers:  providers,
		userStore:  userStore,
		stateStore: stateStore,
		sessions:   sessions,
		stateTTL:   stateTTL,
		now:        time.Now,
	}
}

func (uc *OAuthUseCase) Start(ctx context.Context, providerName string) (string, error) {
	const op = "oauthUseCase.Start"
	log := uc.log.With(slog.String("op", op), slog.String("provider", providerName))

	log.Info("start oauth login")

	provider, ok := uc.providers[providerName]
	if !ok {
		log.Warn("unsupported oauth provider")

		return "", fmt.Errorf("%s: %w", op, apperrors.ErrUnsupportedOAuthProvider)
	}

	now := uc.now()

	if err := uc.stateStore.DeleteExpiredStates(ctx, now); err != nil {
		log.Error("failed to delete expired oauth states", slog.String("error", err.Error()))
	}

	state, err := randomToken()
	if err != nil {
		log.Error("failed to generate oauth state", slog.String("error", err.Error()))

		return "", fmt.Errorf("%s: %w", op, err)
	}

	codeVerifier, err := randomToken()
	if err != nil {
		log.Error("failed to generate pkce code verifier", slog.String("error", err.Error()))

		return "", fmt.Errorf("%s: %w", op, err)
	}

	err = uc.stateStore.SaveState(ctx, domain.OAuthState{
		State:        state,
		Provider:     providerName,
		CodeVerifier: codeVerifier,
		ExpiresAt:    now.Add(uc.stateTTL),
	})
	if err != nil {
		log.Error("failed to save oauth state", slog.String("error", err.Error()))

		return "", fmt.Errorf("%s: %w", op, err)
	}

	return provider.AuthCodeURL(state, codeVerifier), nil
}

func (uc *OAuthUseCase) Callback(ctx context.Context, code, state string) (OAuthLoginOutput, error) {
	const op = "oauthUseCase.Callback"
	log := uc.log.With(slog.String("op", op))

	log.Info("finish oauth login")

	savedState, err := uc.stateStore.ConsumeState(ctx, state)
	if err != nil {
		if errors.Is(err, apperrors.ErrInvalidOAuthState) {
			log.Warn("unknown oauth state")

			return OAuthLoginOutput{}, fmt.Errorf("%s: %w", op, err)
		}

		log.Error("failed to consume oauth state", slog.String("error", err.Error()))

		return OAuthLoginOutput{}, fmt.Errorf("%s: %w", op, err)
	}

	log = log.With(slog.String("provider", savedState.Provider))

	if !uc.now().Before(savedState.ExpiresAt) {
		log.Warn("oauth state expired")

		return OAuthLoginOutput{}, fmt.Errorf("%s: %w", op, apperrors.ErrInvalidOAuthState)
	}

	provider, ok := uc.providers[savedState.Provider]
	if !ok {
		log.Warn("unsupported oauth provider")

		return OAuthLoginOutput{}, fmt.Errorf("%s: %w", op, apperrors.ErrUnsupportedOAuthProvider)
	}

	profile, err := provider.Exchange(ctx, code, savedState.CodeVerifier)
	if err != nil {
		if errors.Is(err, apperrors.ErrInvalidOAuthCode) {
			log.Warn("invalid oauth authorization code")

			return OAuthLoginOutput{}, fmt.Errorf("%s: %w", op, err)
		}

		log.Error("failed to exchange oauth authorization code", slog.String("error", err.Error()))

		return OAuthLoginOutput{}, fmt.Errorf("%s: %w", op, err)
	}

	if !validation.IsValidEmail(profile.Email) {
		log.Warn("oauth profile has no valid email")

		return OAuthLoginOutput{}, fmt.Errorf("%s: %w", op, apperrors.ErrOAuthEmailRequired)
	}

	user, err := uc.resolveUser(ctx, log, savedState.Provider, profile)
	if err != nil {
		return OAuthLoginOutput{}, fmt.Errorf("%s: %w", op, err)
	}

	token, err := uc.sessions.Start(ctx, user)
	if err != nil {
		log.Error("failed to start session", slog.String("error", err.Error()))

		return OAuthLoginOutput{}, fmt.Errorf("%s: %w", op, err)
	}

	log.Info("user logged in via oauth", slog.String("user_id", user.ID.String()))

	return OAuthLoginOutput{
		UserID: user.ID,
		Email:  user.Email,
		Token:  token,
	}, nil
}

func (uc *OAuthUseCase) resolveUser(
	ctx context.Context,
	log *slog.Logger,
	providerName string,
	profile domain.OAuthProfile,
) (domain.User, error) {
	user, err := uc.userStore.UserByIdentity(ctx, providerName, profile.ProviderUserID)
	if err == nil {
		return user, nil
	}
	if !errors.Is(err, apperrors.ErrUserNotFound) {
		log.Error("failed to get user by identity", slog.String("error", err.Error()))

		return domain.User{}, err
	}

	now := uc.now()
	identity := domain.UserIdentity{
		ID:             uuid.New(),
		Provider:       providerName,
		ProviderUserID: profile.ProviderUserID,
		Email:          profile.Email,
		CreatedAt:      now,
	}

	user, err = uc.userStore.UserByEmail(ctx, profile.Email)
	switch {
	case err == nil:

		if !profile.EmailVerified {
			log.Warn("user with this email exists, oauth email is not verified")

			return domain.User{}, apperrors.ErrUserAlreadyExists
		}

		identity.UserID = user.ID
		if err := uc.userStore.SaveIdentity(ctx, identity); err != nil {
			log.Error("failed to link oauth identity", slog.String("error", err.Error()))

			return domain.User{}, err
		}

		log.Info("oauth identity linked to existing user", slog.String("user_id", user.ID.String()))

		return user, nil
	case !errors.Is(err, apperrors.ErrUserNotFound):
		log.Error("failed to get user by email", slog.String("error", err.Error()))

		return domain.User{}, err
	}

	phoneNumber := profile.PhoneNumber
	if !validation.IsValidPhoneNumber(phoneNumber) {
		phoneNumber = ""
	}

	user, err = uc.userStore.SaveUserWithIdentity(ctx, domain.User{
		ID:          uuid.New(),
		Name:        profile.Name,
		Surname:     profile.Surname,
		Email:       profile.Email,
		PhoneNumber: phoneNumber,
		CreatedAt:   now,
	}, identity)
	if err != nil {
		if errors.Is(err, apperrors.ErrUserAlreadyExists) {
			log.Warn("user already exists")

			return domain.User{}, err
		}

		log.Error("failed to save oauth user", slog.String("error", err.Error()))

		return domain.User{}, err
	}

	log.Info("user registered via oauth", slog.String("user_id", user.ID.String()))

	return user, nil
}

func randomToken() (string, error) {
	buf := make([]byte, oauthRandomBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(buf), nil
}

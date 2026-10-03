package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/andrey1af/shop-api/backend/auth-service/internal/domain"
	apperrors "github.com/andrey1af/shop-api/backend/auth-service/internal/errors"
)

const testProvider = "yandex"

type oauthFixture struct {
	*authFixture
	oauth    *OAuthUseCase
	states   *memoryStateStore
	provider *fakeOAuthProvider
}

func newOAuthFixture(t *testing.T, profile domain.OAuthProfile) *oauthFixture {
	t.Helper()

	f := &oauthFixture{
		authFixture: newAuthFixture(t),
		states:      newMemoryStateStore(),
		provider:    &fakeOAuthProvider{profile: profile},
	}
	f.oauth = NewOAuthUseCase(
		newTestLogger(),
		map[string]OAuthProvider{testProvider: f.provider},
		f.users,
		f.states,
		f.sessions,
		10*time.Minute,
	)
	f.oauth.now = func() time.Time { return f.now }

	return f
}

func (f *oauthFixture) start(t *testing.T) string {
	t.Helper()
	if _, err := f.oauth.Start(context.Background(), testProvider); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	return f.provider.gotState
}

func newProfile() domain.OAuthProfile {
	return domain.OAuthProfile{
		ProviderUserID: "ya-42",
		Email:          "oauth@example.com",
		EmailVerified:  true,
		Name:           "Anna",
		Surname:        "Smirnova",
		PhoneNumber:    "+79990001122",
	}
}

func TestOAuthUseCase_Start(t *testing.T) {
	t.Run("saves state with pkce verifier and builds provider url", func(t *testing.T) {
		f := newOAuthFixture(t, newProfile())

		authURL, err := f.oauth.Start(context.Background(), testProvider)
		if err != nil {
			t.Fatalf("Start() error = %v", err)
		}

		saved, ok := f.states.states[f.provider.gotState]
		if !ok {
			t.Fatal("state not saved")
		}
		if saved.Provider != testProvider || saved.CodeVerifier != f.provider.gotVerifier {
			t.Errorf("saved state = %+v", saved)
		}
		if !saved.ExpiresAt.Equal(f.now.Add(10 * time.Minute)) {
			t.Errorf("state expires at %v, want now+10m", saved.ExpiresAt)
		}

		if len(saved.CodeVerifier) < 43 || len(saved.State) < 43 {
			t.Errorf("state/verifier too short: %d/%d", len(saved.State), len(saved.CodeVerifier))
		}
		if !strings.Contains(authURL, saved.State) {
			t.Errorf("auth url %q has no state", authURL)
		}
		if f.states.deleteCalls != 1 || !f.states.deletedBefore.Equal(f.now) {
			t.Error("expired states were not cleaned up")
		}
	})

	t.Run("cleanup failure does not break login", func(t *testing.T) {
		f := newOAuthFixture(t, newProfile())
		f.states.deleteErr = errors.New("db down")

		if _, err := f.oauth.Start(context.Background(), testProvider); err != nil {
			t.Fatalf("Start() error = %v", err)
		}
	})

	t.Run("unknown provider", func(t *testing.T) {
		f := newOAuthFixture(t, newProfile())
		if _, err := f.oauth.Start(context.Background(), "github"); !errors.Is(err, apperrors.ErrUnsupportedOAuthProvider) {
			t.Fatalf("Start() error = %v, want ErrUnsupportedOAuthProvider", err)
		}
	})
}

func TestOAuthUseCase_Callback_NewUser(t *testing.T) {
	profile := newProfile()
	profile.PhoneNumber = "8 999 000-11-22"
	f := newOAuthFixture(t, profile)
	state := f.start(t)
	verifier := f.provider.gotVerifier

	out, err := f.oauth.Callback(context.Background(), "auth-code", state)
	if err != nil {
		t.Fatalf("Callback() error = %v", err)
	}

	if f.provider.gotCode != "auth-code" || f.provider.gotVerifier != verifier {
		t.Error("code was exchanged without the saved pkce verifier")
	}

	user := f.users.users[out.UserID]
	if user.Email != profile.Email || user.Name != "Anna" || user.PasswordHash != nil || user.PhoneNumber != "" {
		t.Errorf("created user = %+v", user)
	}
	if len(f.users.identities) != 1 || f.users.identities[0].UserID != out.UserID {
		t.Errorf("identities = %+v", f.users.identities)
	}
	_, err = f.auth.ValidateToken(context.Background(), out.Token.AccessToken)
	if err != nil || out.Token.RefreshToken == "" {
		t.Errorf("oauth login did not open a session: %v", err)
	}

	_, err = f.oauth.Callback(context.Background(), "auth-code", state)
	if !errors.Is(err, apperrors.ErrInvalidOAuthState) {
		t.Errorf("second Callback() error = %v, want ErrInvalidOAuthState", err)
	}
}

func TestOAuthUseCase_Callback_ExistingIdentity(t *testing.T) {
	f := newOAuthFixture(t, newProfile())
	first, err := f.oauth.Callback(context.Background(), "code", f.start(t))
	if err != nil {
		t.Fatal(err)
	}

	second, err := f.oauth.Callback(context.Background(), "code", f.start(t))
	if err != nil {
		t.Fatalf("Callback() error = %v", err)
	}
	if second.UserID != first.UserID || len(f.users.users) != 1 {
		t.Errorf("repeat oauth login created a new user")
	}
}

func TestOAuthUseCase_Callback_ExistingEmail(t *testing.T) {
	t.Run("verified email links identity to existing user", func(t *testing.T) {
		f := newOAuthFixture(t, newProfile())
		registered := f.register(t, "oauth@example.com", testPassword)

		out, err := f.oauth.Callback(context.Background(), "code", f.start(t))
		if err != nil {
			t.Fatalf("Callback() error = %v", err)
		}
		if out.UserID != registered.UserID {
			t.Errorf("logged in as %v, want existing user %v", out.UserID, registered.UserID)
		}
		if len(f.users.identities) != 1 || f.users.identities[0].UserID != registered.UserID {
			t.Errorf("identity not linked: %+v", f.users.identities)
		}
	})

	t.Run("unverified email is not linked", func(t *testing.T) {
		profile := newProfile()
		profile.EmailVerified = false
		f := newOAuthFixture(t, profile)
		f.register(t, "oauth@example.com", testPassword)

		_, err := f.oauth.Callback(context.Background(), "code", f.start(t))
		if !errors.Is(err, apperrors.ErrUserAlreadyExists) {
			t.Fatalf("Callback() error = %v, want ErrUserAlreadyExists", err)
		}
		if len(f.users.identities) != 0 {
			t.Error("identity linked to account with unverified email")
		}
	})
}

func TestOAuthUseCase_Callback_Rejections(t *testing.T) {
	t.Run("unknown state", func(t *testing.T) {
		f := newOAuthFixture(t, newProfile())
		_, err := f.oauth.Callback(context.Background(), "code", "forged")
		if !errors.Is(err, apperrors.ErrInvalidOAuthState) {
			t.Fatalf("error = %v, want ErrInvalidOAuthState", err)
		}
	})

	t.Run("expired state", func(t *testing.T) {
		f := newOAuthFixture(t, newProfile())
		state := f.start(t)
		f.now = f.now.Add(11 * time.Minute)

		if _, err := f.oauth.Callback(context.Background(), "code", state); !errors.Is(err, apperrors.ErrInvalidOAuthState) {
			t.Fatalf("error = %v, want ErrInvalidOAuthState", err)
		}
	})

	t.Run("provider disabled after start", func(t *testing.T) {
		f := newOAuthFixture(t, newProfile())
		state := f.start(t)
		delete(f.oauth.providers, testProvider)

		_, err := f.oauth.Callback(context.Background(), "code", state)
		if !errors.Is(err, apperrors.ErrUnsupportedOAuthProvider) {
			t.Fatalf("error = %v, want ErrUnsupportedOAuthProvider", err)
		}
	})

	t.Run("invalid authorization code", func(t *testing.T) {
		f := newOAuthFixture(t, newProfile())
		f.provider.exchangeErr = apperrors.ErrInvalidOAuthCode

		_, err := f.oauth.Callback(context.Background(), "bad", f.start(t))
		if !errors.Is(err, apperrors.ErrInvalidOAuthCode) {
			t.Fatalf("error = %v, want ErrInvalidOAuthCode", err)
		}
	})

	t.Run("profile without valid email", func(t *testing.T) {
		profile := newProfile()
		profile.Email = "not-an-email"
		f := newOAuthFixture(t, profile)

		_, err := f.oauth.Callback(context.Background(), "code", f.start(t))
		if !errors.Is(err, apperrors.ErrOAuthEmailRequired) {
			t.Fatalf("error = %v, want ErrOAuthEmailRequired", err)
		}
		if len(f.users.users) != 0 {
			t.Error("user created without email")
		}
	})
}

func TestOAuthUseCase_Callback_KeepsValidPhone(t *testing.T) {
	f := newOAuthFixture(t, newProfile())

	out, err := f.oauth.Callback(context.Background(), "code", f.start(t))
	if err != nil {
		t.Fatalf("Callback() error = %v", err)
	}
	if got := f.users.users[out.UserID].PhoneNumber; got != "+79990001122" {
		t.Errorf("phone = %q, want +79990001122", got)
	}
}

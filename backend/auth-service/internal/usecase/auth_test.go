package usecase

import (
	"bytes"
	"context"
	"errors"
	"testing"

	apperrors "github.com/andrey1af/shop-api/backend/auth-service/internal/errors"
	"golang.org/x/crypto/bcrypt"
)

const (
	testEmail    = "user@example.com"
	testPassword = "correct-horse"
)

func TestAuthUseCase_Register(t *testing.T) {
	t.Run("stores bcrypt hash and opens a session", func(t *testing.T) {
		f := newAuthFixture(t)

		out := f.register(t, testEmail, testPassword)

		user := f.users.users[out.UserID]
		if bytes.Equal(user.PasswordHash, []byte(testPassword)) {
			t.Fatal("password stored in plain text")
		}
		if err := bcrypt.CompareHashAndPassword(user.PasswordHash, []byte(testPassword)); err != nil {
			t.Fatalf("stored hash does not match password: %v", err)
		}
		if out.Token.AccessToken == "" || out.Token.RefreshToken == "" {
			t.Fatalf("token = %+v, want access and refresh", out.Token)
		}

		claims, err := f.auth.ValidateToken(context.Background(), out.Token.AccessToken)
		if err != nil || claims.UserID != out.UserID || claims.Email != testEmail {
			t.Fatalf("ValidateToken() = %+v, %v", claims, err)
		}
	})

	t.Run("duplicate email does not open a session", func(t *testing.T) {
		f := newAuthFixture(t)
		f.register(t, testEmail, testPassword)
		sessionsBefore := len(f.store.sessions)

		_, err := f.auth.Register(context.Background(), RegisterUserInput{Email: testEmail, Password: "other-password"})
		if !errors.Is(err, apperrors.ErrUserAlreadyExists) {
			t.Fatalf("Register() error = %v, want ErrUserAlreadyExists", err)
		}
		if len(f.store.sessions) != sessionsBefore {
			t.Error("session opened for failed registration")
		}
	})
}

func TestAuthUseCase_Login(t *testing.T) {
	f := newAuthFixture(t)
	registered := f.register(t, testEmail, testPassword)

	t.Run("each login opens its own session", func(t *testing.T) {
		first := f.login(t, testEmail, testPassword)
		second := f.login(t, testEmail, testPassword)

		c1, _ := f.auth.ValidateToken(context.Background(), first.AccessToken)
		c2, _ := f.auth.ValidateToken(context.Background(), second.AccessToken)
		if c1.SessionID == c2.SessionID {
			t.Error("two logins share a session")
		}
		if c1.UserID != registered.UserID {
			t.Errorf("logged in as %v, want %v", c1.UserID, registered.UserID)
		}
	})

	for name, in := range map[string]LoginInput{
		"unknown email":  {Email: "nobody@example.com", Password: testPassword},
		"wrong password": {Email: testEmail, Password: "wrong"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := f.auth.Login(context.Background(), in); !errors.Is(err, apperrors.ErrInvalidCredentials) {
				t.Fatalf("Login() error = %v, want ErrInvalidCredentials", err)
			}
		})
	}

	t.Run("oauth user without password cannot log in with password", func(t *testing.T) {
		f := newAuthFixture(t)
		if _, err := f.users.SaveUser(context.Background(), domainUser("oauth@example.com", nil)); err != nil {
			t.Fatal(err)
		}

		if _, err := f.auth.Login(context.Background(), LoginInput{
			Email:    "oauth@example.com",
			Password: "",
		}); !errors.Is(err, apperrors.ErrInvalidCredentials) {
			t.Fatalf("Login() error = %v, want ErrInvalidCredentials", err)
		}
	})
}

func TestAuthUseCase_ChangePassword(t *testing.T) {
	t.Run("changes password, keeps current session, revokes others", func(t *testing.T) {
		f := newAuthFixture(t)
		out := f.register(t, testEmail, testPassword)
		other := f.login(t, testEmail, testPassword)

		token, err := f.auth.ChangePassword(context.Background(), ChangePasswordInput{
			AccessToken: out.Token.AccessToken, OldPassword: testPassword, NewPassword: "new-password",
		})
		if err != nil {
			t.Fatalf("ChangePassword() error = %v", err)
		}
		if token.RefreshToken != "" {
			t.Error("ChangePassword() issued a new refresh token")
		}

		if _, err := f.auth.ValidateToken(context.Background(), token.AccessToken); err != nil {
			t.Errorf("new access token rejected: %v", err)
		}
		if _, err := f.auth.ValidateToken(context.Background(), out.Token.AccessToken); err != nil {
			t.Errorf("current session revoked: %v", err)
		}
		_, err = f.auth.ValidateToken(context.Background(), other.AccessToken)
		if !errors.Is(err, apperrors.ErrInvalidToken) {
			t.Errorf("other session still valid: %v", err)
		}

		f.login(t, testEmail, "new-password")
		if _, err := f.auth.Login(context.Background(), LoginInput{
			Email:    testEmail,
			Password: testPassword,
		}); !errors.Is(err, apperrors.ErrInvalidCredentials) {
			t.Errorf("old password still works: %v", err)
		}
	})

	t.Run("wrong old password changes nothing", func(t *testing.T) {
		f := newAuthFixture(t)
		out := f.register(t, testEmail, testPassword)
		other := f.login(t, testEmail, testPassword)

		_, err := f.auth.ChangePassword(context.Background(), ChangePasswordInput{
			AccessToken: out.Token.AccessToken, OldPassword: "wrong", NewPassword: "new-password",
		})
		if !errors.Is(err, apperrors.ErrInvalidCredentials) {
			t.Fatalf("ChangePassword() error = %v, want ErrInvalidCredentials", err)
		}
		f.login(t, testEmail, testPassword)
		if _, err := f.auth.ValidateToken(context.Background(), other.AccessToken); err != nil {
			t.Errorf("other session revoked after failed change: %v", err)
		}
	})

	t.Run("revoked session cannot change password", func(t *testing.T) {
		f := newAuthFixture(t)
		out := f.register(t, testEmail, testPassword)
		if err := f.auth.LogoutAll(context.Background(), out.Token.AccessToken); err != nil {
			t.Fatal(err)
		}

		_, err := f.auth.ChangePassword(context.Background(), ChangePasswordInput{
			AccessToken: out.Token.AccessToken, OldPassword: testPassword, NewPassword: "new-password",
		})
		if !errors.Is(err, apperrors.ErrInvalidToken) {
			t.Fatalf("ChangePassword() error = %v, want ErrInvalidToken", err)
		}
	})
}

func TestAuthUseCase_ResetPassword(t *testing.T) {
	t.Run("replaces password and revokes all sessions", func(t *testing.T) {
		f := newAuthFixture(t)
		out := f.register(t, testEmail, testPassword)

		if err := f.auth.ResetPassword(context.Background(), testEmail); err != nil {
			t.Fatalf("ResetPassword() error = %v", err)
		}

		if _, err := f.auth.Login(context.Background(), LoginInput{
			Email:    testEmail,
			Password: testPassword,
		}); !errors.Is(err, apperrors.ErrInvalidCredentials) {
			t.Errorf("old password still works: %v", err)
		}
		_, err := f.auth.ValidateToken(context.Background(), out.Token.AccessToken)
		if !errors.Is(err, apperrors.ErrInvalidToken) {
			t.Errorf("session survived reset: %v", err)
		}
		_, err = f.auth.Refresh(context.Background(), out.Token.RefreshToken)
		if !errors.Is(err, apperrors.ErrInvalidToken) {
			t.Errorf("refresh survived reset: %v", err)
		}
	})

	t.Run("unknown email succeeds without revealing that the email is not registered", func(t *testing.T) {
		f := newAuthFixture(t)
		out := f.register(t, testEmail, testPassword)

		if err := f.auth.ResetPassword(context.Background(), "nobody@example.com"); err != nil {
			t.Fatalf("ResetPassword() error = %v, want nil", err)
		}
		if len(f.users.users) != 1 {
			t.Error("user created by password reset")
		}
		f.login(t, testEmail, testPassword)
		if _, err := f.auth.ValidateToken(context.Background(), out.Token.AccessToken); err != nil {
			t.Errorf("other user's session revoked: %v", err)
		}
	})
}

func TestAuthUseCase_ValidateToken_Garbage(t *testing.T) {
	f := newAuthFixture(t)
	if _, err := f.auth.ValidateToken(context.Background(), "not-a-jwt"); !errors.Is(err, apperrors.ErrInvalidToken) {
		t.Fatalf("ValidateToken() error = %v, want ErrInvalidToken", err)
	}
}

func TestAuthUseCase_Refresh(t *testing.T) {
	f := newAuthFixture(t)
	out := f.register(t, testEmail, testPassword)

	next, err := f.auth.Refresh(context.Background(), out.Token.RefreshToken)
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if _, err := f.auth.ValidateToken(context.Background(), next.AccessToken); err != nil {
		t.Errorf("refreshed access rejected: %v", err)
	}
	if _, err := f.auth.Refresh(context.Background(), "unknown"); !errors.Is(err, apperrors.ErrInvalidToken) {
		t.Errorf("Refresh(unknown) error = %v, want ErrInvalidToken", err)
	}
}

func TestAuthUseCase_Logout(t *testing.T) {
	ctx := context.Background()

	t.Run("by refresh token", func(t *testing.T) {
		f := newAuthFixture(t)
		out := f.register(t, testEmail, testPassword)

		if err := f.auth.Logout(ctx, LogoutInput{RefreshToken: out.Token.RefreshToken}); err != nil {
			t.Fatalf("Logout() error = %v", err)
		}
		if _, err := f.auth.ValidateToken(ctx, out.Token.AccessToken); !errors.Is(err, apperrors.ErrInvalidToken) {
			t.Errorf("access survived logout: %v", err)
		}
	})

	t.Run("by access token", func(t *testing.T) {
		f := newAuthFixture(t)
		out := f.register(t, testEmail, testPassword)

		if err := f.auth.Logout(ctx, LogoutInput{AccessToken: out.Token.AccessToken}); err != nil {
			t.Fatalf("Logout() error = %v", err)
		}
		if _, err := f.auth.Refresh(ctx, out.Token.RefreshToken); !errors.Is(err, apperrors.ErrInvalidToken) {
			t.Errorf("refresh survived logout: %v", err)
		}
	})

	t.Run("refresh token wins over access token", func(t *testing.T) {
		f := newAuthFixture(t)
		first := f.register(t, testEmail, testPassword)
		second := f.login(t, testEmail, testPassword)

		if err := f.auth.Logout(ctx, LogoutInput{
			RefreshToken: first.Token.RefreshToken,
			AccessToken:  second.AccessToken,
		}); err != nil {
			t.Fatalf("Logout() error = %v", err)
		}
		if _, err := f.auth.ValidateToken(ctx, second.AccessToken); err != nil {
			t.Errorf("session of access token was revoked: %v", err)
		}
		if _, err := f.auth.ValidateToken(ctx, first.Token.AccessToken); !errors.Is(err, apperrors.ErrInvalidToken) {
			t.Errorf("session of refresh token survived: %v", err)
		}
	})

	t.Run("without tokens", func(t *testing.T) {
		f := newAuthFixture(t)
		if err := f.auth.Logout(ctx, LogoutInput{}); !errors.Is(err, apperrors.ErrInvalidToken) {
			t.Fatalf("Logout() error = %v, want ErrInvalidToken", err)
		}
	})
}

func TestAuthUseCase_LogoutAll(t *testing.T) {
	ctx := context.Background()
	f := newAuthFixture(t)
	first := f.register(t, testEmail, testPassword)
	second := f.login(t, testEmail, testPassword)
	stranger := f.register(t, "other@example.com", testPassword)

	if err := f.auth.LogoutAll(ctx, first.Token.AccessToken); err != nil {
		t.Fatalf("LogoutAll() error = %v", err)
	}
	for name, token := range map[string]string{"first": first.Token.AccessToken, "second": second.AccessToken} {
		if _, err := f.auth.ValidateToken(ctx, token); !errors.Is(err, apperrors.ErrInvalidToken) {
			t.Errorf("%s session survived: %v", name, err)
		}
	}
	if _, err := f.auth.ValidateToken(ctx, stranger.Token.AccessToken); err != nil {
		t.Errorf("another user's session revoked: %v", err)
	}
	if err := f.auth.LogoutAll(ctx, first.Token.AccessToken); !errors.Is(err, apperrors.ErrInvalidToken) {
		t.Errorf("LogoutAll(revoked token) error = %v, want ErrInvalidToken", err)
	}
}

func TestAuthUseCase_PasswordPolicy(t *testing.T) {
	ctx := context.Background()

	t.Run("register rejects short password", func(t *testing.T) {
		f := newAuthFixture(t)

		_, err := f.auth.Register(ctx, RegisterUserInput{Email: testEmail, Password: "1234567"})
		if !errors.Is(err, apperrors.ErrWeakPassword) {
			t.Fatalf("Register() error = %v, want ErrWeakPassword", err)
		}
		if len(f.users.users) != 0 {
			t.Error("user created with weak password")
		}
	})

	t.Run("length is counted in characters, not bytes", func(t *testing.T) {
		f := newAuthFixture(t)

		if _, err := f.auth.Register(ctx, RegisterUserInput{
			Email:    testEmail,
			Password: "пароль1",
		}); !errors.Is(err, apperrors.ErrWeakPassword) {
			t.Errorf("7 cyrillic characters: error = %v, want ErrWeakPassword", err)
		}
		if _, err := f.auth.Register(ctx, RegisterUserInput{Email: testEmail, Password: "пароль12"}); err != nil {
			t.Errorf("8 cyrillic characters: error = %v", err)
		}
	})

	t.Run("change password rejects short new password", func(t *testing.T) {
		f := newAuthFixture(t)
		out := f.register(t, testEmail, testPassword)

		_, err := f.auth.ChangePassword(ctx, ChangePasswordInput{
			AccessToken: out.Token.AccessToken,
			OldPassword: testPassword,
			NewPassword: "short",
		})
		if !errors.Is(err, apperrors.ErrWeakPassword) {
			t.Fatalf("ChangePassword() error = %v, want ErrWeakPassword", err)
		}
		f.login(t, testEmail, testPassword)
	})
}

package usecase

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
	"uuid"

	apperrors "github.com/andrey1af/shop-api/backend/api-service/internal/errors"
)

type recordingAuthProvider struct {
	err error

	token  Token
	claims TokenClaims
	userID uuid.UUID

	gotRegister RegisterUserInput
	gotLogin    LoginInput
	gotChange   ChangePasswordInput
	gotLogout   LogoutInput
	gotString   string
	gotOAuth    OAuthCallbackInput
}

func (p *recordingAuthProvider) Register(_ context.Context, in RegisterUserInput) (RegisterUserOutput, error) {
	p.gotRegister = in
	return RegisterUserOutput{UserID: p.userID, Token: p.token}, p.err
}
func (p *recordingAuthProvider) Login(_ context.Context, in LoginInput) (LoginOutput, error) {
	p.gotLogin = in
	return LoginOutput{UserID: p.userID, Token: p.token}, p.err
}
func (p *recordingAuthProvider) ChangePassword(_ context.Context, in ChangePasswordInput) (Token, error) {
	p.gotChange = in
	return p.token, p.err
}
func (p *recordingAuthProvider) ResetPassword(_ context.Context, email string) error {
	p.gotString = email
	return p.err
}
func (p *recordingAuthProvider) ValidateToken(_ context.Context, accessToken string) (TokenClaims, error) {
	p.gotString = accessToken
	return p.claims, p.err
}
func (p *recordingAuthProvider) Refresh(_ context.Context, refreshToken string) (Token, error) {
	p.gotString = refreshToken
	return p.token, p.err
}
func (p *recordingAuthProvider) Logout(_ context.Context, in LogoutInput) error {
	p.gotLogout = in
	return p.err
}
func (p *recordingAuthProvider) LogoutAll(_ context.Context, accessToken string) error {
	p.gotString = accessToken
	return p.err
}
func (p *recordingAuthProvider) OAuthStart(_ context.Context, provider string) (string, error) {
	p.gotString = provider
	return "https://provider/authorize", p.err
}
func (p *recordingAuthProvider) OAuthCallback(_ context.Context, in OAuthCallbackInput) (OAuthLoginOutput, error) {
	p.gotOAuth = in
	return OAuthLoginOutput{UserID: p.userID, Token: p.token}, p.err
}

func newTestToken() Token {
	return Token{
		AccessToken:      "a",
		ExpiresAt:        time.Now().Add(15 * time.Minute),
		RefreshToken:     "r",
		RefreshExpiresAt: time.Now().Add(720 * time.Hour),
	}
}

func TestAuthUseCase_PassesDataThrough(t *testing.T) {
	ctx := context.Background()
	p := &recordingAuthProvider{
		userID: uuid.New(),
		token:  newTestToken(),
		claims: TokenClaims{UserID: uuid.New(), SessionID: uuid.New()},
	}
	uc := NewAuthUseCase(newTestLogger(), p)

	reg := RegisterUserInput{Name: "I", Surname: "P", Email: "u@example.com", PhoneNumber: "+79991234567", Password: "p"}
	out, err := uc.Register(ctx, reg)
	if err != nil || !reflect.DeepEqual(p.gotRegister, reg) || out.UserID != p.userID || out.Token != p.token {
		t.Errorf("Register: out=%+v err=%v got=%+v", out, err, p.gotRegister)
	}

	login := LoginInput{Email: "u@example.com", Password: "p"}
	lo, err := uc.Login(ctx, login)
	if err != nil || p.gotLogin != login || lo.Token != p.token {
		t.Errorf("Login: out=%+v err=%v", lo, err)
	}

	change := ChangePasswordInput{AccessToken: "t", OldPassword: "o", NewPassword: "n"}
	if tok, err := uc.ChangePassword(ctx, change); err != nil || p.gotChange != change || tok != p.token {
		t.Errorf("ChangePassword: %+v %v", tok, err)
	}

	if err := uc.ResetPassword(ctx, "u@example.com"); err != nil || p.gotString != "u@example.com" {
		t.Errorf("ResetPassword: %v %q", err, p.gotString)
	}

	if claims, err := uc.ValidateToken(ctx, "access"); err != nil || p.gotString != "access" || claims != p.claims {
		t.Errorf("ValidateToken: %+v %v", claims, err)
	}

	if tok, err := uc.Refresh(ctx, "refresh"); err != nil || p.gotString != "refresh" || tok != p.token {
		t.Errorf("Refresh: %+v %v", tok, err)
	}

	logout := LogoutInput{RefreshToken: "r", AccessToken: "a"}
	if err := uc.Logout(ctx, logout); err != nil || p.gotLogout != logout {
		t.Errorf("Logout: %v %+v", err, p.gotLogout)
	}

	if err := uc.LogoutAll(ctx, "access-all"); err != nil || p.gotString != "access-all" {
		t.Errorf("LogoutAll: %v %q", err, p.gotString)
	}

	if url, err := uc.OAuthStart(ctx, "yandex"); err != nil || p.gotString != "yandex" || url == "" {
		t.Errorf("OAuthStart: %q %v", url, err)
	}

	cb := OAuthCallbackInput{Code: "c", State: "s"}
	if o, err := uc.OAuthCallback(ctx, cb); err != nil || p.gotOAuth != cb || o.Token != p.token {
		t.Errorf("OAuthCallback: %+v %v", o, err)
	}
}

func TestAuthUseCase_PropagatesErrors(t *testing.T) {
	ctx := context.Background()

	for _, wantErr := range []error{
		apperrors.ErrInvalidCredentials,
		apperrors.ErrInvalidToken,
		apperrors.ErrUserAlreadyExists,
		errors.New("auth unavailable"),
	} {
		uc := NewAuthUseCase(newTestLogger(), &recordingAuthProvider{err: wantErr})

		calls := map[string]error{}
		_, calls["Register"] = uc.Register(ctx, RegisterUserInput{})
		_, calls["Login"] = uc.Login(ctx, LoginInput{})
		_, calls["ChangePassword"] = uc.ChangePassword(ctx, ChangePasswordInput{})
		calls["ResetPassword"] = uc.ResetPassword(ctx, "")
		_, calls["ValidateToken"] = uc.ValidateToken(ctx, "")
		_, calls["Refresh"] = uc.Refresh(ctx, "")
		calls["Logout"] = uc.Logout(ctx, LogoutInput{})
		calls["LogoutAll"] = uc.LogoutAll(ctx, "")
		_, calls["OAuthStart"] = uc.OAuthStart(ctx, "")
		_, calls["OAuthCallback"] = uc.OAuthCallback(ctx, OAuthCallbackInput{})

		for name, err := range calls {
			if !errors.Is(err, wantErr) {
				t.Errorf("%s: error = %v, want %v", name, err, wantErr)
			}
		}
	}
}

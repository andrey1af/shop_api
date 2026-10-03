package yandex

import (
	"context"
	jsonv2 "encoding/json/v2"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/andrey1af/shop-api/backend/auth-service/internal/domain"
	apperrors "github.com/andrey1af/shop-api/backend/auth-service/internal/errors"
	"golang.org/x/oauth2"
)

const Name = "yandex"

const userInfoURL = "https://login.yandex.ru/info?format=json"

var endpoint = oauth2.Endpoint{
	AuthURL:   "https://oauth.yandex.ru/authorize",
	TokenURL:  "https://oauth.yandex.ru/token",
	AuthStyle: oauth2.AuthStyleInParams,
}

var scopes = []string{"login:email", "login:info"}

const optionalScope = "login:default_phone"

type Provider struct {
	config *oauth2.Config
	client *http.Client
}

func New(clientID, clientSecret, redirectURL string, timeout time.Duration) *Provider {
	return &Provider{
		config: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Endpoint:     endpoint,
			Scopes:       scopes,
		},
		client: &http.Client{Timeout: timeout},
	}
}

func (p *Provider) AuthCodeURL(state, codeVerifier string) string {
	return p.config.AuthCodeURL(state,
		oauth2.S256ChallengeOption(codeVerifier),
		oauth2.SetAuthURLParam("optional_scope", optionalScope),
	)
}

type userInfo struct {
	ID           string `json:"id"`
	Login        string `json:"login"`
	DefaultEmail string `json:"default_email"`
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name"`
	DisplayName  string `json:"display_name"`
	DefaultPhone *struct {
		Number string `json:"number"`
	} `json:"default_phone"`
}

func (p *Provider) Exchange(ctx context.Context, code, codeVerifier string) (domain.OAuthProfile, error) {
	const op = "oauth.yandex.Provider.Exchange"

	ctx = context.WithValue(ctx, oauth2.HTTPClient, p.client)

	token, err := p.config.Exchange(ctx, code, oauth2.VerifierOption(codeVerifier))
	if err != nil {

		var retrieveErr *oauth2.RetrieveError
		if errors.As(err, &retrieveErr) && retrieveErr.ErrorCode == "invalid_grant" {
			return domain.OAuthProfile{}, fmt.Errorf("%s: %w", op, apperrors.ErrInvalidOAuthCode)
		}

		return domain.OAuthProfile{}, fmt.Errorf("%s: %w", op, err)
	}

	info, err := p.userInfo(ctx, token.AccessToken)
	if err != nil {
		return domain.OAuthProfile{}, fmt.Errorf("%s: %w", op, err)
	}

	return toProfile(info), nil
}

func (p *Provider) userInfo(ctx context.Context, accessToken string) (userInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, userInfoURL, nil)
	if err != nil {
		return userInfo{}, err
	}
	req.Header.Set("Authorization", "OAuth "+accessToken)

	resp, err := p.client.Do(req)
	if err != nil {
		return userInfo{}, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return userInfo{}, fmt.Errorf("yandex user info: unexpected status %d", resp.StatusCode)
	}

	var info userInfo
	if err := jsonv2.UnmarshalRead(resp.Body, &info); err != nil {
		return userInfo{}, fmt.Errorf("yandex user info: %w", err)
	}

	return info, nil
}

func toProfile(info userInfo) domain.OAuthProfile {

	name := info.FirstName
	if name == "" {
		name = info.DisplayName
	}
	if name == "" {
		name = info.Login
	}

	var phoneNumber string
	if info.DefaultPhone != nil {
		phoneNumber = info.DefaultPhone.Number
	}

	return domain.OAuthProfile{
		ProviderUserID: info.ID,
		Email:          info.DefaultEmail,

		EmailVerified: true,
		Name:          name,
		Surname:       info.LastName,
		PhoneNumber:   phoneNumber,
	}
}

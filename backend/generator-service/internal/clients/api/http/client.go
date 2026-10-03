package http

import (
	"bytes"
	"context"
	jsonv2 "encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"
	"uuid"

	"github.com/andrey1af/shop-api/backend/generator-service/internal/domain"
	apperrors "github.com/andrey1af/shop-api/backend/generator-service/internal/errors"
)

const (
	pathLogin    = "/api/v1/auth/login"
	pathRegister = "/api/v1/auth/register"
	pathRefresh  = "/api/v1/auth/refresh"
	pathProducts = "/api/v1/products"

	serviceUserName    = "Event"
	serviceUserSurname = "Generator"

	maxErrorBodySize = 512
)

type Credentials struct {
	Email       string
	Password    string
	PhoneNumber string
}

type Client struct {
	baseURL     *url.URL
	httpClient  *http.Client
	credentials Credentials

	mu           sync.Mutex
	accessToken  string
	refreshToken string
}

func New(baseURL string, timeout time.Duration, credentials Credentials) (*Client, error) {
	const op = "apiclient.New"

	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return &Client{
		baseURL:     u,
		httpClient:  &http.Client{Timeout: timeout},
		credentials: credentials,
	}, nil
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type registerRequest struct {
	Email       string `json:"email"`
	Name        string `json:"name"`
	Surname     string `json:"surname"`
	PhoneNumber string `json:"phone_number"`
	Password    string `json:"password"`
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type authResponse struct {
	Token tokenResponse `json:"token"`
}

type productResponse struct {
	ID             uuid.UUID `json:"id"`
	Price          float64   `json:"price"`
	AvailableStock int64     `json:"available_stock"`
}

func (c *Client) ListProducts(ctx context.Context) ([]domain.Product, error) {
	const op = "apiclient.ListProducts"

	products, err := c.listProducts(ctx)
	if errors.Is(err, apperrors.ErrUnauthorized) {
		c.resetToken()
		products, err = c.listProducts(ctx)
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return products, nil
}

func (c *Client) listProducts(ctx context.Context) ([]domain.Product, error) {
	token, err := c.token(ctx)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.endpoint(pathProducts), http.NoBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	var resp []productResponse
	if err := c.do(req, http.StatusOK, &resp); err != nil {
		return nil, err
	}

	products := make([]domain.Product, 0, len(resp))
	for _, p := range resp {
		products = append(products, domain.Product{
			ID:             p.ID,
			Price:          p.Price,
			AvailableStock: p.AvailableStock,
		})
	}

	return products, nil
}

func (c *Client) token(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.accessToken != "" {
		return c.accessToken, nil
	}

	if c.refreshToken != "" {
		token, err := c.refresh(ctx, c.refreshToken)
		if err == nil {
			c.setTokens(token)
			return token.AccessToken, nil
		}
		c.refreshToken = ""
	}

	token, err := c.login(ctx)
	if errors.Is(err, apperrors.ErrInvalidCredentials) {
		token, err = c.register(ctx)
	}
	if err != nil {
		return "", err
	}

	c.setTokens(token)

	return token.AccessToken, nil
}

func (c *Client) setTokens(token tokenResponse) {
	c.accessToken = token.AccessToken
	c.refreshToken = token.RefreshToken
}

func (c *Client) resetToken() {
	c.mu.Lock()
	c.accessToken = ""
	c.mu.Unlock()
}

func (c *Client) refresh(ctx context.Context, refreshToken string) (tokenResponse, error) {
	req, err := c.newJSONRequest(ctx, pathRefresh, refreshRequest{RefreshToken: refreshToken})
	if err != nil {
		return tokenResponse{}, err
	}

	var resp tokenResponse
	if err := c.do(req, http.StatusOK, &resp); err != nil {
		return tokenResponse{}, fmt.Errorf("refresh: %w", err)
	}

	return resp, nil
}

func (c *Client) login(ctx context.Context) (tokenResponse, error) {
	req, err := c.newJSONRequest(ctx, pathLogin, loginRequest{
		Email:    c.credentials.Email,
		Password: c.credentials.Password,
	})
	if err != nil {
		return tokenResponse{}, err
	}

	var resp authResponse
	if err := c.do(req, http.StatusOK, &resp); err != nil {
		if errors.Is(err, apperrors.ErrUnauthorized) {
			return tokenResponse{}, fmt.Errorf("login: %w", apperrors.ErrInvalidCredentials)
		}

		return tokenResponse{}, fmt.Errorf("login: %w", err)
	}

	return resp.Token, nil
}

func (c *Client) register(ctx context.Context) (tokenResponse, error) {
	req, err := c.newJSONRequest(ctx, pathRegister, registerRequest{
		Email:       c.credentials.Email,
		Name:        serviceUserName,
		Surname:     serviceUserSurname,
		PhoneNumber: c.credentials.PhoneNumber,
		Password:    c.credentials.Password,
	})
	if err != nil {
		return tokenResponse{}, err
	}

	var resp authResponse
	if err := c.do(req, http.StatusCreated, &resp); err != nil {
		var statusErr *statusError
		if errors.As(err, &statusErr) && statusErr.code == http.StatusConflict {
			return tokenResponse{}, fmt.Errorf("register: %w", apperrors.ErrInvalidCredentials)
		}

		return tokenResponse{}, fmt.Errorf("register: %w", err)
	}

	return resp.Token, nil
}

func (c *Client) newJSONRequest(ctx context.Context, path string, body any) (*http.Request, error) {
	payload, err := jsonv2.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint(path), bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	return req, nil
}

func (c *Client) do(req *http.Request, wantStatus int, out any) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != wantStatus {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodySize))

		return &statusError{code: resp.StatusCode, body: string(body)}
	}

	return jsonv2.UnmarshalRead(resp.Body, out)
}

func (c *Client) endpoint(path string) string {
	return c.baseURL.JoinPath(path).String()
}

type statusError struct {
	code int
	body string
}

func (e *statusError) Error() string {
	return fmt.Sprintf("%s: %d %s", apperrors.ErrUnexpectedStatus, e.code, e.body)
}

func (e *statusError) Unwrap() error {
	if e.code == http.StatusUnauthorized {
		return apperrors.ErrUnauthorized
	}

	return apperrors.ErrUnexpectedStatus
}

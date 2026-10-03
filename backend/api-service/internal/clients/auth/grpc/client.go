package grpc

import (
	"context"
	"fmt"
	"time"
	"uuid"

	apperrors "github.com/andrey1af/shop-api/backend/api-service/internal/errors"
	"github.com/andrey1af/shop-api/backend/api-service/internal/usecase"
	"github.com/andrey1af/shop-api/backend/gen/authv1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

type Client struct {
	api     authv1.AuthServiceClient
	conn    *grpc.ClientConn
	timeout time.Duration
}

func New(addr string, timeout time.Duration) (*Client, error) {
	const op = "clients.auth.grpc.New"

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return &Client{
		api:     authv1.NewAuthServiceClient(conn),
		conn:    conn,
		timeout: timeout,
	}, nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}

func (c *Client) Register(ctx context.Context, in usecase.RegisterUserInput) (usecase.RegisterUserOutput, error) {
	const op = "clients.auth.grpc.Client.Register"

	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	resp, err := c.api.Register(ctx, &authv1.RegisterRequest{
		Email:       in.Email,
		Name:        in.Name,
		Surname:     in.Surname,
		PhoneNumber: in.PhoneNumber,
		Password:    in.Password,
	})
	if err != nil {
		return usecase.RegisterUserOutput{}, fmt.Errorf("%s: %w", op, toAppError(err, nil))
	}

	userID, err := uuid.Parse(resp.GetUserId())
	if err != nil {
		return usecase.RegisterUserOutput{}, fmt.Errorf("%s: %w", op, err)
	}

	return usecase.RegisterUserOutput{
		UserID: userID,
		Token:  toToken(resp.GetToken()),
	}, nil
}

func (c *Client) Login(ctx context.Context, in usecase.LoginInput) (usecase.LoginOutput, error) {
	const op = "clients.auth.grpc.Client.Login"

	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	resp, err := c.api.Login(ctx, &authv1.LoginRequest{
		Email:    in.Email,
		Password: in.Password,
	})
	if err != nil {
		return usecase.LoginOutput{}, fmt.Errorf("%s: %w", op, toAppError(err, apperrors.ErrInvalidCredentials))
	}

	userID, err := uuid.Parse(resp.GetUserId())
	if err != nil {
		return usecase.LoginOutput{}, fmt.Errorf("%s: %w", op, err)
	}

	return usecase.LoginOutput{
		UserID: userID,
		Token:  toToken(resp.GetToken()),
	}, nil
}

func (c *Client) ChangePassword(ctx context.Context, in usecase.ChangePasswordInput) (usecase.Token, error) {
	const op = "clients.auth.grpc.Client.ChangePassword"

	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	resp, err := c.api.ChangePassword(ctx, &authv1.ChangePasswordRequest{
		AccessToken: in.AccessToken,
		OldPassword: in.OldPassword,
		NewPassword: in.NewPassword,
	})
	if err != nil {
		return usecase.Token{}, fmt.Errorf("%s: %w", op, toAppError(err, apperrors.ErrInvalidCredentials))
	}

	return toToken(resp.GetToken()), nil
}

func (c *Client) ResetPassword(ctx context.Context, email string) error {
	const op = "clients.auth.grpc.Client.ResetPassword"

	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	if _, err := c.api.ResetPassword(ctx, &authv1.ResetPasswordRequest{Email: email}); err != nil {
		return fmt.Errorf("%s: %w", op, toAppError(err, nil))
	}

	return nil
}

func (c *Client) ValidateToken(ctx context.Context, accessToken string) (usecase.TokenClaims, error) {
	const op = "clients.auth.grpc.Client.ValidateToken"

	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	resp, err := c.api.ValidateToken(ctx, &authv1.ValidateTokenRequest{AccessToken: accessToken})
	if err != nil {
		return usecase.TokenClaims{}, fmt.Errorf("%s: %w", op, toAppError(err, apperrors.ErrInvalidToken))
	}

	userID, err := uuid.Parse(resp.GetUserId())
	if err != nil {
		return usecase.TokenClaims{}, fmt.Errorf("%s: %w", op, err)
	}

	sessionID, err := uuid.Parse(resp.GetSessionId())
	if err != nil {
		return usecase.TokenClaims{}, fmt.Errorf("%s: %w", op, err)
	}

	return usecase.TokenClaims{
		UserID:    userID,
		Email:     resp.GetEmail(),
		SessionID: sessionID,
		ExpiresAt: resp.GetExpiresAt().AsTime(),
	}, nil
}

func (c *Client) Refresh(ctx context.Context, refreshToken string) (usecase.Token, error) {
	const op = "clients.auth.grpc.Client.Refresh"

	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	resp, err := c.api.Refresh(ctx, &authv1.RefreshRequest{RefreshToken: refreshToken})
	if err != nil {
		return usecase.Token{}, fmt.Errorf("%s: %w", op, toAppError(err, apperrors.ErrInvalidToken))
	}

	return toToken(resp.GetToken()), nil
}

func (c *Client) Logout(ctx context.Context, in usecase.LogoutInput) error {
	const op = "clients.auth.grpc.Client.Logout"

	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	if _, err := c.api.Logout(ctx, &authv1.LogoutRequest{
		RefreshToken: in.RefreshToken,
		AccessToken:  in.AccessToken,
	}); err != nil {
		return fmt.Errorf("%s: %w", op, toAppError(err, apperrors.ErrInvalidToken))
	}

	return nil
}

func (c *Client) LogoutAll(ctx context.Context, accessToken string) error {
	const op = "clients.auth.grpc.Client.LogoutAll"

	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	if _, err := c.api.LogoutAll(ctx, &authv1.LogoutAllRequest{AccessToken: accessToken}); err != nil {
		return fmt.Errorf("%s: %w", op, toAppError(err, apperrors.ErrInvalidToken))
	}

	return nil
}

func (c *Client) OAuthStart(ctx context.Context, provider string) (string, error) {
	const op = "clients.auth.grpc.Client.OAuthStart"

	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	resp, err := c.api.OAuthStart(ctx, &authv1.OAuthStartRequest{Provider: provider})
	if err != nil {
		return "", fmt.Errorf("%s: %w", op, toAppError(err, nil))
	}

	return resp.GetAuthUrl(), nil
}

func (c *Client) OAuthCallback(ctx context.Context, in usecase.OAuthCallbackInput) (usecase.OAuthLoginOutput, error) {
	const op = "clients.auth.grpc.Client.OAuthCallback"

	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	resp, err := c.api.OAuthCallback(ctx, &authv1.OAuthCallbackRequest{
		Code:  in.Code,
		State: in.State,
	})
	if err != nil {
		return usecase.OAuthLoginOutput{}, fmt.Errorf("%s: %w", op, toAppError(err, apperrors.ErrOAuthFailed))
	}

	userID, err := uuid.Parse(resp.GetUserId())
	if err != nil {
		return usecase.OAuthLoginOutput{}, fmt.Errorf("%s: %w", op, err)
	}

	return usecase.OAuthLoginOutput{
		UserID: userID,
		Email:  resp.GetEmail(),
		Token:  toToken(resp.GetToken()),
	}, nil
}

func toToken(token *authv1.Token) usecase.Token {
	out := usecase.Token{
		AccessToken:  token.GetAccessToken(),
		ExpiresAt:    token.GetExpiresAt().AsTime(),
		RefreshToken: token.GetRefreshToken(),
	}
	if token.GetRefreshExpiresAt() != nil {
		out.RefreshExpiresAt = token.GetRefreshExpiresAt().AsTime()
	}

	return out
}

func toAppError(err error, unauthenticated error) error {
	switch status.Code(err) {
	case codes.AlreadyExists:
		return apperrors.ErrUserAlreadyExists
	case codes.InvalidArgument:
		return apperrors.ErrInvalidAuthRequest
	case codes.NotFound:
		return apperrors.ErrUserNotFound
	case codes.Unauthenticated:
		if unauthenticated != nil {
			return unauthenticated
		}
		return err
	default:
		return err
	}
}

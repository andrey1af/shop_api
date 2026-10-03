package grpc

import (
	"context"
	"errors"

	apperrors "github.com/andrey1af/shop-api/backend/auth-service/internal/errors"
	"github.com/andrey1af/shop-api/backend/auth-service/internal/usecase"
	"github.com/andrey1af/shop-api/backend/auth-service/pkg/validation"
	"github.com/andrey1af/shop-api/backend/gen/authv1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type authUseCase interface {
	Register(ctx context.Context, in usecase.RegisterUserInput) (usecase.RegisterUserOutput, error)
	Login(ctx context.Context, in usecase.LoginInput) (usecase.LoginOutput, error)
	ChangePassword(ctx context.Context, in usecase.ChangePasswordInput) (usecase.Token, error)
	ResetPassword(ctx context.Context, email string) error
	ValidateToken(ctx context.Context, accessToken string) (usecase.TokenClaims, error)
	Refresh(ctx context.Context, refreshToken string) (usecase.Token, error)
	Logout(ctx context.Context, in usecase.LogoutInput) error
	LogoutAll(ctx context.Context, accessToken string) error
}

type oauthUseCase interface {
	Start(ctx context.Context, provider string) (string, error)
	Callback(ctx context.Context, code, state string) (usecase.OAuthLoginOutput, error)
}

type Server struct {
	authv1.UnimplementedAuthServiceServer
	auth  authUseCase
	oauth oauthUseCase
}

func RegisterServer(gRPC *grpc.Server, auth authUseCase, oauth oauthUseCase) {
	authv1.RegisterAuthServiceServer(gRPC, &Server{auth: auth, oauth: oauth})
}

func (s *Server) Register(ctx context.Context, req *authv1.RegisterRequest) (*authv1.RegisterResponse, error) {
	if req.GetEmail() == "" {
		return nil, status.Error(codes.InvalidArgument, "email is required")
	}
	if !validation.IsValidEmail(req.GetEmail()) {
		return nil, status.Error(codes.InvalidArgument, "invalid email")
	}
	if req.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	if req.GetSurname() == "" {
		return nil, status.Error(codes.InvalidArgument, "surname is required")
	}
	if req.GetPhoneNumber() == "" {
		return nil, status.Error(codes.InvalidArgument, "phone number is required")
	}
	if !validation.IsValidPhoneNumber(req.GetPhoneNumber()) {
		return nil, status.Error(codes.InvalidArgument, "invalid phone number")
	}
	if req.GetPassword() == "" {
		return nil, status.Error(codes.InvalidArgument, "password is required")
	}

	out, err := s.auth.Register(ctx, registerRequestToInput(req))
	if err != nil {
		if errors.Is(err, apperrors.ErrUserAlreadyExists) {
			return nil, status.Error(codes.AlreadyExists, "user already exists")
		}
		if errors.Is(err, apperrors.ErrWeakPassword) {
			return nil, status.Error(codes.InvalidArgument, "password is too short")
		}

		return nil, status.Error(codes.Internal, "internal error")
	}

	return registerOutputToResponse(out), nil
}

func (s *Server) Login(ctx context.Context, req *authv1.LoginRequest) (*authv1.LoginResponse, error) {
	if req.GetEmail() == "" {
		return nil, status.Error(codes.InvalidArgument, "email is required")
	}
	if !validation.IsValidEmail(req.GetEmail()) {
		return nil, status.Error(codes.InvalidArgument, "invalid email")
	}
	if req.GetPassword() == "" {
		return nil, status.Error(codes.InvalidArgument, "password is required")
	}

	out, err := s.auth.Login(ctx, loginRequestToInput(req))
	if err != nil {
		if errors.Is(err, apperrors.ErrInvalidCredentials) {
			return nil, status.Error(codes.Unauthenticated, "invalid credentials")
		}

		return nil, status.Error(codes.Internal, "internal error")
	}

	return loginOutputToResponse(out), nil
}

func (s *Server) ChangePassword(
	ctx context.Context,
	req *authv1.ChangePasswordRequest,
) (*authv1.ChangePasswordResponse, error) {
	if req.GetAccessToken() == "" {
		return nil, status.Error(codes.InvalidArgument, "access token is required")
	}
	if req.GetOldPassword() == "" {
		return nil, status.Error(codes.InvalidArgument, "old password is required")
	}
	if req.GetNewPassword() == "" {
		return nil, status.Error(codes.InvalidArgument, "new password is required")
	}

	token, err := s.auth.ChangePassword(ctx, changePasswordRequestToInput(req))
	if err != nil {
		switch {
		case errors.Is(err, apperrors.ErrInvalidToken):
			return nil, status.Error(codes.Unauthenticated, "invalid access token")
		case errors.Is(err, apperrors.ErrInvalidCredentials):
			return nil, status.Error(codes.Unauthenticated, "invalid credentials")
		case errors.Is(err, apperrors.ErrUserNotFound):
			return nil, status.Error(codes.NotFound, "user not found")
		case errors.Is(err, apperrors.ErrWeakPassword):
			return nil, status.Error(codes.InvalidArgument, "password is too short")
		}

		return nil, status.Error(codes.Internal, "internal error")
	}

	return changePasswordTokenToResponse(token), nil
}

func (s *Server) ResetPassword(
	ctx context.Context,
	req *authv1.ResetPasswordRequest,
) (*authv1.ResetPasswordResponse, error) {
	if req.GetEmail() == "" {
		return nil, status.Error(codes.InvalidArgument, "email is required")
	}
	if !validation.IsValidEmail(req.GetEmail()) {
		return nil, status.Error(codes.InvalidArgument, "invalid email")
	}

	if err := s.auth.ResetPassword(ctx, req.GetEmail()); err != nil {
		if errors.Is(err, apperrors.ErrUserNotFound) {
			return nil, status.Error(codes.NotFound, "user not found")
		}

		return nil, status.Error(codes.Internal, "internal error")
	}

	return &authv1.ResetPasswordResponse{}, nil
}

func (s *Server) ValidateToken(
	ctx context.Context,
	req *authv1.ValidateTokenRequest,
) (*authv1.ValidateTokenResponse, error) {
	if req.GetAccessToken() == "" {
		return nil, status.Error(codes.InvalidArgument, "access token is required")
	}

	claims, err := s.auth.ValidateToken(ctx, req.GetAccessToken())
	if err != nil {
		if errors.Is(err, apperrors.ErrInvalidToken) {
			return nil, status.Error(codes.Unauthenticated, "invalid access token")
		}

		return nil, status.Error(codes.Internal, "internal error")
	}

	return claimsToValidateTokenResponse(claims), nil
}

func (s *Server) Refresh(ctx context.Context, req *authv1.RefreshRequest) (*authv1.RefreshResponse, error) {
	if req.GetRefreshToken() == "" {
		return nil, status.Error(codes.InvalidArgument, "refresh token is required")
	}

	token, err := s.auth.Refresh(ctx, req.GetRefreshToken())
	if err != nil {
		if errors.Is(err, apperrors.ErrInvalidToken) {
			return nil, status.Error(codes.Unauthenticated, "invalid refresh token")
		}

		return nil, status.Error(codes.Internal, "internal error")
	}

	return &authv1.RefreshResponse{Token: tokenToProto(token)}, nil
}

func (s *Server) Logout(ctx context.Context, req *authv1.LogoutRequest) (*authv1.LogoutResponse, error) {
	if req.GetRefreshToken() == "" && req.GetAccessToken() == "" {
		return nil, status.Error(codes.InvalidArgument, "refresh token or access token is required")
	}

	if err := s.auth.Logout(ctx, usecase.LogoutInput{
		RefreshToken: req.GetRefreshToken(),
		AccessToken:  req.GetAccessToken(),
	}); err != nil {
		if errors.Is(err, apperrors.ErrInvalidToken) {
			return nil, status.Error(codes.Unauthenticated, "invalid token")
		}

		return nil, status.Error(codes.Internal, "internal error")
	}

	return &authv1.LogoutResponse{}, nil
}

func (s *Server) LogoutAll(ctx context.Context, req *authv1.LogoutAllRequest) (*authv1.LogoutAllResponse, error) {
	if req.GetAccessToken() == "" {
		return nil, status.Error(codes.InvalidArgument, "access token is required")
	}

	if err := s.auth.LogoutAll(ctx, req.GetAccessToken()); err != nil {
		if errors.Is(err, apperrors.ErrInvalidToken) {
			return nil, status.Error(codes.Unauthenticated, "invalid access token")
		}

		return nil, status.Error(codes.Internal, "internal error")
	}

	return &authv1.LogoutAllResponse{}, nil
}

func (s *Server) OAuthStart(ctx context.Context, req *authv1.OAuthStartRequest) (*authv1.OAuthStartResponse, error) {
	if req.GetProvider() == "" {
		return nil, status.Error(codes.InvalidArgument, "provider is required")
	}

	authURL, err := s.oauth.Start(ctx, req.GetProvider())
	if err != nil {
		if errors.Is(err, apperrors.ErrUnsupportedOAuthProvider) {
			return nil, status.Error(codes.InvalidArgument, "unsupported oauth provider")
		}

		return nil, status.Error(codes.Internal, "internal error")
	}

	return &authv1.OAuthStartResponse{AuthUrl: authURL}, nil
}

func (s *Server) OAuthCallback(
	ctx context.Context,
	req *authv1.OAuthCallbackRequest,
) (*authv1.OAuthCallbackResponse, error) {
	if req.GetCode() == "" {
		return nil, status.Error(codes.InvalidArgument, "code is required")
	}
	if req.GetState() == "" {
		return nil, status.Error(codes.InvalidArgument, "state is required")
	}

	out, err := s.oauth.Callback(ctx, req.GetCode(), req.GetState())
	if err != nil {
		switch {
		case errors.Is(err, apperrors.ErrInvalidOAuthState):
			return nil, status.Error(codes.Unauthenticated, "invalid or expired oauth state")
		case errors.Is(err, apperrors.ErrInvalidOAuthCode):
			return nil, status.Error(codes.Unauthenticated, "invalid oauth authorization code")
		case errors.Is(err, apperrors.ErrOAuthEmailRequired):
			return nil, status.Error(codes.Unauthenticated, "oauth provider did not return email")
		case errors.Is(err, apperrors.ErrUnsupportedOAuthProvider):
			return nil, status.Error(codes.InvalidArgument, "unsupported oauth provider")
		case errors.Is(err, apperrors.ErrUserAlreadyExists):
			return nil, status.Error(codes.AlreadyExists, "user already exists")
		}

		return nil, status.Error(codes.Internal, "internal error")
	}

	return oauthOutputToResponse(out), nil
}

func registerRequestToInput(req *authv1.RegisterRequest) usecase.RegisterUserInput {
	return usecase.RegisterUserInput{
		Name:        req.GetName(),
		Surname:     req.GetSurname(),
		Email:       req.GetEmail(),
		PhoneNumber: req.GetPhoneNumber(),
		Password:    req.GetPassword(),
	}
}

func registerOutputToResponse(out usecase.RegisterUserOutput) *authv1.RegisterResponse {
	return &authv1.RegisterResponse{
		UserId: out.UserID.String(),
		Token:  tokenToProto(out.Token),
	}
}

func loginRequestToInput(req *authv1.LoginRequest) usecase.LoginInput {
	return usecase.LoginInput{
		Email:    req.GetEmail(),
		Password: req.GetPassword(),
	}
}

func loginOutputToResponse(out usecase.LoginOutput) *authv1.LoginResponse {
	return &authv1.LoginResponse{
		UserId: out.UserID.String(),
		Token:  tokenToProto(out.Token),
	}
}

func changePasswordRequestToInput(req *authv1.ChangePasswordRequest) usecase.ChangePasswordInput {
	return usecase.ChangePasswordInput{
		AccessToken: req.GetAccessToken(),
		OldPassword: req.GetOldPassword(),
		NewPassword: req.GetNewPassword(),
	}
}

func changePasswordTokenToResponse(token usecase.Token) *authv1.ChangePasswordResponse {
	return &authv1.ChangePasswordResponse{
		Token: tokenToProto(token),
	}
}

func claimsToValidateTokenResponse(claims usecase.TokenClaims) *authv1.ValidateTokenResponse {
	return &authv1.ValidateTokenResponse{
		UserId:    claims.UserID.String(),
		Email:     claims.Email,
		ExpiresAt: timestamppb.New(claims.ExpiresAt),
		SessionId: claims.SessionID.String(),
	}
}

func oauthOutputToResponse(out usecase.OAuthLoginOutput) *authv1.OAuthCallbackResponse {
	return &authv1.OAuthCallbackResponse{
		UserId: out.UserID.String(),
		Email:  out.Email,
		Token:  tokenToProto(out.Token),
	}
}

func tokenToProto(token usecase.Token) *authv1.Token {
	pb := &authv1.Token{
		AccessToken: token.AccessToken,
		ExpiresAt:   timestamppb.New(token.ExpiresAt),
	}
	if token.RefreshToken != "" {
		pb.RefreshToken = token.RefreshToken
		pb.RefreshExpiresAt = timestamppb.New(token.RefreshExpiresAt)
	}

	return pb
}

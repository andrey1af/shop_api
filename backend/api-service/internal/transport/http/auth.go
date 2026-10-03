package http

import (
	"context"
	jsonv2 "encoding/json/v2"
	"errors"
	"net/http"
	"time"
	"unicode/utf8"
	"uuid"

	apperrors "github.com/andrey1af/shop-api/backend/api-service/internal/errors"
	"github.com/andrey1af/shop-api/backend/api-service/internal/transport/http/middleware"
	"github.com/andrey1af/shop-api/backend/api-service/internal/usecase"
	"github.com/andrey1af/shop-api/backend/api-service/pkg/validation"
)

type authUseCase interface {
	Register(ctx context.Context, in usecase.RegisterUserInput) (usecase.RegisterUserOutput, error)
	Login(ctx context.Context, in usecase.LoginInput) (usecase.LoginOutput, error)
	ChangePassword(ctx context.Context, in usecase.ChangePasswordInput) (usecase.Token, error)
	ResetPassword(ctx context.Context, email string) error
	OAuthStart(ctx context.Context, provider string) (string, error)
	OAuthCallback(ctx context.Context, in usecase.OAuthCallbackInput) (usecase.OAuthLoginOutput, error)
	Refresh(ctx context.Context, refreshToken string) (usecase.Token, error)
	Logout(ctx context.Context, in usecase.LogoutInput) error
	LogoutAll(ctx context.Context, accessToken string) error
}

const minPasswordLength = 8

const (
	refreshCookieName = "refresh_token"

	refreshCookiePath = "/api/v1/auth"
)

type AuthHandler struct {
	authUseCase         authUseCase
	refreshCookieSecure bool
}

func NewAuthHandler(authUseCase authUseCase, refreshCookieSecure bool) *AuthHandler {
	return &AuthHandler{
		authUseCase:         authUseCase,
		refreshCookieSecure: refreshCookieSecure,
	}
}

type registerRequest struct {
	Email       string `json:"email"`
	Name        string `json:"name"`
	Surname     string `json:"surname"`
	PhoneNumber string `json:"phone_number"`
	Password    string `json:"password"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type changePasswordRequest struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

type resetPasswordRequest struct {
	Email string `json:"email"`
}

type oauthCallbackRequest struct {
	Code  string `json:"code"`
	State string `json:"state"`
}

type oauthStartResponse struct {
	AuthURL string `json:"auth_url"`
}

type oauthLoginResponse struct {
	UserID uuid.UUID     `json:"user_id"`
	Email  string        `json:"email"`
	Token  tokenResponse `json:"token"`
}

type tokenResponse struct {
	AccessToken      string     `json:"access_token"`
	ExpiresAt        time.Time  `json:"expires_at"`
	RefreshToken     string     `json:"refresh_token,omitempty"`
	RefreshExpiresAt *time.Time `json:"refresh_expires_at,omitempty"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type registerResponse struct {
	UserID uuid.UUID     `json:"user_id"`
	Token  tokenResponse `json:"token"`
}

type loginResponse struct {
	UserID uuid.UUID     `json:"user_id"`
	Token  tokenResponse `json:"token"`
}

type changePasswordResponse struct {
	Token tokenResponse `json:"token"`
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := jsonv2.UnmarshalRead(r.Body, &req); err != nil {
		writeError(w, http.StatusBadRequest, errCodeInvalidRequest, errMessageInvalidRequestBody)
		return
	}

	if !validation.IsValidEmail(req.Email) {
		writeError(w, http.StatusBadRequest, errCodeValidationError, errMessageInvalidEmail)
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, errCodeValidationError, errMessageInvalidUserName)
		return
	}
	if req.Surname == "" {
		writeError(w, http.StatusBadRequest, errCodeValidationError, errMessageInvalidUserSurname)
		return
	}
	if !validation.IsValidPhoneNumber(req.PhoneNumber) {
		writeError(w, http.StatusBadRequest, errCodeValidationError, errMessageInvalidPhoneNumber)
		return
	}
	if req.Password == "" {
		writeError(w, http.StatusBadRequest, errCodeValidationError, errMessageInvalidPassword)
		return
	}
	if utf8.RuneCountInString(req.Password) < minPasswordLength {
		writeError(w, http.StatusBadRequest, errCodeValidationError, errMessageWeakPassword)
		return
	}

	out, err := h.authUseCase.Register(r.Context(), usecase.RegisterUserInput{
		Name:        req.Name,
		Surname:     req.Surname,
		Email:       req.Email,
		PhoneNumber: req.PhoneNumber,
		Password:    req.Password,
	})
	if err != nil {
		switch {
		case errors.Is(err, apperrors.ErrUserAlreadyExists):
			writeError(w, http.StatusConflict, errCodeUserAlreadyExists, errMessageUserAlreadyExists)
		case errors.Is(err, apperrors.ErrInvalidAuthRequest):
			writeError(w, http.StatusBadRequest, errCodeValidationError, errMessageInvalidRegisterRequest)
		default:
			writeError(w, http.StatusInternalServerError, errCodeInternalError, errMessageRegisterFailed)
		}

		return
	}

	h.setRefreshCookie(w, out.Token)
	writeJSON(w, http.StatusCreated, toRegisterResponse(out))
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := jsonv2.UnmarshalRead(r.Body, &req); err != nil {
		writeError(w, http.StatusBadRequest, errCodeInvalidRequest, errMessageInvalidRequestBody)
		return
	}

	if !validation.IsValidEmail(req.Email) {
		writeError(w, http.StatusBadRequest, errCodeValidationError, errMessageInvalidEmail)
		return
	}
	if req.Password == "" {
		writeError(w, http.StatusBadRequest, errCodeValidationError, errMessageInvalidPassword)
		return
	}

	out, err := h.authUseCase.Login(r.Context(), usecase.LoginInput{
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		switch {
		case errors.Is(err, apperrors.ErrInvalidCredentials):
			writeError(w, http.StatusUnauthorized, errCodeInvalidCredentials, errMessageInvalidCredentials)
		case errors.Is(err, apperrors.ErrInvalidAuthRequest):
			writeError(w, http.StatusBadRequest, errCodeValidationError, errMessageInvalidLoginRequest)
		default:
			writeError(w, http.StatusInternalServerError, errCodeInternalError, errMessageLoginFailed)
		}

		return
	}

	h.setRefreshCookie(w, out.Token)
	writeJSON(w, http.StatusOK, toLoginResponse(out))
}

func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	accessToken, ok := middleware.BearerToken(r)
	if !ok {
		middleware.WriteUnauthorized(w)
		return
	}

	var req changePasswordRequest
	if err := jsonv2.UnmarshalRead(r.Body, &req); err != nil {
		writeError(w, http.StatusBadRequest, errCodeInvalidRequest, errMessageInvalidRequestBody)
		return
	}

	if req.OldPassword == "" {
		writeError(w, http.StatusBadRequest, errCodeValidationError, errMessageInvalidOldPassword)
		return
	}
	if req.NewPassword == "" {
		writeError(w, http.StatusBadRequest, errCodeValidationError, errMessageInvalidNewPassword)
		return
	}
	if utf8.RuneCountInString(req.NewPassword) < minPasswordLength {
		writeError(w, http.StatusBadRequest, errCodeValidationError, errMessageWeakPassword)
		return
	}

	token, err := h.authUseCase.ChangePassword(r.Context(), usecase.ChangePasswordInput{
		AccessToken: accessToken,
		OldPassword: req.OldPassword,
		NewPassword: req.NewPassword,
	})
	if err != nil {
		switch {
		case errors.Is(err, apperrors.ErrInvalidCredentials):
			writeError(w, http.StatusUnauthorized, errCodeInvalidCredentials, errMessageInvalidOldPasswordValue)
		case errors.Is(err, apperrors.ErrUserNotFound):
			writeError(w, http.StatusNotFound, errCodeNotFound, errMessageUserNotFound)
		case errors.Is(err, apperrors.ErrInvalidAuthRequest):
			writeError(w, http.StatusBadRequest, errCodeValidationError, errMessageInvalidChangePasswordRequest)
		default:
			writeError(w, http.StatusInternalServerError, errCodeInternalError, errMessageChangePasswordFailed)
		}

		return
	}

	writeJSON(w, http.StatusOK, changePasswordResponse{Token: toTokenResponse(token)})
}

func (h *AuthHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var req resetPasswordRequest
	if err := jsonv2.UnmarshalRead(r.Body, &req); err != nil {
		writeError(w, http.StatusBadRequest, errCodeInvalidRequest, errMessageInvalidRequestBody)
		return
	}

	if !validation.IsValidEmail(req.Email) {
		writeError(w, http.StatusBadRequest, errCodeValidationError, errMessageInvalidEmail)
		return
	}

	if err := h.authUseCase.ResetPassword(r.Context(), req.Email); err != nil {
		if errors.Is(err, apperrors.ErrInvalidAuthRequest) {
			writeError(w, http.StatusBadRequest, errCodeValidationError, errMessageInvalidEmail)
			return
		}
		writeError(w, http.StatusInternalServerError, errCodeInternalError, errMessageResetPasswordFailed)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *AuthHandler) OAuthStart(w http.ResponseWriter, r *http.Request) {
	authURL, err := h.authUseCase.OAuthStart(r.Context(), r.PathValue("provider"))
	if err != nil {
		if errors.Is(err, apperrors.ErrInvalidAuthRequest) {
			writeError(w, http.StatusBadRequest, errCodeValidationError, errMessageUnsupportedOAuthProvider)
			return
		}
		writeError(w, http.StatusInternalServerError, errCodeInternalError, errMessageOAuthStartFailed)
		return
	}

	writeJSON(w, http.StatusOK, oauthStartResponse{AuthURL: authURL})
}

func (h *AuthHandler) OAuthCallback(w http.ResponseWriter, r *http.Request) {
	var req oauthCallbackRequest
	if err := jsonv2.UnmarshalRead(r.Body, &req); err != nil {
		writeError(w, http.StatusBadRequest, errCodeInvalidRequest, errMessageInvalidRequestBody)
		return
	}

	if req.Code == "" || req.State == "" {
		writeError(w, http.StatusBadRequest, errCodeValidationError, errMessageInvalidOAuthCallback)
		return
	}

	out, err := h.authUseCase.OAuthCallback(r.Context(), usecase.OAuthCallbackInput{
		Code:  req.Code,
		State: req.State,
	})
	if err != nil {
		switch {
		case errors.Is(err, apperrors.ErrOAuthFailed):
			writeError(w, http.StatusUnauthorized, errCodeOAuthFailed, errMessageOAuthFailed)
		case errors.Is(err, apperrors.ErrUserAlreadyExists):
			writeError(w, http.StatusConflict, errCodeUserAlreadyExists, errMessageOAuthUserAlreadyExists)
		case errors.Is(err, apperrors.ErrInvalidAuthRequest):
			writeError(w, http.StatusBadRequest, errCodeValidationError, errMessageInvalidOAuthCallback)
		default:
			writeError(w, http.StatusInternalServerError, errCodeInternalError, errMessageOAuthCallbackFailed)
		}

		return
	}

	h.setRefreshCookie(w, out.Token)
	writeJSON(w, http.StatusOK, oauthLoginResponse{
		UserID: out.UserID,
		Email:  out.Email,
		Token:  toTokenResponse(out.Token),
	})
}

func toLoginResponse(out usecase.LoginOutput) loginResponse {
	return loginResponse{
		UserID: out.UserID,
		Token:  toTokenResponse(out.Token),
	}
}

func toTokenResponse(token usecase.Token) tokenResponse {
	resp := tokenResponse{
		AccessToken: token.AccessToken,
		ExpiresAt:   token.ExpiresAt,
	}
	if token.RefreshToken != "" {
		resp.RefreshToken = token.RefreshToken
		resp.RefreshExpiresAt = &token.RefreshExpiresAt
	}

	return resp
}

func toRegisterResponse(out usecase.RegisterUserOutput) registerResponse {
	return registerResponse{
		UserID: out.UserID,
		Token:  toTokenResponse(out.Token),
	}
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	refreshToken := h.refreshTokenFromRequest(r)
	if refreshToken == "" {
		writeError(w, http.StatusBadRequest, errCodeValidationError, errMessageMissingRefreshToken)
		return
	}

	token, err := h.authUseCase.Refresh(r.Context(), refreshToken)
	if err != nil {
		if errors.Is(err, apperrors.ErrInvalidToken) {
			h.clearRefreshCookie(w)
			writeError(w, http.StatusUnauthorized, errCodeInvalidRefresh, errMessageInvalidRefreshToken)
			return
		}
		writeError(w, http.StatusInternalServerError, errCodeInternalError, errMessageRefreshFailed)
		return
	}

	h.setRefreshCookie(w, token)
	writeJSON(w, http.StatusOK, toTokenResponse(token))
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	refreshToken := h.refreshTokenFromRequest(r)
	accessToken, _ := middleware.BearerToken(r)

	h.clearRefreshCookie(w)

	if refreshToken == "" && accessToken == "" {
		writeError(w, http.StatusBadRequest, errCodeValidationError, errMessageMissingRefreshToken)
		return
	}

	err := h.authUseCase.Logout(r.Context(), usecase.LogoutInput{
		RefreshToken: refreshToken,
		AccessToken:  accessToken,
	})
	if err != nil && !errors.Is(err, apperrors.ErrInvalidToken) {
		writeError(w, http.StatusInternalServerError, errCodeInternalError, errMessageLogoutFailed)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *AuthHandler) LogoutAll(w http.ResponseWriter, r *http.Request) {
	accessToken, ok := middleware.BearerToken(r)
	if !ok {
		middleware.WriteUnauthorized(w)
		return
	}

	if err := h.authUseCase.LogoutAll(r.Context(), accessToken); err != nil {
		if errors.Is(err, apperrors.ErrInvalidToken) {
			middleware.WriteUnauthorized(w)
			return
		}
		writeError(w, http.StatusInternalServerError, errCodeInternalError, errMessageLogoutFailed)
		return
	}

	h.clearRefreshCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (h *AuthHandler) refreshTokenFromRequest(r *http.Request) string {
	if cookie, err := r.Cookie(refreshCookieName); err == nil && cookie.Value != "" {
		return cookie.Value
	}

	var req refreshRequest
	if err := jsonv2.UnmarshalRead(r.Body, &req); err != nil {
		return ""
	}

	return req.RefreshToken
}

func (h *AuthHandler) setRefreshCookie(w http.ResponseWriter, token usecase.Token) {
	if token.RefreshToken == "" {
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     refreshCookieName,
		Value:    token.RefreshToken,
		Path:     refreshCookiePath,
		Expires:  token.RefreshExpiresAt,
		HttpOnly: true,
		Secure:   h.refreshCookieSecure,
		SameSite: http.SameSiteStrictMode,
	})
}

func (h *AuthHandler) clearRefreshCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     refreshCookieName,
		Value:    "",
		Path:     refreshCookiePath,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.refreshCookieSecure,
		SameSite: http.SameSiteStrictMode,
	})
}

package middleware

import (
	"context"
	jsonv2 "encoding/json/v2"
	"errors"
	"net/http"
	"strings"

	apperrors "github.com/andrey1af/shop-api/backend/api-service/internal/errors"
	"github.com/andrey1af/shop-api/backend/api-service/internal/usecase"
)

const bearerPrefix = "Bearer "

const (
	errCodeUnauthorized  = "UNAUTHORIZED"
	errCodeInternalError = "INTERNAL_ERROR"

	errMessageUnauthorized        = "missing or invalid access token"
	errMessageValidateTokenFailed = "failed to validate access token"
)

type TokenValidator interface {
	ValidateToken(ctx context.Context, accessToken string) (usecase.TokenClaims, error)
}

type errorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func Auth(validator TokenValidator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			accessToken, ok := BearerToken(r)
			if !ok {
				WriteUnauthorized(w)
				return
			}

			if _, err := validator.ValidateToken(r.Context(), accessToken); err != nil {
				if errors.Is(err, apperrors.ErrInvalidToken) || errors.Is(err, apperrors.ErrInvalidAuthRequest) {
					WriteUnauthorized(w)
					return
				}
				writeError(w, http.StatusInternalServerError, errCodeInternalError, errMessageValidateTokenFailed)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func BearerToken(r *http.Request) (string, bool) {
	header := r.Header.Get("Authorization")
	if len(header) < len(bearerPrefix) || !strings.EqualFold(header[:len(bearerPrefix)], bearerPrefix) {
		return "", false
	}

	token := strings.TrimSpace(header[len(bearerPrefix):])

	return token, token != ""
}

func WriteUnauthorized(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", "Bearer")
	writeError(w, http.StatusUnauthorized, errCodeUnauthorized, errMessageUnauthorized)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = jsonv2.MarshalWrite(w, errorResponse{Code: code, Message: message})
}

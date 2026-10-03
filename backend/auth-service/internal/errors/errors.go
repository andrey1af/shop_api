package errors

import "errors"

var ErrUserNotFound = errors.New("user not found")

var ErrUserAlreadyExists = errors.New("user already exists")

var ErrInvalidToken = errors.New("invalid token")

var ErrInvalidCredentials = errors.New("invalid credentials")

var ErrUnsupportedOAuthProvider = errors.New("unsupported oauth provider")

var ErrInvalidOAuthState = errors.New("invalid oauth state")

var ErrInvalidOAuthCode = errors.New("invalid oauth authorization code")

var ErrOAuthEmailRequired = errors.New("oauth provider did not return email")

var ErrSessionNotFound = errors.New("session not found")

var ErrRefreshTokenNotFound = errors.New("refresh token not found")

var ErrRefreshTokenReused = errors.New("refresh token reused")

var ErrWeakPassword = errors.New("password is too short")

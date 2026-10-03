package errors

import "errors"

var ErrUserAlreadyExists = errors.New("user already exists")

var ErrInvalidAuthRequest = errors.New("invalid auth request")

var ErrInvalidCredentials = errors.New("invalid credentials")

var ErrInvalidToken = errors.New("invalid token")

var ErrUserNotFound = errors.New("user not found")

var ErrOAuthFailed = errors.New("oauth authorization failed")

var ErrInvalidProductUpdate = errors.New("invalid product update")

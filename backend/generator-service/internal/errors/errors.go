package errors

import "errors"

var ErrNoProducts = errors.New("no products to update")

var ErrInvalidCredentials = errors.New("invalid credentials")

var ErrUnauthorized = errors.New("unauthorized")

var ErrUnexpectedStatus = errors.New("unexpected response status")

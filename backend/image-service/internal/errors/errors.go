package errors

import "errors"

var ErrImageNotFound = errors.New("image not found")

var ErrImageAlreadyExists = errors.New("image already exists")

var ErrEmptyImageData = errors.New("image data must not be empty")

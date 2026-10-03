package domain

import "errors"

var ErrClientNotFound = errors.New("client not found")

var ErrSupplierNotFound = errors.New("supplier not found")

var ErrSupplierInUse = errors.New("supplier is used by products")

var ErrProductNotFound = errors.New("product not found")

var ErrInsufficientStock = errors.New("insufficient stock")

var ErrImageNotFound = errors.New("image not found")

var ErrImageAlreadyExists = errors.New("product already has an image")

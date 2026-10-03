package domain

import (
	"time"
	"uuid"
)

type Product struct {
	ID             uuid.UUID
	Name           string
	Category       string
	Price          float64
	AvailableStock int64
	LastUpdateDate time.Time
	SupplierID     uuid.UUID
	ImageID        *uuid.UUID
}

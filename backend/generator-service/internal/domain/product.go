package domain

import (
	"time"
	"uuid"
)

type Product struct {
	ID             uuid.UUID
	Price          float64
	AvailableStock int64
}

type ProductUpdated struct {
	EventID        uuid.UUID
	ProductID      uuid.UUID
	Price          float64
	AvailableStock int64
	OccurredAt     time.Time
}

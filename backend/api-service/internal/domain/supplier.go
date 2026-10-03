package domain

import "uuid"

type Supplier struct {
	ID          uuid.UUID
	Name        string
	PhoneNumber string
	Address     Address
}

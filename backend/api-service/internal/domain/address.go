package domain

import "uuid"

type Address struct {
	ID      uuid.UUID
	Country string
	City    string
	Street  string
}

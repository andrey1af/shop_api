package domain

import (
	"time"
	"uuid"
)

type User struct {
	ID           uuid.UUID
	Name         string
	Surname      string
	Email        string
	PhoneNumber  string
	PasswordHash []byte
	CreatedAt    time.Time
}

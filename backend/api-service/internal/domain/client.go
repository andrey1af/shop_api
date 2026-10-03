package domain

import (
	"time"
	"uuid"
)

type Client struct {
	ID               uuid.UUID
	Name             string
	Surname          string
	Birthday         time.Time
	Gender           GenderType
	RegistrationTime time.Time
	Address          Address
}

type GenderType string

const (
	GenderMale   GenderType = "male"
	GenderFemale GenderType = "female"
)

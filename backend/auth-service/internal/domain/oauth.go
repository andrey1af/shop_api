package domain

import (
	"time"
	"uuid"
)

type UserIdentity struct {
	ID             uuid.UUID
	UserID         uuid.UUID
	Provider       string
	ProviderUserID string
	Email          string
	CreatedAt      time.Time
}

type OAuthState struct {
	State        string
	Provider     string
	CodeVerifier string
	ExpiresAt    time.Time
}

type OAuthProfile struct {
	ProviderUserID string
	Email          string

	EmailVerified bool
	Name          string
	Surname       string
	PhoneNumber   string
}

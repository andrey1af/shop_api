package domain

import "uuid"

type Image struct {
	ID          uuid.UUID
	Data        []byte
	ContentType string
}

package validation

import "net/mail"

const maxEmailLength = 254

func IsValidEmail(value string) bool {
	if len(value) > maxEmailLength {
		return false
	}

	addr, err := mail.ParseAddress(value)

	return err == nil && addr.Address == value
}

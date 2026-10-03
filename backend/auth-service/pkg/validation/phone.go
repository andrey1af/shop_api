package validation

import "regexp"

var phoneNumberPattern = regexp.MustCompile(`^\+[1-9][0-9]{7,14}$`)

func IsValidPhoneNumber(value string) bool {
	return phoneNumberPattern.MatchString(value)
}

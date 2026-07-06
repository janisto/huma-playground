package profile

import "strings"

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func normalizePhoneNumber(phoneNumber string) string {
	return strings.TrimSpace(phoneNumber)
}

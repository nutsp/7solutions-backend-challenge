package domain

import (
	"encoding/hex"
	"net/mail"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Core invariants protect persistence even when requests arrive directly over gRPC.
func NormalizeName(raw string) (string, bool) {
	value := strings.TrimSpace(raw)
	return value, utf8.ValidString(value) && utf8.RuneCountInString(value) >= 1 && utf8.RuneCountInString(value) <= 100 && !strings.ContainsFunc(value, unicode.IsControl)
}

func NormalizeEmail(raw string) (string, bool) {
	value := strings.ToLower(strings.TrimSpace(raw))
	address, err := mail.ParseAddress(value)
	return value, utf8.ValidString(value) && len(value) <= 254 && !strings.ContainsFunc(value, unicode.IsControl) && err == nil && address.Address == value
}

func ValidPassword(value string) bool {
	return utf8.ValidString(value) && len(value) >= 12 && len(value) <= 72
}

func ValidUserID(value string) bool {
	if len(value) != 32 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

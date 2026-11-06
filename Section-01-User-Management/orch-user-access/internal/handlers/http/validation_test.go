package httphandler

import (
	"errors"
	"strings"
	"testing"

	apperrors "github.com/nutsp/7solutions-backend-challenge/orch-user-access/pkg/errors"
)

func TestRegistrationFieldRules(t *testing.T) {
	tests := []struct {
		name     string
		userName string
		email    string
		password string
		valid    bool
	}{
		{"normalize fields", " Alice ", " ALICE@Example.com ", "correct-horse-battery", true},
		{
			"Unicode name and minimum password bytes",
			"ชื่อภาษาไทย",
			"alice@example.com",
			strings.Repeat("ก", 4),
			true,
		},
		{"maximum password bytes", "Alice", "alice@example.com", strings.Repeat("ก", 24), true},
		{"password exceeds bcrypt byte limit", "Alice", "alice@example.com", strings.Repeat("ก", 25), false},
		{"short password", "Alice", "alice@example.com", "short", false},
		{"invalid UTF8 password", "Alice", "alice@example.com", string([]byte{0xff}) + strings.Repeat("a", 12), false},
		{"required name", "  ", "alice@example.com", "correct-horse-battery", false},
		{"name control character", "Alice\x00", "alice@example.com", "correct-horse-battery", false},
		{"overlong Unicode name", strings.Repeat("ก", 101), "alice@example.com", "correct-horse-battery", false},
		{"invalid email", "Alice", "invalid", "correct-horse-battery", false},
		{"display email", "Alice", "Alice <alice@example.com>", "correct-horse-battery", false},
		{"email injection", "Alice", "alice@example.com\r\nInjected: yes", "correct-horse-battery", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := registerJSON{
				Name:     tt.userName,
				Email:    tt.email,
				Password: tt.password,
			}
			err := validateRequest(newRequestValidator(), &input)
			if tt.valid {
				if err != nil {
					t.Fatal(err)
				}
				if input.Name != strings.TrimSpace(tt.userName) || input.Email != strings.ToLower(strings.TrimSpace(tt.email)) || input.Password != tt.password {
					t.Fatal("incorrect normalization")
				}
			} else if !errors.Is(err, apperrors.ErrInvalid) {
				t.Fatalf("invalid request accepted: %v", err)
			}
		})
	}
}

func TestPatchFieldRules(t *testing.T) {
	tests := []struct {
		name     string
		userName *string
		email    *string
		valid    bool
	}{
		{
			name: "empty patch",
		}, {
			name:     "empty supplied name",
			userName: textPointer(""),
		},
		{
			name:  "empty supplied email",
			email: textPointer(""),
		},
		{
			name:     "name only",
			userName: textPointer(" Alice "),
			valid:    true,
		},
		{
			name:  "email only",
			email: textPointer(" ALICE@example.com "),
			valid: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := patchJSON{
				Name:  tt.userName,
				Email: tt.email,
			}
			err := validateRequest(newRequestValidator(), &input)
			if (err == nil) != tt.valid {
				t.Fatalf("valid=%v error=%v", tt.valid, err)
			}
			if err == nil && input.Name != nil && *input.Name != "Alice" {
				t.Fatal("name not normalized")
			}
			if err == nil && input.Email != nil && *input.Email != "alice@example.com" {
				t.Fatal("email not normalized")
			}
		})
	}
}

func TestUserIDFieldRule(t *testing.T) {
	tests := []struct {
		name  string
		id    string
		valid bool
	}{
		{"valid", strings.Repeat("a", 32), true}, {"short", "bad", false},
		{"prefixed", "0x" + strings.Repeat("a", 30), false}, {"missing", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validUserID(newRequestValidator(), tt.id); got != tt.valid {
				t.Fatalf("valid=%v; want %v", got, tt.valid)
			}
		})
	}
}

func textPointer(value string) *string {

	return &value
}

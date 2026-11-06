package hash

import (
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestHashAndCompare(t *testing.T) {
	tests := []struct {
		name      string
		password  string
		candidate string
		valid     bool
	}{
		{"ASCII", "correct-horse-battery", "correct-horse-battery", true},
		{"Unicode", "รหัสผ่านภาษาไทย", "รหัสผ่านภาษาไทย", true},
		{"wrong password", "correct-horse-battery", "wrong", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, err := New(Config{})
			if err != nil {
				t.Fatal(err)
			}
			encoded, err := h.Hash(tt.password)
			if err != nil {
				t.Fatal(err)
			}
			cost, err := bcrypt.Cost([]byte(encoded))
			if err != nil || cost != 10 {
				t.Fatalf("bcrypt cost=%d error=%v", cost, err)
			}
			if err := h.Compare(encoded, tt.candidate); (err == nil) != tt.valid {
				t.Fatalf("valid=%v error=%v", tt.valid, err)
			}
		})
	}
}

func TestConfiguration(t *testing.T) {
	tests := []struct {
		name  string
		cost  int
		valid bool
	}{{"default", 0, true}, {"required cost", 10, true}, {"too small", 1, false}, {"too large", 32, false}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := New(Config{
				Cost: tt.cost,
			}); (err == nil) != tt.valid {
				t.Fatalf("valid=%v error=%v", tt.valid, err)
			}
		})
	}
}

func TestPasswordByteLimit(t *testing.T) {
	tests := []struct {
		name     string
		password string
		valid    bool
	}{{"72 bytes", strings.Repeat("ก", 24), true}, {"75 bytes", strings.Repeat("ก", 25), false}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, err := New(Config{})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := h.Hash(tt.password); (err == nil) != tt.valid {
				t.Fatalf("valid=%v error=%v", tt.valid, err)
			}
		})
	}
}

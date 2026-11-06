package cryptography

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestAuthenticatedEncryption(t *testing.T) {
	tests := []struct {
		name       string
		tamper     bool
		wrongAAD   bool
		unknownKey bool
		valid      bool
	}{
		{
			name:  "round trip",
			valid: true,
		},
		{
			name:   "tampered ciphertext",
			tamper: true,
		},
		{
			name:     "wrong record binding",
			wrongAAD: true,
		},
		{
			name:       "unknown key",
			unknownKey: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := New(Config{
				ActiveKeyID: "v1",
				Keys:        map[string][]byte{"v1": bytes.Repeat([]byte{1}, 32)},
			})
			if err != nil {
				t.Fatal(err)
			}
			aad := []byte("user-1:name")
			first, err := c.Encrypt([]byte("sensitive name"), aad)
			if err != nil {
				t.Fatal(err)
			}
			second, _ := c.Encrypt([]byte("sensitive name"), aad)
			if bytes.Equal(first, second) {
				t.Fatal("nonce reused")
			}
			var value envelope
			if err := json.Unmarshal(first, &value); err != nil {
				t.Fatal(err)
			}
			if tt.tamper {
				value.Ciphertext[0] ^= 1
			}
			if tt.unknownKey {
				value.KeyID = "missing"
			}
			first, _ = json.Marshal(value)
			if tt.wrongAAD {
				aad = []byte("user-2:name")
			}
			plain, err := c.Decrypt(first, aad)
			if (err == nil) != tt.valid {
				t.Fatalf("valid=%v error=%v", tt.valid, err)
			}
			if tt.valid && string(plain) != "sensitive name" {
				t.Fatal("plaintext changed")
			}
		})
	}
}

func TestRotationAndLookup(t *testing.T) {
	tests := []struct {
		name  string
		key   []byte
		valid bool
	}{{"valid", bytes.Repeat([]byte{7}, 32), true}, {"invalid key", []byte("short"), false}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := New(Config{
				ActiveKeyID: "v1",
				Keys:        map[string][]byte{"v1": tt.key},
			})
			if (err == nil) != tt.valid {
				t.Fatal("key validation failed")
			}
			if !tt.valid {
				return
			}
			raw, _ := c.Encrypt([]byte("email"), []byte("binding"))
			rotated, err := New(Config{
				ActiveKeyID: "v2",
				Keys:        map[string][]byte{"v1": tt.key, "v2": bytes.Repeat([]byte{8}, 32)},
			})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := rotated.Decrypt(raw, []byte("binding")); err != nil {
				t.Fatal("old key unreadable")
			}
			lookup, err := NewLookup(LookupConfig{
				Key: tt.key,
			})
			if err != nil {
				t.Fatal(err)
			}
			if lookup.Lookup([]byte("email")) != lookup.Lookup([]byte("email")) || lookup.Lookup([]byte("email")) == lookup.Lookup([]byte("other")) {
				t.Fatal("invalid blind index")
			}
		})
	}
}

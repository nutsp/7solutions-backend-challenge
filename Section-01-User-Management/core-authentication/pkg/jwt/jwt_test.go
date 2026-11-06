package jwt

import (
	"errors"
	"strings"
	"testing"
	"time"

	jwt "github.com/golang-jwt/jwt/v5"

	apperrors "github.com/nutsp/7solutions-backend-challenge/core-authentication/pkg/errors"
)

func TestJWT(t *testing.T) {
	s, err := New(Config{
		Secret:   strings.Repeat("x", 32),
		Issuer:   "auth",
		Audience: "orch",
		TTL:      time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	base := tokenClaims{
		SessionID: "session",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user",
			Issuer:    "auth",
			Audience:  jwt.ClaimStrings{"orch"},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
		},
	}
	tests := []struct {
		name   string
		method jwt.SigningMethod
		change func(*tokenClaims)
	}{
		{"valid", jwt.SigningMethodHS256, func(*tokenClaims) {}},
		{"expiry required", jwt.SigningMethodHS256, func(c *tokenClaims) {

			c.ExpiresAt = nil
		}},
		{
			"expired",
			jwt.SigningMethodHS256,
			func(c *tokenClaims) {

				c.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-time.Minute))
			},
		},
		{"wrong algorithm", jwt.SigningMethodHS384, func(*tokenClaims) {}},
		{"wrong issuer", jwt.SigningMethodHS256, func(c *tokenClaims) {

			c.Issuer = "other"
		}},
		{"wrong audience", jwt.SigningMethodHS256, func(c *tokenClaims) {

			c.Audience = jwt.ClaimStrings{"other"}
		}},
		{"session required", jwt.SigningMethodHS256, func(c *tokenClaims) {

			c.SessionID = ""
		}},
		{"subject required", jwt.SigningMethodHS256, func(c *tokenClaims) {

			c.Subject = ""
		}},
		{
			"future not before",
			jwt.SigningMethodHS256,
			func(c *tokenClaims) {

				c.NotBefore = jwt.NewNumericDate(time.Now().Add(time.Hour))
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			value := base
			tc.change(&value)
			raw, err := jwt.NewWithClaims(tc.method, value).SignedString(s.key)
			if err != nil {
				t.Fatal(err)
			}
			got, err := s.Parse(raw)
			if tc.name == "valid" {
				if err != nil || got.UserID != "user" || got.SessionID != "session" {
					t.Fatalf("JWT round trip: %+v %v", got, err)
				}
			} else if !errors.Is(err, apperrors.ErrUnauthorized) {
				t.Fatalf("bad JWT accepted: %v", err)
			}
		})
	}
}

func TestSecurityConfiguration(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		valid bool
	}{
		{"weak key", "short", false}, {"valid key", strings.Repeat("x", 32), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := New(Config{
				Secret:   tt.key,
				Issuer:   "auth",
				Audience: "orch",
				TTL:      time.Minute,
			})
			if (err == nil) != tt.valid {
				t.Fatalf("valid=%v error=%v", tt.valid, err)
			}
			if !tt.valid {
				return
			}
			raw, _, err := s.Issue(Claims{
				UserID:    "user",
				SessionID: "session",
			})
			if err != nil {
				t.Fatal(err)
			}
			claims, err := s.Parse(raw)
			if err != nil || claims.UserID != "user" || claims.SessionID != "session" {
				t.Fatalf("issued JWT: %+v %v", claims, err)
			}
		})
	}
}

func TestInvalidTokens(t *testing.T) {
	manager, err := New(Config{
		Secret:   strings.Repeat("x", 32),
		Issuer:   "auth",
		Audience: "orch",
		TTL:      time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	other, err := New(Config{
		Secret:   strings.Repeat("y", 32),
		Issuer:   "auth",
		Audience: "orch",
		TTL:      time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	wrongKeyToken, _, err := other.Issue(Claims{
		UserID:    "user",
		SessionID: "session",
	})
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		raw  string
	}{
		{"empty token", ""}, {"oversized token", strings.Repeat("x", 8193)},
		{"malformed token", "not.a.jwt"}, {"different signing key", wrongKeyToken},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := manager.Parse(tt.raw); !errors.Is(err, apperrors.ErrUnauthorized) {
				t.Fatalf("invalid token accepted: %v", err)
			}
		})
	}
}

func TestIssueRequiresIdentity(t *testing.T) {
	tests := []struct {
		name    string
		claims  Claims
		wantErr error
	}{
		{"valid", Claims{
			UserID:    "user",
			SessionID: "session",
		}, nil},
		{"missing subject", Claims{
			SessionID: "session",
		}, apperrors.ErrInvalid},
		{"missing session", Claims{
			UserID: "user",
		}, apperrors.ErrInvalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager, err := New(Config{
				Secret:   strings.Repeat("x", 32),
				Issuer:   "auth",
				Audience: "orch",
				TTL:      time.Minute,
			})
			if err != nil {
				t.Fatal(err)
			}
			raw, _, err := manager.Issue(tt.claims)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("wrong result: %v", err)
			}
			if tt.wantErr != nil && raw != "" {
				t.Fatal("invalid identity received a token")
			}
		})
	}
}

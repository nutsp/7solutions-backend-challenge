// Package jwt signs and verifies HS256 access tokens independently of application domains.
package jwt

import (
	"fmt"
	"time"

	jwtlib "github.com/golang-jwt/jwt/v5"

	apperrors "github.com/nutsp/7solutions-backend-challenge/orch-user-access/pkg/errors"
)

type Config struct {
	Secret   string `json:"-"`
	Issuer   string
	Audience string
	TTL      time.Duration
}

type Claims struct {
	UserID    string
	SessionID string
}

type Manager struct {
	key      []byte
	issuer   string
	audience string
	ttl      time.Duration
}

type tokenClaims struct {
	SessionID string `json:"sid"`
	jwtlib.RegisteredClaims
}

func New(cfg Config) (*Manager, error) {
	if len(cfg.Secret) < 32 || cfg.Issuer == "" || cfg.Audience == "" || cfg.TTL <= 0 {
		return nil, fmt.Errorf("strong JWT secret, issuer, audience and TTL are required")
	}
	return &Manager{
		key:      []byte(cfg.Secret),
		issuer:   cfg.Issuer,
		audience: cfg.Audience,
		ttl:      cfg.TTL,
	}, nil
}

func (m *Manager) Issue(value Claims) (string, time.Time, error) {
	if value.UserID == "" || value.SessionID == "" {
		return "", time.Time{}, apperrors.ErrInvalid
	}
	now := time.Now().UTC()
	expires := now.Add(m.ttl).Truncate(time.Second)
	c := tokenClaims{
		SessionID: value.SessionID,
		RegisteredClaims: jwtlib.RegisteredClaims{
			Issuer:    m.issuer,
			Audience:  jwtlib.ClaimStrings{m.audience},
			Subject:   value.UserID,
			IssuedAt:  jwtlib.NewNumericDate(now),
			NotBefore: jwtlib.NewNumericDate(now),
			ExpiresAt: jwtlib.NewNumericDate(expires),
		},
	}
	raw, err := jwtlib.NewWithClaims(jwtlib.SigningMethodHS256, c).SignedString(m.key)
	return raw, expires, err
}

func (m *Manager) Parse(raw string) (Claims, error) {
	if len(raw) == 0 || len(raw) > 8192 {
		return Claims{}, apperrors.ErrUnauthorized
	}
	c := &tokenClaims{}
	token, err := jwtlib.ParseWithClaims(raw, c, func(_ *jwtlib.Token) (any, error) {

		return m.key, nil
	}, jwtlib.WithValidMethods([]string{"HS256"}), jwtlib.WithIssuer(m.issuer), jwtlib.WithAudience(m.audience), jwtlib.WithExpirationRequired(), jwtlib.WithIssuedAt())
	if err != nil || !token.Valid || c.Subject == "" || c.SessionID == "" {
		return Claims{}, apperrors.ErrUnauthorized
	}
	return Claims{
		UserID:    c.Subject,
		SessionID: c.SessionID,
	}, nil
}

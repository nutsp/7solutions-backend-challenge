package securityadapter

import (
	"time"

	domain "github.com/nutsp/7solutions-backend-challenge/core-authentication/internal/domain"
	"github.com/nutsp/7solutions-backend-challenge/core-authentication/pkg/hash"
	"github.com/nutsp/7solutions-backend-challenge/core-authentication/pkg/jwt"
)

var _ domain.AuthenticationSecurityAdapter = (*Security)(nil)

type Security struct {
	hasher *hash.Hasher
	tokens *jwt.Manager
}

func New(hashConfig hash.Config, jwtConfig jwt.Config) (*Security, error) {
	hasher, err := hash.New(hashConfig)
	if err != nil {
		return nil, err
	}
	tokens, err := jwt.New(jwtConfig)
	if err != nil {
		return nil, err
	}
	return &Security{
		hasher: hasher,
		tokens: tokens,
	}, nil
}

func (s *Security) Hash(password string) (string, error) {

	return s.hasher.Hash(password)
}

func (s *Security) Compare(encoded, password string) error {
	return s.hasher.Compare(encoded, password)
}

func (s *Security) Issue(value domain.Claims) (string, time.Time, error) {
	return s.tokens.Issue(jwt.Claims{
		UserID:    value.UserID,
		SessionID: value.SessionID,
	})
}

func (s *Security) Parse(raw string) (domain.Claims, error) {
	value, err := s.tokens.Parse(raw)
	return domain.Claims{
		UserID:    value.UserID,
		SessionID: value.SessionID,
	}, err
}

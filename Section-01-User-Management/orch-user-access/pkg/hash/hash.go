// Package hash provides bcrypt password hashing with explicit configuration.
package hash

import "golang.org/x/crypto/bcrypt"

type Config struct {
	Cost int
}

type Hasher struct {
	cost int
}

func New(cfg Config) (*Hasher, error) {
	if cfg.Cost == 0 {
		cfg.Cost = 10
	}
	if cfg.Cost < bcrypt.MinCost || cfg.Cost > bcrypt.MaxCost {
		return nil, bcrypt.InvalidCostError(cfg.Cost)
	}
	return &Hasher{
		cost: cfg.Cost,
	}, nil
}

func (h *Hasher) Hash(password string) (string, error) {
	encoded, err := bcrypt.GenerateFromPassword([]byte(password), h.cost)
	return string(encoded), err
}

func (*Hasher) Compare(encoded, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(encoded), []byte(password))
}

package domain

import (
	"time"
)

type Profile struct {
	ID string
}

type Credential struct {
	UserID       string
	PasswordHash string
}

type Session struct {
	ID          string
	UserID      string
	RefreshHash string
	ExpiresAt   time.Time
}

type Claims struct {
	UserID    string
	SessionID string
}

type Principal struct {
	UserID string
}

type Tokens struct {
	AccessToken  string
	RefreshToken string
	UserID       string
	ExpiresAt    time.Time
}

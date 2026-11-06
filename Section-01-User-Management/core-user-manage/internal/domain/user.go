package domain

import (
	"time"
)

type User struct {
	ID           string
	Name         string
	Email        string
	PasswordHash string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type Principal struct {
	UserID string
}

type Create struct {
	Name     string
	Email    string
	Password string
}

type Patch struct {
	Name  *string
	Email *string
}

type Page struct {
	Users      []User
	NextCursor string
}

package domain

import (
	"context"
	"time"
)

// CoreUserAdapter is the outbound contract for calling core-user-manage over gRPC.
type CoreUserAdapter interface {
	Get(ctx context.Context, userID string) (Profile, error)
	Credential(ctx context.Context, email string) (Credential, error)
}

// SessionRepository is the persistence port for authentication sessions, implemented by Redis.
type SessionRepository interface {
	CreateSession(ctx context.Context, session Session) error
	GetSession(ctx context.Context, userID, sessionID string) (Session, error)
	Rotate(ctx context.Context, session Session, previousHash, newHash string) error
	DeleteSession(ctx context.Context, userID, sessionID string) error
	DeleteSessions(ctx context.Context, userID string) error
}

// AuthenticationSecurityAdapter is the outbound contract for bcrypt comparison and JWT operations.
type AuthenticationSecurityAdapter interface {
	Hash(password string) (string, error)
	Compare(passwordHash, password string) error
	Issue(claims Claims) (string, time.Time, error)
	Parse(accessToken string) (Claims, error)
}

// AuthenticationService is the inbound application contract for login and session operations.
type AuthenticationService interface {
	Login(ctx context.Context, email, password string) (Tokens, error)
	Authenticate(ctx context.Context, accessToken string) (Principal, error)
	Refresh(ctx context.Context, refreshToken string) (Tokens, error)
	Logout(ctx context.Context, accessToken string) error
	AuthorizeDelete(ctx context.Context, accessToken, userID string) (Principal, error)
	RevokeUser(ctx context.Context, userID string) error
}

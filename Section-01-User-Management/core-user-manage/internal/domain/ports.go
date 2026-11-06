package domain

import (
	"context"
	"time"
)

// UserRepository is the persistence port for User records, implemented by MongoDB.
type UserRepository interface {
	Create(ctx context.Context, user User) (User, error)
	Get(ctx context.Context, userID string) (User, error)
	FindByEmail(ctx context.Context, email string) (User, error)
	List(ctx context.Context, limit int, afterID string) (Page, error)
	Update(ctx context.Context, userID string, patch Patch, updatedAt time.Time) (User, error)
	Delete(ctx context.Context, userID string) error
	Count(ctx context.Context) (int64, error)
}

// PasswordHasher is the outbound security contract for hashing passwords.
type PasswordHasher interface {
	Hash(password string) (string, error)
}

// UserService is the inbound application contract used by gRPC handlers.
type UserService interface {
	Create(ctx context.Context, input Create) (User, error)
	Get(ctx context.Context, actor Principal, userID string) (User, error)
	Internal(ctx context.Context, userID string) (User, error)
	FindByEmail(ctx context.Context, email string) (User, error)
	List(ctx context.Context, actor Principal, limit int, afterID string) (Page, error)
	Update(ctx context.Context, actor Principal, userID string, patch Patch) (User, error)
	DeleteAuthorized(ctx context.Context, actor Principal, userID string) error
	Count(ctx context.Context) (int64, error)
}

// UserAuditRepository appends committed account changes; it exposes no update/delete operations.
type UserAuditRepository interface {
	Append(ctx context.Context, audit UserAudit) error
}

// TransactionManager runs all repository operations in one atomic transaction.
// The callback may run again after a transient conflict and must have no external side effects.
type TransactionManager interface {
	WithinTransaction(ctx context.Context, run func(context.Context) error) error
}

// SensitiveDataCipher is the authenticated-encryption port consumed by Mongo document mapping.
type SensitiveDataCipher interface {
	Encrypt(plaintext, associatedData []byte) ([]byte, error)
	Decrypt(ciphertext, associatedData []byte) ([]byte, error)
}

// EmailLookupHasher computes a stable keyed lookup for normalized email bytes.
type EmailLookupHasher interface{ Lookup(value []byte) string }

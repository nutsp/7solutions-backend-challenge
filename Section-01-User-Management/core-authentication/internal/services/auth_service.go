package services

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	domain "github.com/nutsp/7solutions-backend-challenge/core-authentication/internal/domain"
	apperrors "github.com/nutsp/7solutions-backend-challenge/core-authentication/pkg/errors"
)

var _ domain.AuthenticationService = (*Auth)(nil)

type Auth struct {
	repo     domain.SessionRepository
	users    domain.CoreUserAdapter
	security domain.AuthenticationSecurityAdapter
	dummy    string
}

func New(repo domain.SessionRepository, users domain.CoreUserAdapter, security domain.AuthenticationSecurityAdapter) (*Auth, error) {
	dummy, err := security.Hash("dummy-password-for-timing-only")
	if err != nil {
		return nil, err
	}
	return &Auth{
		repo:     repo,
		users:    users,
		security: security,
		dummy:    dummy,
	}, nil
}

func (a *Auth) Login(ctx context.Context, email, password string) (domain.Tokens, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || len(email) > 254 || len(password) == 0 || len(password) > 72 {
		return domain.Tokens{}, apperrors.ErrUnauthorized
	}
	credential, err := a.users.Credential(ctx, email)
	if err != nil && !errors.Is(err, apperrors.ErrNotFound) {
		return domain.Tokens{}, err
	}
	hash := a.dummy
	if credential.PasswordHash != "" {
		hash = credential.PasswordHash
	}
	compareErr := a.security.Compare(hash, password)
	if compareErr != nil || credential.PasswordHash == "" || credential.UserID == "" {
		return domain.Tokens{}, apperrors.ErrUnauthorized
	}
	if err := ctx.Err(); err != nil {
		return domain.Tokens{}, err
	}
	sid, err := randomID()
	if err != nil {
		return domain.Tokens{}, err
	}
	refresh, err := refreshToken(credential.UserID, sid)
	if err != nil {
		return domain.Tokens{}, err
	}
	access, expires, err := a.security.Issue(domain.Claims{
		UserID:    credential.UserID,
		SessionID: sid,
	})
	if err != nil {
		return domain.Tokens{}, err
	}
	if err := a.repo.CreateSession(ctx, domain.Session{
		ID:          sid,
		UserID:      credential.UserID,
		RefreshHash: digest(refresh),
		ExpiresAt:   time.Now().UTC().Add(30 * 24 * time.Hour),
	}); err != nil {
		return domain.Tokens{}, err
	}
	return domain.Tokens{
		AccessToken:  access,
		RefreshToken: refresh,
		UserID:       credential.UserID,
		ExpiresAt:    expires,
	}, nil
}

func (a *Auth) Authenticate(ctx context.Context, raw string) (domain.Principal, error) {
	claims, _, err := a.session(ctx, raw)
	if err != nil {
		return domain.Principal{}, err
	}
	user, err := a.users.Get(ctx, claims.UserID)
	if errors.Is(err, apperrors.ErrNotFound) {
		return domain.Principal{}, apperrors.ErrUnauthorized
	}
	if err != nil {
		return domain.Principal{}, err
	}
	return domain.Principal{
		UserID: user.ID,
	}, nil
}

func (a *Auth) Refresh(ctx context.Context, raw string) (domain.Tokens, error) {
	parts := strings.Split(raw, ".")
	if len(parts) != 3 || len(parts[0]) != 32 || len(parts[1]) != 32 || len(parts[2]) != 43 {
		return domain.Tokens{}, apperrors.ErrUnauthorized
	}
	session, err := a.repo.GetSession(ctx, parts[0], parts[1])
	if errors.Is(err, apperrors.ErrNotFound) {
		return domain.Tokens{}, apperrors.ErrUnauthorized
	}
	if err != nil {
		return domain.Tokens{}, err
	}
	oldHash := digest(raw)
	if subtle.ConstantTimeCompare([]byte(session.RefreshHash), []byte(oldHash)) != 1 || !session.ExpiresAt.After(time.Now()) {
		return domain.Tokens{}, apperrors.ErrUnauthorized
	}
	if _, err := a.users.Get(ctx, session.UserID); err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			return domain.Tokens{}, apperrors.ErrUnauthorized
		}
		return domain.Tokens{}, err
	}
	refresh, err := refreshToken(session.UserID, session.ID)
	if err != nil {
		return domain.Tokens{}, err
	}
	access, expires, err := a.security.Issue(domain.Claims{
		UserID:    session.UserID,
		SessionID: session.ID,
	})
	if err != nil {
		return domain.Tokens{}, err
	}
	if err := a.repo.Rotate(ctx, session, oldHash, digest(refresh)); err != nil {
		return domain.Tokens{}, err
	}
	return domain.Tokens{
		AccessToken:  access,
		RefreshToken: refresh,
		UserID:       session.UserID,
		ExpiresAt:    expires,
	}, nil
}

func (a *Auth) Logout(ctx context.Context, raw string) error {
	claims, err := a.security.Parse(raw)
	if err != nil {
		return apperrors.ErrUnauthorized
	}
	return a.repo.DeleteSession(ctx, claims.UserID, claims.SessionID)
}

// Deletion retries may finish cleanup after the owner profile/session has been removed.
func (a *Auth) AuthorizeDelete(ctx context.Context, raw, id string) (domain.Principal, error) {
	claims, err := a.security.Parse(raw)
	if err != nil {
		return domain.Principal{}, apperrors.ErrUnauthorized
	}
	if claims.UserID != id {
		return domain.Principal{}, apperrors.ErrForbidden
	}
	_, err = a.users.Get(ctx, id)
	if errors.Is(err, apperrors.ErrNotFound) {
		return domain.Principal{
			UserID: id,
		}, nil
	}
	if err != nil {
		return domain.Principal{}, err
	}
	return a.Authenticate(ctx, raw)
}

func (a *Auth) RevokeUser(ctx context.Context, id string) error {
	return a.repo.DeleteSessions(ctx, id)
}

func digest(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func randomID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func refreshToken(uid, sid string) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return uid + "." + sid + "." + base64.RawURLEncoding.EncodeToString(b), nil
}

func (a *Auth) session(ctx context.Context, raw string) (domain.Claims, domain.Session, error) {
	claims, err := a.security.Parse(raw)
	if err != nil {
		return claims, domain.Session{}, apperrors.ErrUnauthorized
	}
	session, err := a.repo.GetSession(ctx, claims.UserID, claims.SessionID)
	if errors.Is(err, apperrors.ErrNotFound) {
		return claims, session, apperrors.ErrUnauthorized
	}
	if err != nil {
		return claims, session, err
	}
	if session.UserID != claims.UserID || !session.ExpiresAt.After(time.Now()) {
		return claims, session, apperrors.ErrUnauthorized
	}
	return claims, session, nil
}

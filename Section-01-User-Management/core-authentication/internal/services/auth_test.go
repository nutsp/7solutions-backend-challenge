package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	domain "github.com/nutsp/7solutions-backend-challenge/core-authentication/internal/domain"
	"github.com/nutsp/7solutions-backend-challenge/core-authentication/internal/mocks"
	apperrors "github.com/nutsp/7solutions-backend-challenge/core-authentication/pkg/errors"
)

const uid = "0123456789abcdef0123456789abcdef"

const sid = "abcdef0123456789abcdef0123456789"

func TestLogin(t *testing.T) {
	tests := []struct {
		name         string
		email        string
		password     string
		passwordHash string
		credential   domain.Credential
		lookupErr    error
		compareErr   error
		issueErr     error
		sessionErr   error
		wantErr      error
	}{
		{
			name:         "persists session before returning tokens",
			email:        " ALICE@example.com ",
			password:     "correct-horse-battery",
			passwordHash: "stored-hash",
			credential: domain.Credential{
				UserID:       uid,
				PasswordHash: "stored-hash",
			},
		},
		{
			name:         "wrong password",
			email:        "alice@example.com",
			password:     "wrong",
			passwordHash: "stored-hash",
			credential: domain.Credential{
				UserID:       uid,
				PasswordHash: "stored-hash",
			},
			compareErr: apperrors.ErrUnauthorized,
			wantErr:    apperrors.ErrUnauthorized,
		},
		{
			name:         "unknown account uses dummy comparison",
			email:        "alice@example.com",
			password:     "wrong",
			passwordHash: "dummy-hash",
			lookupErr:    apperrors.ErrNotFound,
			wantErr:      apperrors.ErrUnauthorized,
		},
		{
			name:         "session repository unavailable",
			email:        "alice@example.com",
			password:     "correct-horse-battery",
			passwordHash: "stored-hash",
			credential: domain.Credential{
				UserID:       uid,
				PasswordHash: "stored-hash",
			},
			sessionErr: apperrors.ErrUnavailable,
			wantErr:    apperrors.ErrUnavailable,
		},
		{
			name:         "session limit reached",
			email:        "alice@example.com",
			password:     "correct-horse-battery",
			passwordHash: "stored-hash",
			credential: domain.Credential{
				UserID:       uid,
				PasswordHash: "stored-hash",
			},
			sessionErr: apperrors.ErrSessionLimit,
			wantErr:    apperrors.ErrSessionLimit,
		},
		{
			name:         "signing failure skips session write",
			email:        "alice@example.com",
			password:     "correct-horse-battery",
			passwordHash: "stored-hash",
			credential: domain.Credential{
				UserID:       uid,
				PasswordHash: "stored-hash",
			},
			issueErr: apperrors.ErrUnavailable,
			wantErr:  apperrors.ErrUnavailable,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, repo, users, security := newAuthService(t)
			ctx := context.Background()
			expires := time.Now().UTC().Add(15 * time.Minute)
			found := users.EXPECT().Credential(ctx, "alice@example.com").Return(tt.credential, tt.lookupErr).Once()
			compared := security.EXPECT().Compare(tt.passwordHash, tt.password).Return(tt.compareErr).Once()
			mock.InOrder(found, compared)
			var claims domain.Claims
			var stored domain.Session
			if tt.lookupErr == nil && tt.compareErr == nil {
				issued := security.EXPECT().Issue(mock.Anything).Run(func(c domain.Claims) {

					claims = c
				}).Return("access-token", expires, tt.issueErr).Once()
				mock.InOrder(compared, issued)
				if tt.issueErr == nil {
					saved := repo.EXPECT().CreateSession(ctx, mock.Anything).Run(func(_ context.Context, session domain.Session) {

						stored = session
					}).Return(tt.sessionErr).Once()
					mock.InOrder(issued, saved)
				}
			}
			tokens, err := service.Login(ctx, tt.email, tt.password)
			require.ErrorIs(t, err, tt.wantErr)
			if tt.wantErr != nil {
				require.Empty(t, tokens.AccessToken)
				require.Empty(t, tokens.RefreshToken)
				return
			}
			require.Equal(t, uid, claims.UserID)
			require.Len(t, claims.SessionID, 32)
			require.Equal(t, claims.SessionID, stored.ID)
			require.Equal(t, uid, stored.UserID)
			require.Equal(t, tokenDigest(tokens.RefreshToken), stored.RefreshHash)
			require.NotEqual(t, tokens.RefreshToken, stored.RefreshHash)
			require.WithinDuration(t, time.Now().Add(30*24*time.Hour), stored.ExpiresAt, 2*time.Second)
			require.Equal(t, "access-token", tokens.AccessToken)
			require.Equal(t, expires, tokens.ExpiresAt)
		})
	}
}

func TestAuthenticate(t *testing.T) {
	tests := []struct {
		name       string
		sessionTTL time.Duration
		parseErr   error
		sessionErr error
		userErr    error
		wantErr    error
	}{
		{
			name:       "active",
			sessionTTL: time.Hour,
		},
		{
			name:     "invalid token",
			parseErr: apperrors.ErrUnauthorized,
			wantErr:  apperrors.ErrUnauthorized,
		},
		{
			name:       "revoked",
			sessionTTL: time.Hour,
			sessionErr: apperrors.ErrNotFound,
			wantErr:    apperrors.ErrUnauthorized,
		},
		{
			name:       "expired",
			sessionTTL: -time.Minute,
			wantErr:    apperrors.ErrUnauthorized,
		},
		{
			name:       "deleted account",
			sessionTTL: time.Hour,
			userErr:    apperrors.ErrNotFound,
			wantErr:    apperrors.ErrUnauthorized,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, repo, users, security := newAuthService(t)
			ctx := context.Background()
			security.EXPECT().Parse("access-token").Return(domain.Claims{
				UserID:    uid,
				SessionID: sid,
			}, tt.parseErr).Once()
			if tt.parseErr == nil {
				session := domain.Session{
					ID:        sid,
					UserID:    uid,
					ExpiresAt: time.Now().Add(tt.sessionTTL),
				}
				repo.EXPECT().GetSession(ctx, uid, sid).Return(session, tt.sessionErr).Once()
				if tt.sessionErr == nil && tt.sessionTTL > 0 {
					users.EXPECT().Get(ctx, uid).Return(domain.Profile{
						ID: uid,
					}, tt.userErr).Once()
				}
			}
			actor, err := service.Authenticate(ctx, "access-token")
			require.ErrorIs(t, err, tt.wantErr)
			if err == nil {
				require.Equal(t, uid, actor.UserID)
			}
		})
	}
}

func TestConcurrentRefreshReturnsOnlyTheAtomicRotationWinner(t *testing.T) {
	tests := []struct {
		name     string
		requests int
	}{{"two callers", 2}, {"four callers", 4}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, repo, users, security := newAuthService(t)
			refresh := uid + "." + sid + "." + strings.Repeat("a", 43)
			oldHash := tokenDigest(refresh)
			session := domain.Session{
				ID:          sid,
				UserID:      uid,
				RefreshHash: oldHash,
				ExpiresAt:   time.Now().Add(time.Hour),
			}
			repo.EXPECT().GetSession(mock.Anything, uid, sid).Return(session, nil).Times(tt.requests)
			users.EXPECT().Get(mock.Anything, uid).Return(domain.Profile{
				ID: uid,
			}, nil).Times(tt.requests)
			security.EXPECT().Issue(domain.Claims{
				UserID:    uid,
				SessionID: sid,
			}).Return("access", time.Now().Add(time.Minute), nil).Times(tt.requests)
			var rotated atomic.Bool
			repo.EXPECT().Rotate(mock.Anything, session, oldHash, mock.Anything).RunAndReturn(func(_ context.Context, _ domain.Session, old, new string) error {
				if old == new || len(new) != 64 {
					return apperrors.ErrInvalid
				}
				if rotated.CompareAndSwap(false, true) {
					return nil
				}
				return apperrors.ErrUnauthorized
			}).Times(tt.requests)
			var wg sync.WaitGroup
			out := make(chan error, tt.requests)
			for range tt.requests {
				wg.Add(1)
				go func() {

					defer wg.Done()
					_, err := service.Refresh(context.Background(), refresh)
					out <- err
				}()
			}
			wg.Wait()
			close(out)
			successes, rejected := 0, 0
			for err := range out {
				if err == nil {
					successes++
				} else {
					require.ErrorIs(t, err, apperrors.ErrUnauthorized)
					rejected++
				}
			}
			require.Equal(t, 1, successes)
			require.Equal(t, tt.requests-1, rejected)

		})
	}
}

func TestLogout(t *testing.T) {
	tests := []struct {
		name      string
		parseErr  error
		deleteErr error
		wantErr   error
	}{
		{
			name: "deletes active session",
		},
		{
			name:     "invalid token",
			parseErr: apperrors.ErrUnauthorized,
			wantErr:  apperrors.ErrUnauthorized,
		},
		{
			name:      "repository failure",
			deleteErr: apperrors.ErrUnavailable,
			wantErr:   apperrors.ErrUnavailable,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, repo, _, security := newAuthService(t)
			security.EXPECT().Parse("access-token").Return(domain.Claims{
				UserID:    uid,
				SessionID: sid,
			}, tt.parseErr).Once()
			if tt.parseErr == nil {
				repo.EXPECT().DeleteSession(mock.Anything, uid, sid).Return(tt.deleteErr).Once()
			}
			require.ErrorIs(t, service.Logout(context.Background(), "access-token"), tt.wantErr)
		})
	}
}

func TestAuthorizeDelete(t *testing.T) {
	tests := []struct {
		name     string
		targetID string
		userErr  error
		wantErr  error
	}{
		{
			name:     "owner with active session",
			targetID: uid,
		},
		{
			name:     "cross owner rejection",
			targetID: "another-user",
			wantErr:  apperrors.ErrForbidden,
		},
		{
			name:     "recovery after profile deletion",
			targetID: uid,
			userErr:  apperrors.ErrNotFound,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, repo, users, security := newAuthService(t)
			ctx := context.Background()
			calls := 1
			if tt.targetID == uid && tt.userErr == nil {
				calls = 2
			}
			security.EXPECT().Parse("access-token").Return(domain.Claims{
				UserID:    uid,
				SessionID: sid,
			}, nil).Times(calls)
			if tt.targetID == uid {
				users.EXPECT().Get(ctx, uid).Return(domain.Profile{
					ID: uid,
				}, tt.userErr).Times(calls)
				if tt.userErr == nil {
					repo.EXPECT().GetSession(ctx, uid, sid).Return(domain.Session{
						ID:        sid,
						UserID:    uid,
						ExpiresAt: time.Now().Add(time.Hour),
					}, nil).Once()
				}
			}
			actor, err := service.AuthorizeDelete(ctx, "access-token", tt.targetID)
			require.ErrorIs(t, err, tt.wantErr)
			if err == nil {
				require.Equal(t, uid, actor.UserID)
			}
		})
	}
}

func TestRevokeUser(t *testing.T) {
	tests := []struct {
		name    string
		wantErr error
	}{
		{
			name: "revokes all user sessions",
		}, {
			name:    "repository unavailable",
			wantErr: apperrors.ErrUnavailable,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, repo, _, _ := newAuthService(t)
			repo.EXPECT().DeleteSessions(mock.Anything, uid).Return(tt.wantErr).Once()
			require.ErrorIs(t, service.RevokeUser(context.Background(), uid), tt.wantErr)
		})
	}
}

func newAuthService(t *testing.T) (*Auth, *mocks.MockSessionRepository, *mocks.MockCoreUserAdapter, *mocks.MockAuthenticationSecurityAdapter) {
	t.Helper()
	repo := mocks.NewMockSessionRepository(t)
	users := mocks.NewMockCoreUserAdapter(t)
	security := mocks.NewMockAuthenticationSecurityAdapter(t)
	security.EXPECT().Hash("dummy-password-for-timing-only").Return("dummy-hash", nil).Once()
	service, err := New(repo, users, security)
	require.NoError(t, err)
	return service, repo, users, security
}

func tokenDigest(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

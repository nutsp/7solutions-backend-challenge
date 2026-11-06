package services

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	domain "github.com/nutsp/7solutions-backend-challenge/core-user-manage/internal/domain"
	"github.com/nutsp/7solutions-backend-challenge/core-user-manage/internal/mocks"
	apperrors "github.com/nutsp/7solutions-backend-challenge/core-user-manage/pkg/errors"
)

const testUserID = "0123456789abcdef0123456789abcdef"

type userFixture struct {
	service *Users
	repo    *mocks.MockUserRepository
	hasher  *mocks.MockPasswordHasher
	audits  *mocks.MockUserAuditRepository
	tx      *mocks.MockTransactionManager
}

func TestCreate(t *testing.T) {
	tests := []struct {
		name     string
		invalid  bool
		hashErr  error
		repoErr  error
		auditErr error
		wantErr  error
	}{
		{
			name: "normalize hash and audit",
		}, {
			name:    "invalid input",
			invalid: true,
			wantErr: apperrors.ErrInvalid,
		},
		{
			name:    "hash failure",
			hashErr: apperrors.ErrUnavailable,
			wantErr: apperrors.ErrUnavailable,
		},
		{
			name:    "duplicate email",
			repoErr: apperrors.ErrConflict,
			wantErr: apperrors.ErrConflict,
		},
		{
			name:     "audit failure aborts operation",
			auditErr: apperrors.ErrUnavailable,
			wantErr:  apperrors.ErrUnavailable,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newUserFixture(t)
			input := domain.Create{
				Name:     " Alice ",
				Email:    " ALICE@example.com ",
				Password: "correct-horse-battery",
			}
			if tt.invalid {
				input.Email = "bad"
			}
			var saved domain.User
			if !tt.invalid {
				hashed := f.hasher.EXPECT().Hash(input.Password).Return("bcrypt-hash", tt.hashErr).Once()
				if tt.hashErr == nil {
					f.transaction()
					written := f.repo.EXPECT().Create(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, u domain.User) (domain.User, error) {

						saved = u
						return u, tt.repoErr
					}).Once()
					mock.InOrder(hashed, written)
					if tt.repoErr == nil {
						audited := f.audits.EXPECT().Append(mock.Anything, mock.Anything).Run(func(_ context.Context, event domain.UserAudit) {
							require.Equal(t, "CREATE", event.Action)
							require.Equal(t, saved.ID, event.UserID)
							require.Equal(t, saved.UpdatedAt, event.OccurredAt)
							require.Equal(t, "anonymous", event.Actor.ActorType)
							require.Equal(t, "request-1", event.Actor.RequestID)
							require.Nil(t, event.Changes["name"].Before)
							require.Equal(t, "Alice", *event.Changes["name"].After)
							require.Len(t, event.Changes, 2)
						}).Return(tt.auditErr).Once()
						mock.InOrder(written, audited)
					}
				}
			}
			ctx := domain.WithAuditContext(context.Background(), domain.AuditContext{
				CallerService: "orch",
				RequestID:     "request-1",
			})
			got, err := f.service.Create(ctx, input)
			require.ErrorIs(t, err, tt.wantErr)
			if err != nil {
				require.Empty(t, got.ID)
				return
			}
			require.Equal(t, saved, got)
			require.Equal(t, "Alice", got.Name)
			require.Equal(t, "alice@example.com", got.Email)
			require.Equal(t, "bcrypt-hash", got.PasswordHash)
			require.Len(t, got.ID, 32)
			require.False(t, got.CreatedAt.IsZero())
			require.Equal(t, got.CreatedAt, got.UpdatedAt)
		})
	}
}

func TestUpdate(t *testing.T) {
	tests := []struct {
		name     string
		other    bool
		empty    bool
		noOp     bool
		readErr  error
		writeErr error
		auditErr error
		wantErr  error
	}{
		{
			name: "normalized change and before after audit",
		}, {
			name: "no-op keeps timestamp",
			noOp: true,
		},
		{
			name:    "other owner",
			other:   true,
			wantErr: apperrors.ErrForbidden,
		}, {
			name:    "empty patch",
			empty:   true,
			wantErr: apperrors.ErrInvalid,
		},
		{
			name:    "missing user",
			readErr: apperrors.ErrNotFound,
			wantErr: apperrors.ErrNotFound,
		},
		{
			name:     "email conflict",
			writeErr: apperrors.ErrConflict,
			wantErr:  apperrors.ErrConflict,
		},
		{
			name:     "audit failure",
			auditErr: apperrors.ErrUnavailable,
			wantErr:  apperrors.ErrUnavailable,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newUserFixture(t)
			name, email := " Updated ", " NEW@example.com "
			before := domain.User{
				ID:        testUserID,
				Name:      "Original",
				Email:     "original@example.com",
				CreatedAt: time.Now().UTC().Add(-time.Hour).Truncate(time.Millisecond),
				UpdatedAt: time.Now().UTC().Add(-time.Minute).Truncate(time.Millisecond),
			}
			owner := domain.Principal{
				UserID: testUserID,
			}
			if tt.other {
				owner.UserID = "other"
			}
			patch := domain.Patch{
				Name:  &name,
				Email: &email,
			}
			if tt.empty {
				patch = domain.Patch{}
			}
			if tt.noOp {
				name = " Original "
				email = " ORIGINAL@example.com "
			}
			var expected domain.User
			if !tt.other && !tt.empty {
				f.transaction()
				read := f.repo.EXPECT().Get(mock.Anything, testUserID).Return(before, tt.readErr).Once()
				if tt.readErr == nil {
					if tt.noOp {
						expected = before
					} else {
						written := f.repo.EXPECT().Update(mock.Anything, testUserID, mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, _ string, p domain.Patch, at time.Time) (domain.User, error) {
							require.Equal(t, "Updated", *p.Name)
							require.Equal(t, "new@example.com", *p.Email)
							require.True(t, at.After(before.UpdatedAt))
							expected = before
							expected.Name = *p.Name
							expected.Email = *p.Email
							expected.UpdatedAt = at
							return expected, tt.writeErr
						}).Once()
						mock.InOrder(read, written)
						if tt.writeErr == nil {
							audited := f.audits.EXPECT().Append(mock.Anything, mock.Anything).Run(func(_ context.Context, event domain.UserAudit) {
								require.Equal(t, "UPDATE", event.Action)
								require.Equal(t, owner.UserID, event.Actor.ActorID)
								require.Equal(t, expected.UpdatedAt, event.OccurredAt)
								require.Equal(t, "Original", *event.Changes["name"].Before)
								require.Equal(t, "Updated", *event.Changes["name"].After)
							}).Return(tt.auditErr).Once()
							mock.InOrder(written, audited)
						}
					}
				}
			}
			got, err := f.service.Update(context.Background(), owner, testUserID, patch)
			require.ErrorIs(t, err, tt.wantErr)
			if err == nil {
				require.Equal(t, expected, got)
			} else {
				require.Empty(t, got.ID)
			}
		})
	}
}

func TestDeleteAuthorized(t *testing.T) {
	tests := []struct {
		name     string
		other    bool
		readErr  error
		writeErr error
		auditErr error
		wantErr  error
	}{
		{
			name: "owner deletion",
		}, {
			name:    "already deleted",
			readErr: apperrors.ErrNotFound,
		},
		{
			name:    "other owner",
			other:   true,
			wantErr: apperrors.ErrForbidden,
		},
		{
			name:     "delete failure",
			writeErr: apperrors.ErrUnavailable,
			wantErr:  apperrors.ErrUnavailable,
		},
		{
			name:     "audit failure",
			auditErr: apperrors.ErrUnavailable,
			wantErr:  apperrors.ErrUnavailable,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newUserFixture(t)
			owner := domain.Principal{
				UserID: testUserID,
			}
			if tt.other {
				owner.UserID = "other"
			} else {
				f.transaction()
				read := f.repo.EXPECT().Get(mock.Anything, testUserID).Return(domain.User{
					ID:    testUserID,
					Name:  "Alice",
					Email: "alice@example.com",
				}, tt.readErr).Once()
				if tt.readErr == nil {
					deleted := f.repo.EXPECT().Delete(mock.Anything, testUserID).Return(tt.writeErr).Once()
					mock.InOrder(read, deleted)
					if tt.writeErr == nil {
						audited := f.audits.EXPECT().Append(mock.Anything, mock.Anything).Run(func(_ context.Context, event domain.UserAudit) {
							require.Equal(t, "DELETE", event.Action)
							require.Equal(t, "Alice", *event.Changes["name"].Before)
							require.Nil(t, event.Changes["name"].After)
						}).Return(tt.auditErr).Once()
						mock.InOrder(deleted, audited)
					}
				}
			}
			require.ErrorIs(t, f.service.DeleteAuthorized(context.Background(), owner, testUserID), tt.wantErr)
		})
	}
}

func TestGet(t *testing.T) {
	tests := []struct {
		name    string
		actor   domain.Principal
		userID  string
		repoErr error
		wantErr error
	}{
		{
			name: "authenticated reader",
			actor: domain.Principal{
				UserID: "another-user",
			},
			userID: testUserID,
		},
		{
			name:    "anonymous reader",
			userID:  testUserID,
			wantErr: apperrors.ErrForbidden,
		},
		{
			name: "invalid identifier",
			actor: domain.Principal{
				UserID: testUserID,
			},
			userID:  "bad",
			wantErr: apperrors.ErrInvalid,
		},
		{
			name: "missing user",
			actor: domain.Principal{
				UserID: testUserID,
			},
			userID:  testUserID,
			repoErr: apperrors.ErrNotFound,
			wantErr: apperrors.ErrNotFound,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newUserFixture(t)
			service, repo := f.service, f.repo
			user := domain.User{
				ID:   testUserID,
				Name: "Alice",
			}
			if tt.wantErr == nil || tt.repoErr != nil {
				repo.EXPECT().Get(mock.Anything, tt.userID).Return(user, tt.repoErr).Once()
			}
			got, err := service.Get(context.Background(), tt.actor, tt.userID)
			require.ErrorIs(t, err, tt.wantErr)
			if err == nil {
				require.Equal(t, user, got)
			}
		})
	}
}

func TestList(t *testing.T) {
	tests := []struct {
		name      string
		actor     domain.Principal
		limit     int
		wantLimit int
		afterID   string
		wantErr   error
	}{
		{
			name: "default pagination",
			actor: domain.Principal{
				UserID: testUserID,
			},
			wantLimit: 20,
		},
		{
			name: "explicit pagination",
			actor: domain.Principal{
				UserID: testUserID,
			},
			limit:     10,
			wantLimit: 10,
			afterID:   testUserID,
		},
		{
			name:    "anonymous",
			limit:   20,
			wantErr: apperrors.ErrForbidden,
		},
		{
			name: "oversized page",
			actor: domain.Principal{
				UserID: testUserID,
			},
			limit:   101,
			wantErr: apperrors.ErrInvalid,
		},
		{
			name: "invalid cursor",
			actor: domain.Principal{
				UserID: testUserID,
			},
			limit:   20,
			afterID: "bad-cursor",
			wantErr: apperrors.ErrInvalid,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newUserFixture(t)
			service, repo := f.service, f.repo
			page := domain.Page{
				Users: []domain.User{{
					ID:   testUserID,
					Name: "Alice",
				}},
			}
			if tt.wantErr == nil {
				repo.EXPECT().List(mock.Anything, tt.wantLimit, tt.afterID).Return(page, nil).Once()
			}
			got, err := service.List(context.Background(), tt.actor, tt.limit, tt.afterID)
			require.ErrorIs(t, err, tt.wantErr)
			if err == nil {
				require.Equal(t, page, got)
			}
		})
	}
}

func newUserFixture(t *testing.T) *userFixture {
	t.Helper()
	f := &userFixture{
		repo:   mocks.NewMockUserRepository(t),
		hasher: mocks.NewMockPasswordHasher(t),
		audits: mocks.NewMockUserAuditRepository(t),
		tx:     mocks.NewMockTransactionManager(t),
	}
	f.service = New(f.repo, f.hasher, f.audits, f.tx)
	return f
}

func (f *userFixture) transaction() {
	f.tx.EXPECT().WithinTransaction(mock.Anything, mock.Anything).RunAndReturn(func(ctx context.Context, run func(context.Context) error) error {

		return run(ctx)
	}).Once()
}

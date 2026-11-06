package services

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	domain "github.com/nutsp/7solutions-backend-challenge/orch-user-access/internal/domain"
	"github.com/nutsp/7solutions-backend-challenge/orch-user-access/internal/mocks"
	apperrors "github.com/nutsp/7solutions-backend-challenge/orch-user-access/pkg/errors"
)

func TestRegister(t *testing.T) {
	tests := []struct {
		name      string
		createErr error
		loginErr  error
		wantErr   error
	}{
		{
			name: "create before login using canonical email",
		},
		{
			name:      "create failure skips login",
			createErr: apperrors.ErrConflict,
			wantErr:   apperrors.ErrConflict,
		},
		{
			name:     "token failure keeps created account",
			loginErr: apperrors.ErrUnavailable,
			wantErr:  apperrors.ErrUnavailable,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			users := mocks.NewMockCoreUserAdapter(t)
			auth := mocks.NewMockCoreAuthenticationAdapter(t)
			ctx := context.Background()
			input := domain.Register{
				Name:     "Alice",
				Email:    " ALICE@example.com ",
				Password: "correct-horse-battery",
			}
			user := domain.User{
				ID:    "id",
				Email: "alice@example.com",
			}
			tokens := domain.Tokens{
				AccessToken: "token",
				UserID:      "id",
			}
			created := users.EXPECT().Create(ctx, input).Return(user, tt.createErr).Once()
			if tt.createErr == nil {
				issued := auth.EXPECT().Login(ctx, domain.Login{
					Email:    user.Email,
					Password: input.Password,
				}).Return(tokens, tt.loginErr).Once()
				mock.InOrder(created, issued)
			}
			result, err := New(users, auth).Register(ctx, input)
			require.ErrorIs(t, err, tt.wantErr)
			if err == nil {
				require.Equal(t, domain.Registration{
					User:   user,
					Tokens: tokens,
				}, result)
			}
			// No Delete expectation: unexpected compensation fails the test.
		})
	}
}

func TestDelete(t *testing.T) {
	tests := []struct {
		name         string
		authorizeErr error
		deleteErr    error
		revokeErr    error
		wantErr      error
	}{
		{
			name: "authorize then delete then revoke",
		},
		{
			name:         "authorization failure",
			authorizeErr: apperrors.ErrForbidden,
			wantErr:      apperrors.ErrForbidden,
		},
		{
			name:      "delete failure",
			deleteErr: apperrors.ErrUnavailable,
			wantErr:   apperrors.ErrUnavailable,
		},
		{
			name:      "revoke failure",
			revokeErr: apperrors.ErrUnavailable,
			wantErr:   apperrors.ErrUnavailable,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			users := mocks.NewMockCoreUserAdapter(t)
			auth := mocks.NewMockCoreAuthenticationAdapter(t)
			ctx := context.Background()
			actor := domain.Principal{
				UserID: "id",
			}
			authorized := auth.EXPECT().AuthorizeDelete(ctx, "token", "id").Return(actor, tt.authorizeErr).Once()
			if tt.authorizeErr == nil {
				deleted := users.EXPECT().Delete(ctx, actor, "id").Return(tt.deleteErr).Once()
				mock.InOrder(authorized, deleted)
				if tt.deleteErr == nil {
					revoked := auth.EXPECT().RevokeUser(ctx, actor, "id").Return(tt.revokeErr).Once()
					mock.InOrder(deleted, revoked)
				}
			}
			require.ErrorIs(t, New(users, auth).Delete(ctx, "token", "id"), tt.wantErr)
		})
	}
}

package grpchandler

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	domain "github.com/nutsp/7solutions-backend-challenge/core-authentication/internal/domain"
	"github.com/nutsp/7solutions-backend-challenge/core-authentication/internal/mocks"
	apperrors "github.com/nutsp/7solutions-backend-challenge/core-authentication/pkg/errors"
	pb "github.com/nutsp/7solutions-backend-challenge/core-authentication/pkg/proto/core-authentication"
)

func TestLogin(t *testing.T) {
	tests := []struct {
		name       string
		serviceErr error
		wantCode   codes.Code
	}{
		{
			name:     "token pair and expiry",
			wantCode: codes.OK,
		},
		{
			name:       "invalid credentials",
			serviceErr: apperrors.ErrUnauthorized,
			wantCode:   codes.Unauthenticated,
		},
		{
			name:       "session unavailable",
			serviceErr: apperrors.ErrUnavailable,
			wantCode:   codes.Unavailable,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := mocks.NewMockAuthenticationService(t)
			server := New(service, slog.New(slog.NewTextHandler(io.Discard, nil)))
			expires := time.Now().UTC().Truncate(time.Second).Add(time.Minute)
			tokens := domain.Tokens{
				UserID:       "user",
				AccessToken:  "access",
				RefreshToken: "refresh",
				ExpiresAt:    expires,
			}
			service.EXPECT().Login(mock.Anything, "alice@example.com", "correct-horse-battery").Return(tokens, tt.serviceErr).Once()
			response, err := server.Login(context.Background(), &pb.LoginRequest{
				Email:    "alice@example.com",
				Password: "correct-horse-battery",
			})
			require.Equal(t, tt.wantCode, status.Code(err))
			if err != nil {
				return
			}
			require.Equal(t, "access", response.AccessToken)
			require.Equal(t, "refresh", response.RefreshToken)
			require.Equal(t, "Bearer", response.TokenType)
			require.Equal(t, expires.Format(time.RFC3339), response.ExpiresAt)
		})
	}
}

func TestAuthenticate(t *testing.T) {
	tests := []struct {
		name       string
		serviceErr error
		wantCode   codes.Code
	}{
		{
			name:     "valid token",
			wantCode: codes.OK,
		},
		{
			name:       "revoked session",
			serviceErr: apperrors.ErrUnauthorized,
			wantCode:   codes.Unauthenticated,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := mocks.NewMockAuthenticationService(t)
			server := New(service, slog.New(slog.NewTextHandler(io.Discard, nil)))
			service.EXPECT().Authenticate(mock.Anything, "access").Return(domain.Principal{
				UserID: "user",
			}, tt.serviceErr).Once()
			actor, err := server.Authenticate(context.Background(), &pb.AccessToken{
				AccessToken: "access",
			})
			require.Equal(t, tt.wantCode, status.Code(err))
			if err == nil {
				require.Equal(t, "user", actor.UserId)
			}
		})
	}
}

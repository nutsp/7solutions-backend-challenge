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
	"google.golang.org/protobuf/encoding/protojson"

	domain "github.com/nutsp/7solutions-backend-challenge/core-user-manage/internal/domain"
	"github.com/nutsp/7solutions-backend-challenge/core-user-manage/internal/mocks"
	apperrors "github.com/nutsp/7solutions-backend-challenge/core-user-manage/pkg/errors"
	"github.com/nutsp/7solutions-backend-challenge/core-user-manage/pkg/grpcclient"
	pb "github.com/nutsp/7solutions-backend-challenge/core-user-manage/pkg/proto/core-user-manage"
)

func TestCreateUser(t *testing.T) {
	tests := []struct {
		name       string
		serviceErr error
		wantCode   codes.Code
	}{
		{
			name:     "public DTO omits password hash",
			wantCode: codes.OK,
		},
		{
			name:       "duplicate email",
			serviceErr: apperrors.ErrConflict,
			wantCode:   codes.AlreadyExists,
		},
		{
			name:       "invalid input",
			serviceErr: apperrors.ErrInvalid,
			wantCode:   codes.InvalidArgument,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := mocks.NewMockUserService(t)
			server := New(service, slog.New(slog.NewTextHandler(io.Discard, nil)))
			input := domain.Create{
				Name:     "Alice",
				Email:    "alice@example.com",
				Password: "correct-horse-battery",
			}
			user := domain.User{
				ID:           "0123456789abcdef0123456789abcdef",
				Name:         input.Name,
				Email:        input.Email,
				PasswordHash: "private-hash",
				CreatedAt:    time.Now().UTC(),
				UpdatedAt:    time.Now().UTC(),
			}
			service.EXPECT().Create(mock.Anything, input).Return(user, tt.serviceErr).Once()
			response, err := server.CreateUser(context.Background(), &pb.CreateUserRequest{
				Name:     input.Name,
				Email:    input.Email,
				Password: input.Password,
			})
			require.Equal(t, tt.wantCode, status.Code(err))
			if err != nil {
				return
			}
			require.Equal(t, user.ID, response.Id)
			require.Equal(t, user.CreatedAt.Format(time.RFC3339Nano), response.CreatedAt)
			require.Equal(t, user.UpdatedAt.Format(time.RFC3339Nano), response.UpdatedAt)
			raw, err := protojson.Marshal(response)
			require.NoError(t, err)
			require.NotContains(t, string(raw), "private-hash")
			require.NotContains(t, string(raw), "password")
		})
	}
}

func TestGetUser(t *testing.T) {
	tests := []struct {
		name       string
		serviceErr error
		wantCode   codes.Code
	}{
		{
			name:     "forwards trusted principal",
			wantCode: codes.OK,
		},
		{
			name:       "missing user",
			serviceErr: apperrors.ErrNotFound,
			wantCode:   codes.NotFound,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := mocks.NewMockUserService(t)
			server := New(service, slog.New(slog.NewTextHandler(io.Discard, nil)))
			user := domain.User{
				ID: "0123456789abcdef0123456789abcdef",
			}
			actor := domain.Principal{
				UserID: user.ID,
			}
			service.EXPECT().Get(mock.Anything, actor, user.ID).Return(user, tt.serviceErr).Once()
			ctx := grpcclient.WithPrincipal(context.Background(), grpcclient.Principal{
				UserID: user.ID,
				Caller: "orch",
			})
			response, err := server.GetUser(ctx, &pb.UserID{
				Id: user.ID,
			})
			require.Equal(t, tt.wantCode, status.Code(err))
			if err == nil {
				require.Equal(t, user.ID, response.Id)
			}
		})
	}
}

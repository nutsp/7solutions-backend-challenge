package grpcerrors

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	apperrors "github.com/nutsp/7solutions-backend-challenge/core-user-manage/pkg/errors"
)

func TestGRPCMappings(t *testing.T) {
	if FromDownstream("core", nil) != nil || ToStatus(nil, nil) != nil {
		t.Fatal("nil error converted")
	}
	for _, tc := range []struct {
		input  codes.Code
		output codes.Code
		kind   error
	}{
		{codes.InvalidArgument, codes.InvalidArgument, apperrors.ErrInvalid},
		{codes.NotFound, codes.NotFound, apperrors.ErrNotFound},
		{codes.AlreadyExists, codes.AlreadyExists, apperrors.ErrConflict},
		{codes.Aborted, codes.AlreadyExists, apperrors.ErrConflict},
		{codes.Unauthenticated, codes.Unauthenticated, apperrors.ErrUnauthorized},
		{codes.PermissionDenied, codes.PermissionDenied, apperrors.ErrForbidden},
		{codes.ResourceExhausted, codes.ResourceExhausted, apperrors.ErrSessionLimit},
		{codes.DeadlineExceeded, codes.DeadlineExceeded, apperrors.ErrTimeout},
		{codes.Canceled, codes.Canceled, context.Canceled},
		{codes.Unavailable, codes.Unavailable, apperrors.ErrUnavailable},
		{codes.Internal, codes.Unavailable, apperrors.ErrUnavailable},
	} {
		t.Run(tc.input.String(), func(t *testing.T) {
			semantic := FromDownstream("private operation", status.Error(tc.input, "private detail"))
			if !errors.Is(semantic, tc.kind) {
				t.Fatalf("%s: wrong semantic error %v", tc.input, semantic)
			}
			response := ToStatus(apperrors.Wrap("private operation", semantic), nil)
			if status.Code(response) != tc.output || strings.Contains(response.Error(), "private") {
				t.Fatalf("%s: unsafe or wrong response %v", tc.input, response)
			}

		})
	}
	for _, tc := range []struct {
		input error
		want  error
	}{
		{context.Canceled, context.Canceled},
		{context.DeadlineExceeded, apperrors.ErrTimeout},
		{errors.New("connection failed"), apperrors.ErrUnavailable},
	} {
		t.Run(tc.input.Error(), func(t *testing.T) {
			if !errors.Is(FromDownstream("core", tc.input), tc.want) {
				t.Fatalf("wrong conversion of %v", tc.input)
			}

		})
	}
}

func TestUnexpectedFailureIsLoggedButNotExposed(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{"raw failure", errors.New("private diagnostic")},
		{"wrapped failure", apperrors.Wrap("downstream", errors.New("private diagnostic"))},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var logs bytes.Buffer
			response := ToStatus(tt.err, slog.New(slog.NewJSONHandler(&logs, nil)))
			if status.Code(response) != codes.Unavailable || strings.Contains(response.Error(), "private") {
				t.Fatalf("unsafe response %v", response)
			}
			if !strings.Contains(logs.String(), "private diagnostic") {
				t.Fatal("diagnostic cause was not logged")
			}

		})
	}
}

// Package grpcerrors translates between gRPC status and semantic application errors.
package grpcerrors

import (
	"context"
	"errors"
	"log/slog"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	apperrors "github.com/nutsp/7solutions-backend-challenge/core-user-manage/pkg/errors"
)

func FromDownstream(operation string, err error) error {
	if err == nil {
		return nil
	}
	var kind error
	switch {
	case errors.Is(err, context.Canceled):
		kind = context.Canceled
	case errors.Is(err, context.DeadlineExceeded):
		kind = apperrors.ErrTimeout
	default:
		switch status.Code(err) {
		case codes.InvalidArgument:
			kind = apperrors.ErrInvalid
		case codes.NotFound:
			kind = apperrors.ErrNotFound
		case codes.AlreadyExists, codes.Aborted:
			kind = apperrors.ErrConflict
		case codes.Unauthenticated:
			kind = apperrors.ErrUnauthorized
		case codes.PermissionDenied:
			kind = apperrors.ErrForbidden
		case codes.ResourceExhausted:
			kind = apperrors.ErrSessionLimit
		case codes.DeadlineExceeded:
			kind = apperrors.ErrTimeout
		case codes.Canceled:
			kind = context.Canceled
		default:
			kind = apperrors.ErrUnavailable
		}
	}
	return apperrors.Wrap(operation, kind)
}

// ToStatus never sends dependency details or operation context to the caller.
// A nil logger disables diagnostic logging, which is useful for pure mapping tests.
func ToStatus(err error, log *slog.Logger) error {
	if err == nil {
		return nil
	}
	code, message := codes.Unavailable, apperrors.ErrUnavailable.Error()
	switch {
	case errors.Is(err, apperrors.ErrInvalid):
		code, message = codes.InvalidArgument, apperrors.ErrInvalid.Error()
	case errors.Is(err, apperrors.ErrNotFound):
		code, message = codes.NotFound, apperrors.ErrNotFound.Error()
	case errors.Is(err, apperrors.ErrConflict):
		code, message = codes.AlreadyExists, apperrors.ErrConflict.Error()
	case errors.Is(err, apperrors.ErrUnauthorized):
		code, message = codes.Unauthenticated, apperrors.ErrUnauthorized.Error()
	case errors.Is(err, apperrors.ErrForbidden):
		code, message = codes.PermissionDenied, apperrors.ErrForbidden.Error()
	case errors.Is(err, apperrors.ErrSessionLimit):
		code, message = codes.ResourceExhausted, apperrors.ErrSessionLimit.Error()
	case errors.Is(err, context.Canceled):
		code, message = codes.Canceled, context.Canceled.Error()
	case errors.Is(err, apperrors.ErrTimeout), errors.Is(err, context.DeadlineExceeded):
		code, message = codes.DeadlineExceeded, apperrors.ErrTimeout.Error()
	}
	if code == codes.Unavailable && log != nil {
		log.Error("operation failed", "error", err)
	}
	return status.Error(code, message)
}

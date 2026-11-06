// Package httperrors renders semantic application errors as safe HTTP JSON responses.
package httperrors

import (
	"context"
	"errors"
	"net/http"

	apperrors "github.com/nutsp/7solutions-backend-challenge/orch-user-access/pkg/errors"
	"github.com/nutsp/7solutions-backend-challenge/orch-user-access/pkg/httpresponse"
)

func Write(w http.ResponseWriter, err error) {
	if err == nil {
		return
	}
	code, kind, message := http.StatusServiceUnavailable, "service_unavailable", "service temporarily unavailable"
	switch {
	case errors.Is(err, apperrors.ErrInvalid):
		code, kind, message = 400, "invalid_request", "invalid request"
	case errors.Is(err, apperrors.ErrUnauthorized):
		code, kind, message = 401, "unauthorized", "invalid credentials or token"
	case errors.Is(err, apperrors.ErrForbidden):
		code, kind, message = 403, "forbidden", "access denied"
	case errors.Is(err, apperrors.ErrNotFound):
		code, kind, message = 404, "not_found", "user not found"
	case errors.Is(err, apperrors.ErrConflict):
		code, kind, message = 409, "conflict", "email already registered"
	case errors.Is(err, apperrors.ErrSessionLimit):
		code, kind, message = 429, "session_limit", "active session limit reached"
	case errors.Is(err, apperrors.ErrTimeout), errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		code, kind, message = 504, "timeout", "request timed out"
	}
	if code == http.StatusUnauthorized {
		w.Header().Set("WWW-Authenticate", "Bearer")
	}
	httpresponse.Failure(w, code, kind, message)
}

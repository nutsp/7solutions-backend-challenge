// Package errors defines shared semantic failures. Adapters translate transport/driver errors into these values.
package errors

import (
	"context"
	stderrors "errors"
	"fmt"
)

var (
	ErrInvalid      = stderrors.New("invalid input")
	ErrNotFound     = stderrors.New("resource not found")
	ErrConflict     = stderrors.New("resource already exists")
	ErrUnauthorized = stderrors.New("invalid credentials or token")
	ErrForbidden    = stderrors.New("access denied")
	ErrSessionLimit = stderrors.New("active session limit reached")
	ErrUnavailable  = stderrors.New("dependency unavailable")
	ErrTimeout      = stderrors.New("operation timed out")
)

// Wrap adds operation context while preserving errors.Is/errors.As semantics.
func Wrap(operation string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", operation, err)
}

// FromDependency classifies failures from non-transport dependencies while retaining diagnostic causes.
func FromDependency(operation string, err error) error {
	if err == nil {
		return nil
	}
	if stderrors.Is(err, context.Canceled) {
		return Wrap(operation, context.Canceled)
	}
	if stderrors.Is(err, context.DeadlineExceeded) {
		return Wrap(operation, stderrors.Join(ErrTimeout, err))
	}
	return Wrap(operation, stderrors.Join(ErrUnavailable, err))
}

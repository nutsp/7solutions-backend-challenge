package errors

import (
	"context"
	stderrors "errors"
	"testing"
)

func TestWrappedErrorsKeepIdentity(t *testing.T) {
	tests := []struct {
		name string
		kind error
	}{
		{"nil", nil}, {"invalid", ErrInvalid}, {"not found", ErrNotFound}, {"conflict", ErrConflict},
		{"unauthorized", ErrUnauthorized}, {"forbidden", ErrForbidden}, {"session limit", ErrSessionLimit}, {"unavailable", ErrUnavailable}, {"timeout", ErrTimeout},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Wrap("read user", Wrap("downstream", tt.kind))
			if !stderrors.Is(err, tt.kind) {
				t.Fatalf("lost error identity: %v", err)
			}
		})
	}
}

func TestDependencyFailuresKeepKindAndCause(t *testing.T) {
	cause := stderrors.New("dependency lost")
	tests := []struct {
		name  string
		input error
		want  error
	}{
		{"nil", nil, nil}, {"canceled", context.Canceled, context.Canceled}, {"timeout", context.DeadlineExceeded, ErrTimeout}, {"unavailable", cause, ErrUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := FromDependency("redis", tt.input)
			if !stderrors.Is(err, tt.want) || !stderrors.Is(err, tt.input) {
				t.Fatalf("kind or underlying cause lost: %v", err)
			}
		})
	}
}

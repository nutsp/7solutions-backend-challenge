// Package requestcontext carries a validated correlation ID across transport boundaries.
package requestcontext

import (
	"context"
	"crypto/rand"
	"encoding/hex"
)

const Header = "X-Request-ID"

const MetadataKey = "x-request-id"

type key struct{}

func New(ctx context.Context, candidate string) (context.Context, error) {
	if !Valid(candidate) {
		raw := make([]byte, 16)
		if _, err := rand.Read(raw); err != nil {
			return ctx, err
		}
		candidate = hex.EncodeToString(raw)
	}
	return context.WithValue(ctx, key{}, candidate), nil
}

// Valid bounds header/log input and permits only a small printable ASCII alphabet.
func Valid(id string) bool {
	if len(id) == 0 || len(id) > 128 {
		return false
	}
	for _, c := range id {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' || c == '.') {
			return false
		}
	}
	return true
}

func ID(ctx context.Context) string {

	value, _ := ctx.Value(key{}).(string)
	return value
}

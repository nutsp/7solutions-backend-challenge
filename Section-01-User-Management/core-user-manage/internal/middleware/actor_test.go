package middleware

import (
	"context"
	"testing"

	"google.golang.org/grpc/metadata"
)

func TestActorWithoutServiceToken(t *testing.T) {
	for _, tt := range []struct {
		name    string
		headers []string
		user    string
		caller  string
	}{
		{
			name: "no metadata",
		},
		{
			name:    "caller metadata",
			headers: []string{"x-user-id", "user", "x-service-name", "orch"},
			user:    "user",
			caller:  "orch",
		},
		{
			name:    "duplicate user",
			headers: []string{"x-user-id", "one", "x-user-id", "two"},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p := Actor(metadata.NewIncomingContext(context.Background(), metadata.Pairs(tt.headers...)))
			if p.UserID != tt.user || p.Caller != tt.caller {
				t.Fatalf("unexpected actor: %+v", p)
			}
		})
	}
}

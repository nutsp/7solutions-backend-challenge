package middleware

import (
	"context"

	"google.golang.org/grpc/metadata"

	"github.com/nutsp/7solutions-backend-challenge/core-authentication/pkg/grpcclient"
)

// Actor reads caller metadata forwarded by the orchestration layer.
func Actor(ctx context.Context) grpcclient.Principal {
	if p := grpcclient.Actor(ctx); p != (grpcclient.Principal{}) {
		return p
	}
	md, _ := metadata.FromIncomingContext(ctx)
	return grpcclient.Principal{
		UserID: first(md.Get("x-user-id")),
		Caller: first(md.Get("x-service-name")),
	}
}

func first(values []string) string {
	if len(values) != 1 {
		return ""
	}
	return values[0]
}

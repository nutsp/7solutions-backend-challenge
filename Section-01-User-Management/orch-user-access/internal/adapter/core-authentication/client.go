package coreauthadapter

import (
	"context"

	"google.golang.org/grpc"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	domain "github.com/nutsp/7solutions-backend-challenge/orch-user-access/internal/domain"
	grpcerrors "github.com/nutsp/7solutions-backend-challenge/orch-user-access/pkg/errors/grpc"
	"github.com/nutsp/7solutions-backend-challenge/orch-user-access/pkg/grpcclient"
	pb "github.com/nutsp/7solutions-backend-challenge/orch-user-access/pkg/proto/core-authentication"
)

type AuthClient struct {
	auth   pb.AuthenticationServiceClient
	health healthpb.HealthClient
}

func New(conn *grpc.ClientConn) *AuthClient {
	return &AuthClient{
		auth:   pb.NewAuthenticationServiceClient(conn),
		health: healthpb.NewHealthClient(conn),
	}
}

func (c *AuthClient) AuthorizeDelete(ctx context.Context, raw, id string) (domain.Principal, error) {
	p, err := c.auth.AuthorizeDelete(ctx, &pb.DeleteUserRequest{
		AccessToken: raw,
		UserId:      id,
	})
	if err != nil {
		return domain.Principal{}, grpcerrors.FromDownstream("core-authentication", err)
	}
	return domain.Principal{
		UserID: p.UserId,
	}, nil
}

func (c *AuthClient) RevokeUser(ctx context.Context, p domain.Principal, id string) error {
	_, err := c.auth.RevokeUser(grpcclient.WithActor(ctx, p.UserID), &pb.UserID{
		Id: id,
	})
	return grpcerrors.FromDownstream("core-authentication", err)
}

func (c *AuthClient) Ready(ctx context.Context) error {
	return grpcerrors.FromDownstream("core-authentication", grpcclient.CheckHealth(ctx, c.health))
}

func (c *AuthClient) Login(ctx context.Context, in domain.Login) (domain.Tokens, error) {
	v, err := c.auth.Login(ctx, &pb.LoginRequest{
		Email:    in.Email,
		Password: in.Password,
	})
	if err != nil {
		return domain.Tokens{}, grpcerrors.FromDownstream("core-authentication", err)
	}
	return tokens(v), nil
}

func (c *AuthClient) Authenticate(ctx context.Context, raw string) (domain.Principal, error) {
	v, err := c.auth.Authenticate(ctx, &pb.AccessToken{
		AccessToken: raw,
	})
	if err != nil {
		return domain.Principal{}, grpcerrors.FromDownstream("core-authentication", err)
	}
	return domain.Principal{
		UserID: v.UserId,
	}, nil
}

func (c *AuthClient) Refresh(ctx context.Context, raw string) (domain.Tokens, error) {
	v, err := c.auth.Refresh(ctx, &pb.RefreshToken{
		RefreshToken: raw,
	})
	if err != nil {
		return domain.Tokens{}, grpcerrors.FromDownstream("core-authentication", err)
	}
	return tokens(v), nil
}

func (c *AuthClient) Logout(ctx context.Context, raw string) error {
	_, err := c.auth.Logout(ctx, &pb.AccessToken{
		AccessToken: raw,
	})
	return grpcerrors.FromDownstream("core-authentication", err)
}

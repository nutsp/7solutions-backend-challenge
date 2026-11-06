package coreuseradapter

import (
	"context"

	domain "github.com/nutsp/7solutions-backend-challenge/core-authentication/internal/domain"
	grpcerrors "github.com/nutsp/7solutions-backend-challenge/core-authentication/pkg/errors/grpc"
	pb "github.com/nutsp/7solutions-backend-challenge/core-authentication/pkg/proto/core-user-manage"
)

type Client struct {
	rpc pb.IdentityServiceClient
}

func New(rpc pb.IdentityServiceClient) *Client {

	return &Client{
		rpc: rpc,
	}
}

func (c *Client) Get(ctx context.Context, id string) (domain.Profile, error) {
	u, err := c.rpc.GetInternal(ctx, &pb.UserID{
		Id: id,
	})
	if err != nil {
		return domain.Profile{}, grpcerrors.FromDownstream("core-user-manage", err)
	}
	return domain.Profile{
		ID: u.Id,
	}, nil
}

func (c *Client) Credential(ctx context.Context, email string) (domain.Credential, error) {
	credential, err := c.rpc.FindCredential(ctx, &pb.Email{
		Email: email,
	})
	if err != nil {
		return domain.Credential{}, grpcerrors.FromDownstream("core-user-manage", err)
	}
	return domain.Credential{
		UserID:       credential.UserId,
		PasswordHash: credential.PasswordHash,
	}, nil
}

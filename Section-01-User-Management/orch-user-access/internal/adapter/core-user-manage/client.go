package coreuseradapter

import (
	"context"

	"google.golang.org/grpc"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	domain "github.com/nutsp/7solutions-backend-challenge/orch-user-access/internal/domain"
	grpcerrors "github.com/nutsp/7solutions-backend-challenge/orch-user-access/pkg/errors/grpc"
	"github.com/nutsp/7solutions-backend-challenge/orch-user-access/pkg/grpcclient"
	pb "github.com/nutsp/7solutions-backend-challenge/orch-user-access/pkg/proto/core-user-manage"
)

type UserClient struct {
	identity pb.IdentityServiceClient
	health   healthpb.HealthClient
}

func New(conn *grpc.ClientConn) *UserClient {
	return &UserClient{
		identity: pb.NewIdentityServiceClient(conn),
		health:   healthpb.NewHealthClient(conn),
	}
}

func (c *UserClient) Create(ctx context.Context, in domain.Register) (domain.User, error) {
	u, err := c.identity.CreateUser(ctx, &pb.CreateUserRequest{
		Name:     in.Name,
		Email:    in.Email,
		Password: in.Password,
	})
	if err != nil {
		return domain.User{}, grpcerrors.FromDownstream("core-user-manage", err)
	}
	return user(u), nil
}

func (c *UserClient) Delete(ctx context.Context, p domain.Principal, id string) error {
	_, err := c.identity.DeleteUser(grpcclient.WithActor(ctx, p.UserID), &pb.UserID{
		Id: id,
	})
	return grpcerrors.FromDownstream("core-user-manage", err)
}

func (c *UserClient) Ready(ctx context.Context) error {
	return grpcerrors.FromDownstream("core-user-manage", grpcclient.CheckHealth(ctx, c.health))
}

func (c *UserClient) Get(ctx context.Context, p domain.Principal, id string) (domain.User, error) {
	v, err := c.identity.GetUser(grpcclient.WithActor(ctx, p.UserID), &pb.UserID{
		Id: id,
	})
	if err != nil {
		return domain.User{}, grpcerrors.FromDownstream("core-user-manage", err)
	}
	return user(v), nil
}

func (c *UserClient) List(ctx context.Context, p domain.Principal, limit int, after string) (domain.Page, error) {
	v, err := c.identity.ListUsers(grpcclient.WithActor(ctx, p.UserID), &pb.ListUsersRequest{
		Limit: int32(limit),
		After: after,
	})
	if err != nil {
		return domain.Page{}, grpcerrors.FromDownstream("core-user-manage", err)
	}
	out := domain.Page{
		Users:      []domain.User{},
		NextCursor: v.NextCursor,
	}
	for _, u := range v.Users {
		out.Users = append(out.Users, user(u))
	}
	return out, nil
}

func (c *UserClient) Update(ctx context.Context, p domain.Principal, id string, in domain.Patch) (domain.User, error) {
	v, err := c.identity.UpdateUser(grpcclient.WithActor(ctx, p.UserID), &pb.UpdateUserRequest{
		Id:    id,
		Name:  in.Name,
		Email: in.Email,
	})
	if err != nil {
		return domain.User{}, grpcerrors.FromDownstream("core-user-manage", err)
	}
	return user(v), nil
}

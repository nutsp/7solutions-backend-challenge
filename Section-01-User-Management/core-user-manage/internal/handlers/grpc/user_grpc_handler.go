package grpchandler

import (
	"context"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel/trace"

	domain "github.com/nutsp/7solutions-backend-challenge/core-user-manage/internal/domain"
	middleware "github.com/nutsp/7solutions-backend-challenge/core-user-manage/internal/middleware"
	grpcerrors "github.com/nutsp/7solutions-backend-challenge/core-user-manage/pkg/errors/grpc"
	"github.com/nutsp/7solutions-backend-challenge/core-user-manage/pkg/logger"
	pb "github.com/nutsp/7solutions-backend-challenge/core-user-manage/pkg/proto/core-user-manage"
	"github.com/nutsp/7solutions-backend-challenge/core-user-manage/pkg/requestcontext"
)

type Server struct {
	pb.UnimplementedIdentityServiceServer
	users domain.UserService
	log   *slog.Logger
}

func New(users domain.UserService, log *slog.Logger) *Server {

	return &Server{
		users: users,
		log:   log,
	}
}

func User(v domain.User) *pb.User {
	return &pb.User{
		Id:        v.ID,
		Name:      v.Name,
		Email:     v.Email,
		CreatedAt: v.CreatedAt.Format(time.RFC3339Nano),
		UpdatedAt: v.UpdatedAt.Format(time.RFC3339Nano),
	}
}

func (s *Server) CreateUser(ctx context.Context, in *pb.CreateUserRequest) (*pb.User, error) {
	ctx = auditContext(ctx)
	u, err := s.users.Create(ctx, domain.Create{
		Password: in.Password,
		Name:     in.Name,
		Email:    in.Email,
	})
	if err != nil {
		return nil, grpcerrors.ToStatus(err, logger.WithContext(ctx, s.log))
	}
	return User(u), nil
}

func (s *Server) GetUser(ctx context.Context, in *pb.UserID) (*pb.User, error) {
	u, err := s.users.Get(ctx, actor(ctx), in.Id)
	if err != nil {
		return nil, grpcerrors.ToStatus(err, logger.WithContext(ctx, s.log))
	}
	return User(u), nil
}

func (s *Server) GetInternal(ctx context.Context, in *pb.UserID) (*pb.User, error) {
	u, err := s.users.Internal(ctx, in.Id)
	if err != nil {
		return nil, grpcerrors.ToStatus(err, logger.WithContext(ctx, s.log))
	}
	return User(u), nil
}

func (s *Server) FindCredential(ctx context.Context, in *pb.Email) (*pb.Credential, error) {
	u, err := s.users.FindByEmail(ctx, in.Email)
	if err != nil {
		return nil, grpcerrors.ToStatus(err, logger.WithContext(ctx, s.log))
	}
	return &pb.Credential{
		UserId:       u.ID,
		PasswordHash: u.PasswordHash,
	}, nil
}

func (s *Server) ListUsers(ctx context.Context, in *pb.ListUsersRequest) (*pb.ListUsersResponse, error) {
	p, err := s.users.List(ctx, actor(ctx), int(in.Limit), in.After)
	if err != nil {
		return nil, grpcerrors.ToStatus(err, logger.WithContext(ctx, s.log))
	}
	out := &pb.ListUsersResponse{
		Users:      []*pb.User{},
		NextCursor: p.NextCursor,
	}
	for _, u := range p.Users {
		out.Users = append(out.Users, User(u))
	}
	return out, nil
}

func (s *Server) UpdateUser(ctx context.Context, in *pb.UpdateUserRequest) (*pb.User, error) {
	ctx = auditContext(ctx)
	u, err := s.users.Update(ctx, actor(ctx), in.Id, domain.Patch{
		Name:  in.Name,
		Email: in.Email,
	})
	if err != nil {
		return nil, grpcerrors.ToStatus(err, logger.WithContext(ctx, s.log))
	}
	return User(u), nil
}

func (s *Server) DeleteUser(ctx context.Context, in *pb.UserID) (*pb.Empty, error) {
	ctx = auditContext(ctx)
	if err := s.users.DeleteAuthorized(ctx, actor(ctx), in.Id); err != nil {
		return nil, grpcerrors.ToStatus(err, logger.WithContext(ctx, s.log))
	}
	return &pb.Empty{}, nil
}

func actor(ctx context.Context) domain.Principal {
	p := middleware.Actor(ctx)
	return domain.Principal{
		UserID: p.UserID,
	}
}

func auditContext(ctx context.Context) context.Context {
	caller := middleware.Actor(ctx)
	traceID := ""
	if span := trace.SpanContextFromContext(ctx); span.IsValid() {
		traceID = span.TraceID().String()
	}
	return domain.WithAuditContext(ctx, domain.AuditContext{
		CallerService: caller.Caller,
		RequestID:     requestcontext.ID(ctx),
		TraceID:       traceID,
	})
}

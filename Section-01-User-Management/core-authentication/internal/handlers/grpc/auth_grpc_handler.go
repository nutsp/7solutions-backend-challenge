package grpchandler

import (
	"context"
	"log/slog"
	"time"

	domain "github.com/nutsp/7solutions-backend-challenge/core-authentication/internal/domain"
	middleware "github.com/nutsp/7solutions-backend-challenge/core-authentication/internal/middleware"
	apperrors "github.com/nutsp/7solutions-backend-challenge/core-authentication/pkg/errors"
	grpcerrors "github.com/nutsp/7solutions-backend-challenge/core-authentication/pkg/errors/grpc"
	"github.com/nutsp/7solutions-backend-challenge/core-authentication/pkg/logger"
	pb "github.com/nutsp/7solutions-backend-challenge/core-authentication/pkg/proto/core-authentication"
)

type Server struct {
	pb.UnimplementedAuthenticationServiceServer
	auth domain.AuthenticationService
	log  *slog.Logger
}

func New(auth domain.AuthenticationService, log *slog.Logger) *Server {
	return &Server{
		auth: auth,
		log:  log,
	}
}

func (s *Server) Login(ctx context.Context, in *pb.LoginRequest) (*pb.TokenPair, error) {
	v, err := s.auth.Login(ctx, in.Email, in.Password)
	if err != nil {
		return nil, grpcerrors.ToStatus(err, logger.WithContext(ctx, s.log))
	}
	return pair(v), nil
}

func (s *Server) Authenticate(ctx context.Context, in *pb.AccessToken) (*pb.Principal, error) {
	v, err := s.auth.Authenticate(ctx, in.AccessToken)
	if err != nil {
		return nil, grpcerrors.ToStatus(err, logger.WithContext(ctx, s.log))
	}
	return &pb.Principal{
		UserId: v.UserID,
	}, nil
}

func (s *Server) Refresh(ctx context.Context, in *pb.RefreshToken) (*pb.TokenPair, error) {
	v, err := s.auth.Refresh(ctx, in.RefreshToken)
	if err != nil {
		return nil, grpcerrors.ToStatus(err, logger.WithContext(ctx, s.log))
	}
	return pair(v), nil
}

func (s *Server) Logout(ctx context.Context, in *pb.AccessToken) (*pb.Empty, error) {
	if err := s.auth.Logout(ctx, in.AccessToken); err != nil {
		return nil, grpcerrors.ToStatus(err, logger.WithContext(ctx, s.log))
	}
	return &pb.Empty{}, nil
}

func (s *Server) AuthorizeDelete(ctx context.Context, in *pb.DeleteUserRequest) (*pb.Principal, error) {
	p, err := s.auth.AuthorizeDelete(ctx, in.AccessToken, in.UserId)
	if err != nil {
		return nil, grpcerrors.ToStatus(err, logger.WithContext(ctx, s.log))
	}
	return &pb.Principal{
		UserId: p.UserID,
	}, nil
}

func (s *Server) RevokeUser(ctx context.Context, in *pb.UserID) (*pb.Empty, error) {
	p := middleware.Actor(ctx)
	if p.UserID == "" || p.UserID != in.Id {
		return nil, grpcerrors.ToStatus(apperrors.ErrForbidden, s.log)
	}
	if err := s.auth.RevokeUser(ctx, in.Id); err != nil {
		return nil, grpcerrors.ToStatus(err, logger.WithContext(ctx, s.log))
	}
	return &pb.Empty{}, nil
}

func pair(v domain.Tokens) *pb.TokenPair {
	return &pb.TokenPair{
		AccessToken:  v.AccessToken,
		RefreshToken: v.RefreshToken,
		TokenType:    "Bearer",
		ExpiresAt:    v.ExpiresAt.Format(time.RFC3339),
		UserId:       v.UserID,
	}
}

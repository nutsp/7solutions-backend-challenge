package coreuseradapter

import (
	domain "github.com/nutsp/7solutions-backend-challenge/orch-user-access/internal/domain"
	pb "github.com/nutsp/7solutions-backend-challenge/orch-user-access/pkg/proto/core-user-manage"
)

func user(in *pb.User) domain.User {
	return domain.User{
		ID:        in.Id,
		Name:      in.Name,
		Email:     in.Email,
		CreatedAt: in.CreatedAt,
		UpdatedAt: in.UpdatedAt,
	}
}

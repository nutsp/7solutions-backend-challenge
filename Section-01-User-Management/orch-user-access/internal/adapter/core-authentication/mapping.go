package coreauthadapter

import (
	domain "github.com/nutsp/7solutions-backend-challenge/orch-user-access/internal/domain"
	pb "github.com/nutsp/7solutions-backend-challenge/orch-user-access/pkg/proto/core-authentication"
)

func tokens(in *pb.TokenPair) domain.Tokens {
	return domain.Tokens{
		AccessToken:  in.AccessToken,
		RefreshToken: in.RefreshToken,
		TokenType:    in.TokenType,
		ExpiresAt:    in.ExpiresAt,
		UserID:       in.UserId,
	}
}

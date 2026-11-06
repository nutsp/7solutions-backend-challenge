package services

import (
	"context"

	domain "github.com/nutsp/7solutions-backend-challenge/orch-user-access/internal/domain"
)

var _ domain.UserAccessService = (*Access)(nil)

type Access struct {
	users domain.CoreUserAdapter
	auth  domain.CoreAuthenticationAdapter
}

func New(users domain.CoreUserAdapter, auth domain.CoreAuthenticationAdapter) *Access {
	return &Access{
		users: users,
		auth:  auth,
	}
}

func (a *Access) Register(ctx context.Context, in domain.Register) (domain.Registration, error) {
	user, err := a.users.Create(ctx, in)
	if err != nil {
		return domain.Registration{}, err
	}
	tokens, err := a.auth.Login(ctx, domain.Login{
		Email:    user.Email,
		Password: in.Password,
	})
	if err != nil {
		return domain.Registration{}, err
	}
	return domain.Registration{
		User:   user,
		Tokens: tokens,
	}, nil
}

func (a *Access) Login(ctx context.Context, in domain.Login) (domain.Tokens, error) {
	return a.auth.Login(ctx, in)
}

func (a *Access) Authenticate(ctx context.Context, raw string) (domain.Principal, error) {
	return a.auth.Authenticate(ctx, raw)
}

func (a *Access) Refresh(ctx context.Context, raw string) (domain.Tokens, error) {
	return a.auth.Refresh(ctx, raw)
}

func (a *Access) Logout(ctx context.Context, raw string) error {

	return a.auth.Logout(ctx, raw)
}

func (a *Access) Get(ctx context.Context, p domain.Principal, id string) (domain.User, error) {
	return a.users.Get(ctx, p, id)
}

func (a *Access) List(ctx context.Context, p domain.Principal, limit int, after string) (domain.Page, error) {
	return a.users.List(ctx, p, limit, after)
}

func (a *Access) Update(ctx context.Context, p domain.Principal, id string, in domain.Patch) (domain.User, error) {
	return a.users.Update(ctx, p, id, in)
}

func (a *Access) Delete(ctx context.Context, raw, id string) error {
	p, err := a.auth.AuthorizeDelete(ctx, raw, id)
	if err != nil {
		return err
	}
	if err := a.users.Delete(ctx, p, id); err != nil {
		return err
	}
	return a.auth.RevokeUser(ctx, p, id)
}

func (a *Access) Ready(ctx context.Context) error {
	if err := a.users.Ready(ctx); err != nil {
		return err
	}
	return a.auth.Ready(ctx)
}

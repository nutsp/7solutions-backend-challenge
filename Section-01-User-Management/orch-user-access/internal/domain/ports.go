package domain

import "context"

// UserAccessService is the inbound application contract used by the REST adapter.
type UserAccessService interface {
	Register(ctx context.Context, input Register) (Registration, error)
	Login(ctx context.Context, input Login) (Tokens, error)
	Authenticate(ctx context.Context, accessToken string) (Principal, error)
	Refresh(ctx context.Context, refreshToken string) (Tokens, error)
	Logout(ctx context.Context, accessToken string) error
	Get(ctx context.Context, actor Principal, userID string) (User, error)
	List(ctx context.Context, actor Principal, limit int, afterID string) (Page, error)
	Update(ctx context.Context, actor Principal, userID string, patch Patch) (User, error)
	Delete(ctx context.Context, accessToken, userID string) error
	Ready(ctx context.Context) error
}

// CoreUserAdapter is the outbound contract for calling core-user-manage over gRPC.
type CoreUserAdapter interface {
	Create(ctx context.Context, input Register) (User, error)
	Get(ctx context.Context, actor Principal, userID string) (User, error)
	List(ctx context.Context, actor Principal, limit int, afterID string) (Page, error)
	Update(ctx context.Context, actor Principal, userID string, patch Patch) (User, error)
	Delete(ctx context.Context, actor Principal, userID string) error
	Ready(ctx context.Context) error
}

// CoreAuthenticationAdapter is the outbound contract for calling core-authentication over gRPC.
type CoreAuthenticationAdapter interface {
	Login(ctx context.Context, input Login) (Tokens, error)
	Authenticate(ctx context.Context, accessToken string) (Principal, error)
	Refresh(ctx context.Context, refreshToken string) (Tokens, error)
	Logout(ctx context.Context, accessToken string) error
	AuthorizeDelete(ctx context.Context, accessToken, userID string) (Principal, error)
	RevokeUser(ctx context.Context, actor Principal, userID string) error
	Ready(ctx context.Context) error
}

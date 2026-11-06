package httphandler

import domain "github.com/nutsp/7solutions-backend-challenge/orch-user-access/internal/domain"

type registerJSON struct {
	Name     string `json:"name" validate:"required,validutf8,nocontrol,max=100"`
	Email    string `json:"email" validate:"required,validutf8,email,maxbytes=254"`
	Password string `json:"password" validate:"validutf8,minbytes=12,maxbytes=72"`
}

type loginJSON struct {
	Email    string `json:"email" validate:"required,validutf8,email,maxbytes=254"`
	Password string `json:"password" validate:"required,validutf8,maxbytes=72"`
}

type patchJSON struct {
	Name  *string `json:"name,omitempty" validate:"omitnil,required,validutf8,nocontrol,min=1,max=100"`
	Email *string `json:"email,omitempty" validate:"omitnil,required,validutf8,email,maxbytes=254"`
}

type userJSON struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	UpdatedAt string `json:"updated_at"`
	CreatedAt string `json:"created_at"`
}

type tokensJSON struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresAt    string `json:"expires_at"`
	UserID       string `json:"user_id"`
}

type pageJSON struct {
	Users      []userJSON `json:"users"`
	NextCursor string     `json:"next_cursor,omitempty"`
}

func (r registerJSON) value() domain.Register {
	return domain.Register{
		Name:     r.Name,
		Email:    r.Email,
		Password: r.Password,
	}
}

func (p patchJSON) value() domain.Patch {
	return domain.Patch{
		Name:  p.Name,
		Email: p.Email,
	}
}

func publicUser(u domain.User) userJSON {
	return userJSON{
		ID:        u.ID,
		Name:      u.Name,
		Email:     u.Email,
		UpdatedAt: u.UpdatedAt,
		CreatedAt: u.CreatedAt,
	}
}

func publicTokens(t domain.Tokens) tokensJSON {
	return tokensJSON{
		AccessToken:  t.AccessToken,
		RefreshToken: t.RefreshToken,
		TokenType:    t.TokenType,
		ExpiresAt:    t.ExpiresAt,
		UserID:       t.UserID,
	}
}

func publicPage(p domain.Page) pageJSON {
	out := pageJSON{
		Users:      []userJSON{},
		NextCursor: p.NextCursor,
	}
	for _, u := range p.Users {
		out.Users = append(out.Users, publicUser(u))
	}
	return out
}

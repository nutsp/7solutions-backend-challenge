package domain

type User struct {
	ID        string
	Name      string
	Email     string
	UpdatedAt string
	CreatedAt string
}

type Register struct {
	Name     string
	Email    string
	Password string
}

type Login struct {
	Email    string
	Password string
}

type Patch struct {
	Name  *string
	Email *string
}

type Tokens struct {
	AccessToken  string
	RefreshToken string
	TokenType    string
	ExpiresAt    string
	UserID       string
}

type Principal struct {
	UserID string
}

type Page struct {
	Users      []User
	NextCursor string
}

type Registration struct {
	User   User
	Tokens Tokens
}

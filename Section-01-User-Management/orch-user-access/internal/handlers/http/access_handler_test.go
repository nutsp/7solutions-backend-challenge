package httphandler

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nutsp/7solutions-backend-challenge/orch-user-access/internal/domain"
	"github.com/nutsp/7solutions-backend-challenge/orch-user-access/internal/mocks"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestVersionedResourceRoutes(t *testing.T) {
	const id = "0123456789abcdef0123456789abcdef"
	user := domain.User{ID: id, Name: "Alice", Email: "alice@example.com"}
	principal := domain.Principal{UserID: id}
	tokens := domain.Tokens{AccessToken: "access", RefreshToken: "refresh", UserID: id}
	authorize := func(app *mocks.MockUserAccessService) {
		app.EXPECT().Authenticate(mock.Anything, "access").Return(principal, nil).Once()
	}
	tests := []struct {
		name, method, path, body, location string
		status                             int
		setup                              func(*mocks.MockUserAccessService)
		check                              func(*testing.T, map[string]json.RawMessage)
	}{
		{name: "create user includes initial tokens", method: http.MethodPost, path: "/api/v1/users", body: `{"name":"Alice","email":"alice@example.com","password":"correct-horse-battery"}`, status: 201, location: "/api/v1/users/" + id,
			setup: func(app *mocks.MockUserAccessService) {
				app.EXPECT().Register(mock.Anything, domain.Register{Name: "Alice", Email: "alice@example.com", Password: "correct-horse-battery"}).Return(domain.Registration{User: user, Tokens: tokens}, nil).Once()
			},
			check: func(t *testing.T, data map[string]json.RawMessage) {
				require.Contains(t, data, "user")
				require.Contains(t, data, "tokens")
			}},
		{name: "create session", method: http.MethodPost, path: "/api/v1/sessions", body: `{"email":"alice@example.com","password":"correct-horse-battery"}`, status: 201, location: "/api/v1/sessions/current",
			setup: func(app *mocks.MockUserAccessService) {
				app.EXPECT().Login(mock.Anything, domain.Login{Email: "alice@example.com", Password: "correct-horse-battery"}).Return(tokens, nil).Once()
			}},
		{name: "current profile takes precedence over ID", method: http.MethodGet, path: "/api/v1/users/me", status: 200,
			setup: func(app *mocks.MockUserAccessService) {
				authorize(app)
				app.EXPECT().Get(mock.Anything, principal, id).Return(user, nil).Once()
			}},
		{name: "get user", method: http.MethodGet, path: "/api/v1/users/" + id, status: 200,
			setup: func(app *mocks.MockUserAccessService) {
				authorize(app)
				app.EXPECT().Get(mock.Anything, principal, id).Return(user, nil).Once()
			}},
		{name: "list with pagination", method: http.MethodGet, path: "/api/v1/users?limit=2&after=" + id, status: 200,
			setup: func(app *mocks.MockUserAccessService) {
				authorize(app)
				app.EXPECT().List(mock.Anything, principal, 2, id).Return(domain.Page{Users: []domain.User{user}}, nil).Once()
			}},
		{name: "patch user", method: http.MethodPatch, path: "/api/v1/users/" + id, body: `{"name":"Updated"}`, status: 200,
			setup: func(app *mocks.MockUserAccessService) {
				authorize(app)
				app.EXPECT().Update(mock.Anything, principal, id, mock.MatchedBy(func(p domain.Patch) bool { return p.Name != nil && *p.Name == "Updated" && p.Email == nil })).Return(user, nil).Once()
			}},
		{name: "delete user keeps dedicated authorization workflow", method: http.MethodDelete, path: "/api/v1/users/" + id, status: 204,
			setup: func(app *mocks.MockUserAccessService) {
				app.EXPECT().Delete(mock.Anything, "access", id).Return(nil).Once()
			}},
		{name: "delete current session", method: http.MethodDelete, path: "/api/v1/sessions/current", status: 204,
			setup: func(app *mocks.MockUserAccessService) {
				app.EXPECT().Logout(mock.Anything, "access").Return(nil).Once()
			}},
		{name: "rotate current session tokens", method: http.MethodPost, path: "/api/v1/sessions/current/tokens", body: `{"refresh_token":"refresh"}`, status: 200,
			setup: func(app *mocks.MockUserAccessService) {
				app.EXPECT().Refresh(mock.Anything, "refresh").Return(tokens, nil).Once()
			}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := mocks.NewMockUserAccessService(t)
			tt.setup(app)
			handler := New(app, slog.New(slog.NewTextHandler(io.Discard, nil)))
			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer access")
			res := httptest.NewRecorder()
			handler.ServeHTTP(res, req)
			require.Equal(t, tt.status, res.Code, res.Body.String())
			require.Equal(t, tt.location, res.Header().Get("Location"))
			if tt.status == 204 {
				require.Empty(t, res.Body.String())
				return
			}
			var envelope struct {
				Data map[string]json.RawMessage `json:"data"`
			}
			require.NoError(t, json.Unmarshal(res.Body.Bytes(), &envelope))
			require.NotNil(t, envelope.Data)
			if tt.check != nil {
				tt.check(t, envelope.Data)
			}
		})
	}
}

func TestRouteBoundaries(t *testing.T) {
	const id = "0123456789abcdef0123456789abcdef"
	tests := []struct {
		name, method, path string
		status             int
	}{
		{"profile requires JWT", "GET", "/api/v1/users/me", 401},
		{"list requires JWT", "GET", "/api/v1/users", 401},
		{"get requires JWT", "GET", "/api/v1/users/" + id, 401},
		{"patch requires JWT", "PATCH", "/api/v1/users/" + id, 401},
		{"delete requires JWT", "DELETE", "/api/v1/users/" + id, 401},
		{"logout requires token", "DELETE", "/api/v1/sessions/current", 401},
		{"registration alias removed", "POST", "/register", 404},
		{"login action removed", "POST", "/login", 404},
		{"logout action removed", "POST", "/logout", 404},
		{"profile alias removed", "GET", "/profile", 404},
		{"unversioned users removed", "POST", "/users", 404},
		{"unversioned list removed", "GET", "/users", 404},
		{"refresh action removed", "POST", "/sessions/refresh", 404},
		{"unsupported user method", "PUT", "/api/v1/users/" + id, 405},
		{"unsupported session method", "POST", "/api/v1/sessions/current", 405},
		{"liveness remains unversioned", "GET", "/healthzc", 200},
		{"legacy liveness remains unversioned", "GET", "/healthz", 200},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := mocks.NewMockUserAccessService(t)
			res := httptest.NewRecorder()
			New(app, slog.New(slog.NewTextHandler(io.Discard, nil))).ServeHTTP(res, httptest.NewRequest(tt.method, tt.path, nil))
			require.Equal(t, tt.status, res.Code, res.Body.String())
		})
	}
}

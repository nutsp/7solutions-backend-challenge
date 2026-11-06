//go:build integration

package integration

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"
)

const password = "integration-password-2026"

var (
	requestID   = regexp.MustCompile(`^[A-Za-z0-9._-]{1,128}$`)
	traceID     = regexp.MustCompile(`^[0-9a-f]{32}$`)
	traceParent = regexp.MustCompile(`^00-[0-9a-f]{32}-[0-9a-f]{16}-[0-9a-f]{2}$`)
)

type user struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
type tokens struct {
	Access  string `json:"access_token"`
	Refresh string `json:"refresh_token"`
}
type registration struct {
	User   user   `json:"user"`
	Tokens tokens `json:"tokens"`
}
type reply struct {
	status int
	data   json.RawMessage
	err    error
}
type api struct {
	base   string
	client *http.Client
}

// call is goroutine-safe; failures are reported by the test after joining workers.
func (a api) call(method, path string, body any, token string, extra map[string]string) reply {
	var payload []byte
	var err error
	if body != nil {
		payload, err = json.Marshal(body)
		if err != nil {
			return reply{err: err}
		}
	}
	req, err := http.NewRequest(method, a.base+path, bytes.NewReader(payload))
	if err != nil {
		return reply{err: err}
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for key, value := range extra {
		req.Header.Set(key, value)
	}
	res, err := a.client.Do(req)
	if err != nil {
		return reply{err: err}
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	result := reply{status: res.StatusCode, err: err}
	if err != nil {
		return result
	}
	if !requestID.MatchString(res.Header.Get("X-Request-ID")) || !traceID.MatchString(res.Header.Get("X-Trace-ID")) || !traceParent.MatchString(res.Header.Get("traceparent")) {
		result.err = fmt.Errorf("missing or invalid correlation headers")
		return result
	}
	if res.StatusCode == http.StatusNoContent {
		if len(raw) != 0 {
			result.err = fmt.Errorf("204 response has a body")
		}
		return result
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(raw, &envelope); err != nil {
		result.err = fmt.Errorf("invalid JSON envelope: %w", err)
		return result
	}
	if len(envelope) != 1 {
		result.err = fmt.Errorf("expected one response envelope field")
		return result
	}
	if res.StatusCode >= 200 && res.StatusCode < 300 {
		result.data = envelope["data"]
		if len(result.data) == 0 {
			result.err = fmt.Errorf("missing data envelope")
		}
	} else {
		var problem map[string]string
		if err := json.Unmarshal(envelope["error"], &problem); err != nil || len(problem) != 2 || problem["code"] == "" || problem["message"] == "" {
			result.err = fmt.Errorf("invalid error envelope")
		}
	}
	return result
}
func expect(t *testing.T, result reply, status int) json.RawMessage {
	t.Helper()
	if result.err != nil {
		t.Fatal(result.err)
	}
	if result.status != status {
		t.Fatalf("HTTP status %d, expected %d", result.status, status)
	}
	return result.data
}
func decode[T any](t *testing.T, raw json.RawMessage) T {
	t.Helper()
	var value T
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatal(err)
	}
	return value
}
func publicUser(t *testing.T, raw json.RawMessage) user {
	t.Helper()
	fields := decode[map[string]json.RawMessage](t, raw)
	if len(fields) != 5 {
		t.Fatal("public user must contain exactly five fields")
	}
	for _, key := range []string{"id", "name", "email", "created_at", "updated_at"} {
		if _, ok := fields[key]; !ok {
			t.Fatalf("missing user field %s", key)
		}
	}
	return decode[user](t, raw)
}

func TestUserLifecycle(t *testing.T) {
	base := os.Getenv("TEST_GATEWAY_URL")
	if base == "" {
		base = "http://127.0.0.1:8080"
	}
	a := api{base: strings.TrimRight(base, "/"), client: &http.Client{Timeout: 12 * time.Second}}
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		t.Fatal(err)
	}
	suffix := hex.EncodeToString(raw)
	created := map[string]string{}
	login := func(email string) tokens {
		return decode[tokens](t, expect(t, a.call("POST", "/api/v1/sessions", map[string]string{"email": email, "password": password}, "", nil), 201))
	}
	register := func(email string) reply {
		return a.call("POST", "/api/v1/users", map[string]string{"name": "Alice", "email": email, "password": password}, "", nil)
	}
	t.Cleanup(func() {
		for id, email := range created {
			sessionReply := a.call("POST", "/api/v1/sessions", map[string]string{"email": email, "password": password}, "", nil)
			if sessionReply.err != nil || sessionReply.status != 201 {
				t.Errorf("could not authenticate a temporary account for cleanup")
				continue
			}
			var session tokens
			if err := json.Unmarshal(sessionReply.data, &session); err != nil {
				t.Error("could not decode cleanup session")
				continue
			}
			result := a.call("DELETE", "/api/v1/users/"+id, nil, session.Access, nil)
			if result.err != nil || result.status != 204 {
				t.Error("could not delete a temporary account")
			}
		}
	})
	email := "alice-" + suffix + "@example.com"
	var first registration
	var own tokens
	var path, newEmail string
	tests := []struct {
		name string
		run  func(*testing.T)
	}{
		{"readiness and removed routes", func(t *testing.T) {
			expect(t, a.call("GET", "/readyz", nil, "", nil), 200)
			expect(t, a.call("GET", "/healthzc", nil, "", nil), 200)
			for _, route := range []struct{ method, path string }{{"POST", "/register"}, {"POST", "/login"}, {"POST", "/logout"}, {"GET", "/profile"}, {"GET", "/users"}, {"POST", "/sessions/refresh"}} {
				expect(t, a.call(route.method, route.path, nil, "", nil), 404)
			}
		}},
		{"registration and JWT", func(t *testing.T) {
			data := expect(t, register(email), 201)
			first = decode[registration](t, data)
			created[first.User.ID] = email
			body := decode[map[string]json.RawMessage](t, data)
			publicUser(t, body["user"])
			if first.User.UpdatedAt != first.User.CreatedAt {
				t.Fatal("initial timestamps differ")
			}
			own = first.Tokens
			path = "/api/v1/users/" + first.User.ID
			expect(t, register(email), 409)
			expect(t, a.call("GET", path, nil, "", nil), 401)
			expect(t, a.call("GET", path, nil, "invalid", nil), 401)
			profile := publicUser(t, expect(t, a.call("GET", "/api/v1/users/me", nil, own.Access, nil), 200))
			if profile != first.User {
				t.Fatal("profile differs from created user")
			}
			expect(t, a.call("POST", "/api/v1/sessions", map[string]string{"email": email, "password": "wrong"}, "", nil), 401)
			own = login(strings.ToUpper(email))
			for _, field := range []string{"role", "updated_at", "registration_key", "registration_fingerprint", "seller"} {
				body := map[string]string{"name": "Invalid", "email": email, "password": password, field: "invalid"}
				expect(t, a.call("POST", "/api/v1/users", body, "", nil), 400)
			}
		}},
		{"ownership and update", func(t *testing.T) {
			bobEmail := "bob-" + suffix + "@example.com"
			bob := decode[registration](t, expect(t, register(bobEmail), 201))
			created[bob.User.ID] = bobEmail
			bobPath := "/api/v1/users/" + bob.User.ID
			publicUser(t, expect(t, a.call("GET", bobPath, nil, own.Access, nil), 200))
			expect(t, a.call("PATCH", bobPath, map[string]string{"name": "Forged"}, own.Access, map[string]string{"X-User-ID": bob.User.ID, "X-Service-Name": "auth"}), 403)
			expect(t, a.call("DELETE", bobPath, nil, own.Access, nil), 403)
			expect(t, a.call("PATCH", path, map[string]string{"email": bobEmail}, own.Access, nil), 409)
			expect(t, a.call("PATCH", path, map[string]string{"updated_at": "2026-01-01T00:00:00Z"}, own.Access, nil), 400)
			newEmail = "updated-" + suffix + "@example.com"
			updated := publicUser(t, expect(t, a.call("PATCH", path, map[string]string{"name": "Updated Alice", "email": newEmail}, own.Access, nil), 200))
			created[first.User.ID] = newEmail
			if updated.CreatedAt != first.User.CreatedAt || !updated.UpdatedAt.After(first.User.UpdatedAt) || updated.Name != "Updated Alice" || updated.Email != newEmail {
				t.Fatal("incorrect update or timestamps")
			}
			expect(t, a.call("POST", "/api/v1/sessions", map[string]string{"email": email, "password": password}, "", nil), 401)
			own = login(newEmail)
		}},
		{"concurrent refresh and logout", func(t *testing.T) {
			results := make(chan reply, 2)
			for range 2 {
				go func() {
					results <- a.call("POST", "/api/v1/sessions/current/tokens", map[string]string{"refresh_token": own.Refresh}, "", nil)
				}()
			}
			success, rejected := 0, 0
			var rotated tokens
			for range 2 {
				result := <-results
				if result.err != nil {
					t.Fatal(result.err)
				}
				switch result.status {
				case 200:
					success++
					rotated = decode[tokens](t, result.data)
				case 401:
					rejected++
				default:
					t.Fatalf("unexpected refresh status %d", result.status)
				}
			}
			if success != 1 || rejected != 1 {
				t.Fatal("refresh must have exactly one winner")
			}
			expect(t, a.call("DELETE", "/api/v1/sessions/current", nil, rotated.Access, nil), 204)
			expect(t, a.call("GET", "/api/v1/users/me", nil, rotated.Access, nil), 401)
			expect(t, a.call("POST", "/api/v1/sessions/current/tokens", map[string]string{"refresh_token": rotated.Refresh}, "", nil), 401)
			own = login(newEmail)
		}},
		{"concurrent unique email", func(t *testing.T) {
			raceEmail := "race-" + suffix + "@example.com"
			results := make(chan reply, 4)
			for range 4 {
				go func() { results <- register(raceEmail) }()
			}
			// Join all requests before assertions so successful accounts always enter cleanup.
			replies := make([]reply, 0, 4)
			for range 4 {
				result := <-results
				replies = append(replies, result)
				if result.status == 201 {
					value := decode[registration](t, result.data)
					created[value.User.ID] = raceEmail
				}
			}
			success, conflicts := 0, 0
			for _, result := range replies {
				if result.err != nil {
					t.Fatal(result.err)
				}
				switch result.status {
				case 201:
					success++
				case 409:
					conflicts++
				default:
					t.Fatalf("unexpected registration status %d", result.status)
				}
			}
			if success != 1 || conflicts != 3 {
				t.Fatal("unique email constraint did not produce one winner")
			}
		}},
		{"pagination and deletion revocation", func(t *testing.T) {
			seen := map[string]bool{}
			cursor := ""
			for range 1000 {
				page := decode[struct {
					Users []json.RawMessage `json:"users"`
					Next  string            `json:"next_cursor"`
				}](t, expect(t, a.call("GET", "/api/v1/users?limit=2&after="+cursor, nil, own.Access, nil), 200))
				for _, raw := range page.Users {
					u := publicUser(t, raw)
					if seen[u.ID] {
						t.Fatal("duplicate user across pages")
					}
					seen[u.ID] = true
				}
				cursor = page.Next
				if cursor == "" {
					break
				}
			}
			if cursor != "" {
				t.Fatal("pagination did not terminate")
			}
			for id := range created {
				if !seen[id] {
					t.Fatal("created user missing from list")
				}
			}
			expect(t, a.call("GET", "/api/v1/users", nil, "", nil), 401)
			expect(t, a.call("DELETE", path, nil, own.Access, nil), 204)
			delete(created, first.User.ID)
			expect(t, a.call("DELETE", path, nil, own.Access, nil), 204)
			expect(t, a.call("GET", "/api/v1/users/me", nil, own.Access, nil), 401)
			expect(t, a.call("POST", "/api/v1/sessions", map[string]string{"email": newEmail, "password": password}, "", nil), 401)
		}},
	}
	for _, tt := range tests {
		if !t.Run(tt.name, tt.run) {
			t.FailNow()
		}
	}
}

package securityadapter

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	domain "github.com/nutsp/7solutions-backend-challenge/core-authentication/internal/domain"
	"github.com/nutsp/7solutions-backend-challenge/core-authentication/pkg/hash"
	"github.com/nutsp/7solutions-backend-challenge/core-authentication/pkg/jwt"
)

func TestDomainClaimsTranslation(t *testing.T) {
	tests := []struct {
		name      string
		userID    string
		sessionID string
	}{{"first user", "user-1", "session-1"}, {"second user", "user-2", "session-2"}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter, err := New(hash.Config{
				Cost: 10,
			}, jwt.Config{
				Secret:   strings.Repeat("x", 32),
				Issuer:   "auth",
				Audience: "orch",
				TTL:      time.Minute,
			})
			require.NoError(t, err)
			want := domain.Claims{
				UserID:    tt.userID,
				SessionID: tt.sessionID,
			}
			raw, _, err := adapter.Issue(want)
			require.NoError(t, err)
			got, err := adapter.Parse(raw)
			require.NoError(t, err)
			require.Equal(t, want, got)
		})
	}
}

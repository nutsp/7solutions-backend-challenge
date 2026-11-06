package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	domain "github.com/nutsp/7solutions-backend-challenge/orch-user-access/internal/domain"
	apperrors "github.com/nutsp/7solutions-backend-challenge/orch-user-access/pkg/errors"
)

type userPrincipalKey struct{}

func User(ctx context.Context) domain.Principal {
	p, _ := ctx.Value(userPrincipalKey{}).(domain.Principal)
	return p
}

func Bearer(r *http.Request) (string, error) {
	values := r.Header.Values("Authorization")
	if len(values) != 1 {
		return "", apperrors.ErrUnauthorized
	}
	parts := strings.Fields(values[0])
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || len(parts[1]) > 8192 {
		return "", apperrors.ErrUnauthorized
	}
	return parts[1], nil
}

func Authenticate(verify func(context.Context, string) (domain.Principal, error), fail func(http.ResponseWriter, error)) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw, err := Bearer(c.Request)
		if err != nil {
			fail(c.Writer, err)
			c.Abort()
			return
		}
		p, err := verify(c.Request.Context(), raw)
		if err != nil {
			fail(c.Writer, err)
			c.Abort()
			return
		}
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), userPrincipalKey{}, p))
		c.Next()
	}
}

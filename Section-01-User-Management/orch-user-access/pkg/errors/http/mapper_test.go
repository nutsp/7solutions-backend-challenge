package httperrors

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	apperrors "github.com/nutsp/7solutions-backend-challenge/orch-user-access/pkg/errors"
)

func TestHTTPMappingsAreSafe(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
	}{
		{apperrors.ErrInvalid, 400}, {apperrors.ErrUnauthorized, 401}, {apperrors.ErrForbidden, 403},
		{apperrors.ErrNotFound, 404}, {apperrors.ErrConflict, 409}, {apperrors.ErrSessionLimit, 429},
		{apperrors.ErrTimeout, 504}, {context.DeadlineExceeded, 504}, {context.Canceled, 504},
		{apperrors.ErrUnavailable, 503}, {errors.New("private diagnostic"), 503},
	} {
		t.Run(tc.err.Error(), func(t *testing.T) {
			rec := httptest.NewRecorder()
			Write(rec, apperrors.Wrap("private operation", tc.err))
			if rec.Code != tc.status || strings.Contains(rec.Body.String(), "private") {
				t.Fatalf("unsafe or wrong response %d %s", rec.Code, rec.Body.String())
			}
			var body struct {
				Error struct {
					Code    string `json:"code"`
					Message string `json:"message"`
				} `json:"error"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body.Error.Code == "" || body.Error.Message == "" {
				t.Fatalf("invalid error JSON: %s", rec.Body.String())
			}
			if rec.Header().Get("Content-Type") != "application/json" {
				t.Fatal("JSON content type missing")
			}
			if tc.status == 401 && rec.Header().Get("WWW-Authenticate") != "Bearer" {
				t.Fatal("Bearer challenge missing")
			}

		})
	}
	rec := httptest.NewRecorder()
	Write(rec, nil)
	if rec.Body.Len() != 0 {
		t.Fatal("nil error wrote a response")
	}
}

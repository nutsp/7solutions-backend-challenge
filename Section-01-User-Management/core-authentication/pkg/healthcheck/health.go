// Package healthcheck provides transport-independent probes and shared HTTP health handlers.
package healthcheck

import (
	"context"
	"net/http"
	"strings"
	"time"

	apperrors "github.com/nutsp/7solutions-backend-challenge/core-authentication/pkg/errors"
	httperrors "github.com/nutsp/7solutions-backend-challenge/core-authentication/pkg/errors/http"
	"github.com/nutsp/7solutions-backend-challenge/core-authentication/pkg/httpresponse"
)

const LivenessPath = "/healthzc"

const LegacyLivenessPath = "/healthz"

const ReadinessPath = "/readyz"

type Config struct {
	ReadinessTimeout time.Duration
}

type Probe func(context.Context) error

type Handler struct {
	timeout time.Duration
	probe   Probe
}

// Nonpositive timeouts use the default two-second readiness bound.
func New(cfg Config, probe Probe) *Handler {
	if cfg.ReadinessTimeout <= 0 {
		cfg.ReadinessTimeout = 2 * time.Second
	}
	return &Handler{
		timeout: cfg.ReadinessTimeout,
		probe:   probe,
	}
}

func (*Handler) Liveness(w http.ResponseWriter, _ *http.Request) {
	httpresponse.Success(w, http.StatusOK, map[string]string{"status": "alive"})
}

func (h *Handler) Readiness(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()
	if h.probe == nil {
		httperrors.Write(w, apperrors.ErrUnavailable)
		return
	}
	if err := h.probe(ctx); err != nil {
		httperrors.Write(w, apperrors.ErrUnavailable)
		return
	}
	httpresponse.Success(w, http.StatusOK, map[string]string{"status": "ready"})
}

func IsHTTP(path string) bool {
	return path == LivenessPath || path == LegacyLivenessPath || path == ReadinessPath
}

func IsGRPC(method string) bool {

	return strings.HasPrefix(method, "/grpc.health.v1.Health/")
}

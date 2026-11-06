package httphandler

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	playground "github.com/go-playground/validator/v10"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	otelcodes "go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"github.com/nutsp/7solutions-backend-challenge/orch-user-access/internal/constant"
	domain "github.com/nutsp/7solutions-backend-challenge/orch-user-access/internal/domain"
	middleware "github.com/nutsp/7solutions-backend-challenge/orch-user-access/internal/middleware"
	apperrors "github.com/nutsp/7solutions-backend-challenge/orch-user-access/pkg/errors"
	httperrors "github.com/nutsp/7solutions-backend-challenge/orch-user-access/pkg/errors/http"
	"github.com/nutsp/7solutions-backend-challenge/orch-user-access/pkg/healthcheck"
	"github.com/nutsp/7solutions-backend-challenge/orch-user-access/pkg/httpresponse"
	"github.com/nutsp/7solutions-backend-challenge/orch-user-access/pkg/logger"
	"github.com/nutsp/7solutions-backend-challenge/orch-user-access/pkg/requestcontext"
)

type Handler struct {
	app      domain.UserAccessService
	log      *slog.Logger
	validate *playground.Validate
}

func New(app domain.UserAccessService, log *slog.Logger) http.Handler {
	h := &Handler{
		app:      app,
		log:      log,
		validate: newRequestValidator(),
	}
	r := gin.New()
	r.RedirectTrailingSlash = false
	r.RedirectFixedPath = false
	r.HandleMethodNotAllowed = true
	// Source IPs must come from the socket, never untrusted forwarded headers.
	_ = r.SetTrustedProxies(nil)
	r.Use(h.observation)
	r.NoRoute(func(c *gin.Context) {
		httpresponse.Failure(c.Writer, http.StatusNotFound, "not_found", "route not found")
	})
	r.NoMethod(func(c *gin.Context) {
		httpresponse.Failure(c.Writer, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	})
	probes := healthcheck.New(healthcheck.Config{}, app.Ready)
	r.GET(healthcheck.LivenessPath, gin.WrapF(probes.Liveness))
	r.GET(healthcheck.LegacyLivenessPath, gin.WrapF(probes.Liveness))
	r.GET(healthcheck.ReadinessPath, gin.WrapF(probes.Readiness))
	v1 := r.Group("/api/v1")
	users := v1.Group("/users")
	users.POST("", h.register)
	// Delete authorizes through Auth separately so retries can finish session cleanup.
	users.DELETE("/:id", h.delete)
	protected := users.Group("", middleware.Authenticate(app.Authenticate, httperrors.Write))
	protected.GET("", h.list)
	protected.GET("/me", h.profile)
	protected.GET("/:id", h.get)
	protected.PATCH("/:id", h.update)

	sessions := v1.Group("/sessions")
	sessions.POST("", h.login)
	sessions.DELETE("/current", h.logout)
	sessions.POST("/current/tokens", h.refresh)
	return r
}

func (h *Handler) decode(w http.ResponseWriter, r *http.Request, out any) error {
	content, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || content != "application/json" {
		return apperrors.ErrInvalid
	}
	r.Body = http.MaxBytesReader(w, r.Body, constant.MaxBodyBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		return apperrors.ErrInvalid
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return apperrors.ErrInvalid
	}
	return validateRequest(h.validate, out)
}

func (h *Handler) observation(c *gin.Context) {
	w, r := c.Writer, c.Request
	started := time.Now()
	candidate := ""
	if values := r.Header.Values(requestcontext.Header); len(values) == 1 {
		candidate = values[0]
	}
	ctx, err := requestcontext.New(r.Context(), candidate)
	if err != nil {
		httperrors.Write(w, apperrors.ErrUnavailable)
		c.Abort()
		return
	}
	// Extract only W3C tracing headers; no baggage, auth or arbitrary client metadata.
	carrier := propagation.HeaderCarrier(http.Header{})
	if values := r.Header.Values("traceparent"); len(values) == 1 && len(values[0]) <= 512 {
		carrier.Set("traceparent", values[0])
		if states := r.Header.Values("tracestate"); len(states) == 1 && len(states[0]) <= 512 {
			carrier.Set("tracestate", states[0])
		}
	}
	ctx = propagation.TraceContext{}.Extract(ctx, carrier)
	route := c.FullPath()
	if route == "" {
		route = "/unmatched"
	}
	ctx, span := otel.Tracer("user-api/http").Start(ctx, r.Method+" "+route, trace.WithSpanKind(trace.SpanKindServer), trace.WithAttributes(attribute.String("http.request.method", r.Method), attribute.String("http.route", route)))
	defer span.End()
	requestLog := logger.WithContext(ctx, h.log)
	w.Header().Set(requestcontext.Header, requestcontext.ID(ctx))
	if sc := span.SpanContext(); sc.IsValid() {
		w.Header().Set("X-Trace-ID", sc.TraceID().String())
		propagation.TraceContext{}.Inject(ctx, propagation.HeaderCarrier(w.Header()))
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	defer func() {
		if recovered := recover(); recovered != nil {
			requestLog.ErrorContext(ctx, "HTTP panic", "method", r.Method, "path", r.URL.Path)
			if !w.Written() {
				httpresponse.Failure(w, http.StatusInternalServerError, "internal_error", "internal server error")
			}
			c.Abort()
		}
		span.SetAttributes(attribute.Int("http.response.status_code", w.Status()))
		if w.Status() >= 500 {
			span.SetStatus(otelcodes.Error, "HTTP request failed")
		}
		if !healthcheck.IsHTTP(r.URL.Path) {
			requestLog.Log(ctx, logger.HTTPLevel(w.Status()), "HTTP request", "method", r.Method, "path", r.URL.Path, "status", w.Status(), "duration_ms", time.Since(started).Milliseconds())
		}
	}()
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	c.Request = r.WithContext(ctx)
	c.Next()
}

func (h *Handler) register(c *gin.Context) {
	w, r := c.Writer, c.Request
	var in registerJSON
	if err := h.decode(w, r, &in); err != nil {
		httperrors.Write(w, err)
		return
	}
	result, err := h.app.Register(r.Context(), in.value())
	if err != nil {
		httperrors.Write(w, err)
		return
	}
	w.Header().Set("Location", "/api/v1/users/"+result.User.ID)
	httpresponse.Success(w, http.StatusCreated, struct {
		User   userJSON   `json:"user"`
		Tokens tokensJSON `json:"tokens"`
	}{publicUser(result.User), publicTokens(result.Tokens)})
}

func (h *Handler) login(c *gin.Context) {
	w, r := c.Writer, c.Request
	var in loginJSON
	if err := h.decode(w, r, &in); err != nil {
		httperrors.Write(w, err)
		return
	}
	v, err := h.app.Login(r.Context(), domain.Login{
		Email:    in.Email,
		Password: in.Password,
	})
	if err != nil {
		httperrors.Write(w, err)
		return
	}
	w.Header().Set("Location", "/api/v1/sessions/current")
	httpresponse.Success(w, http.StatusCreated, publicTokens(v))
}

func (h *Handler) refresh(c *gin.Context) {
	w, r := c.Writer, c.Request
	var in struct {
		RefreshToken string `json:"refresh_token" validate:"required,maxbytes=256"`
	}
	if err := h.decode(w, r, &in); err != nil {
		httperrors.Write(w, err)
		return
	}
	v, err := h.app.Refresh(r.Context(), in.RefreshToken)
	if err != nil {
		httperrors.Write(w, err)
		return
	}
	httpresponse.Success(w, http.StatusOK, publicTokens(v))
}

func (h *Handler) logout(c *gin.Context) {
	w, r := c.Writer, c.Request
	raw, err := middleware.Bearer(r)
	if err == nil {
		err = h.app.Logout(r.Context(), raw)
	}
	if err != nil {
		httperrors.Write(w, err)
		return
	}
	httpresponse.NoContent(w)
}

func (h *Handler) get(c *gin.Context) {
	w, r := c.Writer, c.Request
	if !validUserID(h.validate, c.Param("id")) {
		httperrors.Write(w, apperrors.ErrInvalid)
		return
	}
	u, err := h.app.Get(r.Context(), middleware.User(r.Context()), c.Param("id"))
	if err != nil {
		httperrors.Write(w, err)
		return
	}
	httpresponse.Success(w, http.StatusOK, publicUser(u))
}

func (h *Handler) list(c *gin.Context) {
	w, r := c.Writer, c.Request
	limit := 20
	if raw := r.URL.Query().Get("limit"); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil || v < 1 || v > 100 {
			httperrors.Write(w, apperrors.ErrInvalid)
			return
		}
		limit = v
	}
	after := r.URL.Query().Get("after")
	if after != "" && !validUserID(h.validate, after) {
		httperrors.Write(w, apperrors.ErrInvalid)
		return
	}
	v, err := h.app.List(r.Context(), middleware.User(r.Context()), limit, after)
	if err != nil {
		httperrors.Write(w, err)
		return
	}
	httpresponse.Success(w, http.StatusOK, publicPage(v))
}

func (h *Handler) update(c *gin.Context) {
	w, r := c.Writer, c.Request
	if !validUserID(h.validate, c.Param("id")) {
		httperrors.Write(w, apperrors.ErrInvalid)
		return
	}
	var in patchJSON
	if err := h.decode(w, r, &in); err != nil {
		httperrors.Write(w, err)
		return
	}
	u, err := h.app.Update(r.Context(), middleware.User(r.Context()), c.Param("id"), in.value())
	if err != nil {
		httperrors.Write(w, err)
		return
	}
	httpresponse.Success(w, http.StatusOK, publicUser(u))
}

func (h *Handler) delete(c *gin.Context) {
	w, r := c.Writer, c.Request
	if !validUserID(h.validate, c.Param("id")) {
		httperrors.Write(w, apperrors.ErrInvalid)
		return
	}
	raw, err := middleware.Bearer(r)
	if err == nil {
		err = h.app.Delete(r.Context(), raw, c.Param("id"))
	}
	if err != nil {
		httperrors.Write(w, err)
		return
	}
	httpresponse.NoContent(w)
}

func (h *Handler) profile(c *gin.Context) {
	w, r := c.Writer, c.Request
	p := middleware.User(r.Context())
	u, err := h.app.Get(r.Context(), p, p.UserID)
	if err != nil {
		httperrors.Write(w, err)
		return
	}
	httpresponse.Success(w, http.StatusOK, publicUser(u))
}

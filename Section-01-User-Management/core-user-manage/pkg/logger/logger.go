package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"go.opentelemetry.io/otel/trace"

	"github.com/nutsp/7solutions-backend-challenge/core-user-manage/pkg/requestcontext"
)

type Config struct {
	Service   string    `yaml:"service"`
	Level     string    `yaml:"level"`
	Format    string    `yaml:"format"`
	AddSource bool      `yaml:"add_source"`
	Output    io.Writer `yaml:"-" json:"-"`
}

func New(cfg Config) (*slog.Logger, error) {
	if cfg.Level == "" {
		cfg.Level = "info"
	}
	var level slog.Level
	if err := level.UnmarshalText([]byte(strings.ToUpper(cfg.Level))); err != nil {
		return nil, fmt.Errorf("invalid log level")
	}
	if cfg.Format == "" {
		cfg.Format = "json"
	}
	output := cfg.Output
	if output == nil {
		output = os.Stdout
	}
	options := &slog.HandlerOptions{
		Level:     level,
		AddSource: cfg.AddSource,
		ReplaceAttr: func(_ []string, attr slog.Attr) slog.Attr {
			if attr.Key == slog.TimeKey && attr.Value.Kind() == slog.KindTime {
				return slog.Time(slog.TimeKey, attr.Value.Time().UTC())
			}
			switch strings.NewReplacer("-", "", "_", "").Replace(strings.ToLower(attr.Key)) {
			case "password", "passwordhash", "authorization", "cookie", "setcookie", "accesstoken", "refreshtoken", "servicetoken", "xservicetoken", "jwtsecret", "secret", "token", "apikey", "xapikey", "keys", "key", "userdatakeys", "userauditkeys", "emaillookupkey":
				return slog.String(attr.Key, "[REDACTED]")
			}
			return attr
		},
	}
	var handler slog.Handler
	switch cfg.Format {
	case "json":
		handler = slog.NewJSONHandler(output, options)
	case "text":
		handler = slog.NewTextHandler(output, options)
	default:
		return nil, fmt.Errorf("unsupported log format")
	}
	return slog.New(handler).With("service", cfg.Service), nil
}

func DefaultConfig() Config {

	return Config{
		Level:  "info",
		Format: "json",
	}
}

// WithContext adds correlation values even when the caller supplies a plain slog logger.
// Logs created through this logger must never include request bodies or auth headers.
func WithContext(ctx context.Context, log *slog.Logger) *slog.Logger {
	if log == nil {
		log = slog.Default()
	}
	attrs := []any{}
	if id := requestcontext.ID(ctx); id != "" {
		attrs = append(attrs, "request_id", id)
	}
	if span := trace.SpanContextFromContext(ctx); span.IsValid() {
		attrs = append(attrs, "trace_id", span.TraceID().String(), "span_id", span.SpanID().String())
	}
	return log.With(attrs...)
}

func HTTPLevel(status int) slog.Level {
	if status >= 500 {
		return slog.LevelError
	}
	if status >= 400 {
		return slog.LevelWarn
	}
	return slog.LevelInfo
}

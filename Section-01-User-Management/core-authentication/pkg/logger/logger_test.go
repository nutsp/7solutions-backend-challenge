package logger

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/trace"

	"github.com/nutsp/7solutions-backend-challenge/core-authentication/pkg/requestcontext"
)

func TestConfigControlsLevelAndOutput(t *testing.T) {
	tests := []struct {
		name   string
		format string
	}{{"JSON", "json"}, {"text", "text"}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			log, err := New(Config{
				Service: "configured-service",
				Level:   "warn",
				Format:  tt.format,
				Output:  &out,
			})
			if err != nil {
				t.Fatal(err)
			}
			log.Info("filtered")
			log.Warn("visible")
			if strings.Contains(out.String(), "filtered") || !strings.Contains(out.String(), "configured-service") || !strings.Contains(out.String(), "visible") {
				t.Fatal("logger config was not applied")
			}

		})
	}
}

func TestInvalidLoggingConfig(t *testing.T) {
	tests := []struct {
		name   string
		config Config
	}{
		{"unknown level", Config{
			Level: "unknown",
		}}, {"unknown format", Config{
			Format: "unknown",
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := New(tt.config); err == nil {
				t.Fatal("invalid log settings accepted")
			}
		})
	}
}

func TestCorrelationAndRedaction(t *testing.T) {
	tests := []struct {
		name   string
		format string
	}{{"JSON", "json"}, {"text", "text"}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			log, err := New(Config{
				Service: "auth",
				Format:  tt.format,
				Output:  &out,
			})
			if err != nil {
				t.Fatal(err)
			}
			ctx, err := requestcontext.New(context.Background(), "request-123")
			if err != nil {
				t.Fatal(err)
			}
			traceID, _ := trace.TraceIDFromHex("0123456789abcdef0123456789abcdef")
			spanID, _ := trace.SpanIDFromHex("0123456789abcdef")
			ctx = trace.ContextWithSpanContext(ctx, trace.NewSpanContext(trace.SpanContextConfig{
				TraceID: traceID,
				SpanID:  spanID,
			}))
			WithContext(ctx, log).With("service_token", "private-service-token").Info("event", "authorization", "Bearer private-access-token", "refresh_token", "private-refresh-token", slog.Group("credentials", slog.String("password", "private-password")))
			raw := out.String()
			for _, want := range []string{"request-123", traceID.String(), spanID.String(), "[REDACTED]"} {
				if !strings.Contains(raw, want) {
					t.Fatalf("missing correlation/redaction field %s", want)
				}
			}
			for _, secret := range []string{"private-service-token", "private-access-token", "private-refresh-token", "private-password"} {
				if strings.Contains(raw, secret) {
					t.Fatal("secret exposed in logs")
				}
			}
		})
	}
}

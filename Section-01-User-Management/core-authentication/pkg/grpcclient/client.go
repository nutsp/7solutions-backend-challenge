// Package grpcclient configures TLS, request deadlines and trusted outgoing service metadata.
package grpcclient

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"os"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/nutsp/7solutions-backend-challenge/core-authentication/pkg/requestcontext"
)

type Config struct {
	Address        string        `yaml:"address"`
	ServiceName    string        `yaml:"service_name"`
	ServiceToken   string        `yaml:"service_token" json:"-"`
	CAFile         string        `yaml:"ca_file"`
	ServerName     string        `yaml:"server_name"`
	AllowInsecure  bool          `yaml:"allow_insecure"`
	RequestTimeout time.Duration `yaml:"request_timeout"`
	HealthTimeout  time.Duration `yaml:"health_timeout"`
}

type Principal struct {
	UserID string
	Caller string
}

type principalKey struct{}

func DefaultConfig() Config {
	return Config{
		RequestTimeout: 5 * time.Second,
		HealthTimeout:  2 * time.Second,
	}
}

func Actor(ctx context.Context) Principal {

	p, _ := ctx.Value(principalKey{}).(Principal)
	return p
}

func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p)
}

func Dial(cfg Config) (*grpc.ClientConn, error) {
	if cfg.Address == "" {
		return nil, fmt.Errorf("gRPC address is required")
	}
	if cfg.RequestTimeout == 0 {
		cfg.RequestTimeout = DefaultConfig().RequestTimeout
	}
	if cfg.RequestTimeout < 0 || cfg.HealthTimeout < 0 {
		return nil, fmt.Errorf("gRPC timeouts must not be negative")
	}

	var transport credentials.TransportCredentials
	caFile := cfg.CAFile
	if caFile != "" {
		pem, err := os.ReadFile(caFile)
		if err != nil {
			return nil, err
		}
		roots := x509.NewCertPool()
		if !roots.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("invalid gRPC CA file")
		}
		transport = credentials.NewTLS(&tls.Config{
			RootCAs:    roots,
			MinVersion: tls.VersionTLS12,
			ServerName: cfg.ServerName,
		})
	} else if cfg.AllowInsecure {
		transport = insecure.NewCredentials()
	} else {
		return nil, fmt.Errorf("gRPC CA file is required unless insecure transport is explicitly enabled")
	}
	return grpc.NewClient(cfg.Address, grpc.WithTransportCredentials(transport), grpc.WithStatsHandler(otelgrpc.NewClientHandler()), grpc.WithUnaryInterceptor(func(ctx context.Context, method string, req, reply any, conn *grpc.ClientConn, invoke grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		// Replace outgoing metadata so untrusted inbound headers cannot become service identity.
		p := Actor(ctx)
		md := metadata.Pairs("x-service-name", cfg.ServiceName, "x-service-token", cfg.ServiceToken)
		if id := requestcontext.ID(ctx); id != "" {
			md.Set(requestcontext.MetadataKey, id)
		}
		if p.UserID != "" {
			md.Set("x-user-id", p.UserID)
		}
		ctx, cancel := context.WithTimeout(metadata.NewOutgoingContext(ctx, md), cfg.RequestTimeout)
		defer cancel()
		return invoke(ctx, method, req, reply, conn, opts...)
	}))
}

func WithActor(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, principalKey{}, Principal{
		UserID: id,
	})
}

func Healthcheck(cfg Config) error {
	address := cfg.Address
	if cfg.HealthTimeout == 0 {
		cfg.HealthTimeout = DefaultConfig().HealthTimeout
	}
	if cfg.HealthTimeout < 0 {
		return fmt.Errorf("health timeout must be positive")
	}

	// Checks the same TLS policy as ordinary clients.
	if host, port, err := net.SplitHostPort(address); err == nil && (host == "" || host == "0.0.0.0") {
		address = net.JoinHostPort("127.0.0.1", port)
	}
	cfg.Address = address
	cfg.ServiceName = "healthcheck"
	cfg.ServiceToken = ""
	conn, err := Dial(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), cfg.HealthTimeout)
	defer cancel()
	return CheckHealth(ctx, healthpb.NewHealthClient(conn))
}

func CheckHealth(ctx context.Context, client healthpb.HealthClient) error {
	response, err := client.Check(ctx, &healthpb.HealthCheckRequest{})
	if err != nil {
		return err
	}
	if response.Status != healthpb.HealthCheckResponse_SERVING {
		return status.Error(codes.Unavailable, "service is not ready")
	}
	return nil
}

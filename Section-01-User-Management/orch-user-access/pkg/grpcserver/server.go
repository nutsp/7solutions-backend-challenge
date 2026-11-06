// Package grpcserver configures bounded gRPC serving, health probes, worker lifetime and graceful shutdown.
package grpcserver

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/nutsp/7solutions-backend-challenge/orch-user-access/pkg/healthcheck"
	"github.com/nutsp/7solutions-backend-challenge/orch-user-access/pkg/logger"
	"github.com/nutsp/7solutions-backend-challenge/orch-user-access/pkg/requestcontext"
)

type Config struct {
	Address              string        `yaml:"address"`
	CertFile             string        `yaml:"cert_file"`
	KeyFile              string        `yaml:"key_file"`
	AllowInsecure        bool          `yaml:"allow_insecure"`
	RequestTimeout       time.Duration `yaml:"request_timeout"`
	ShutdownTimeout      time.Duration `yaml:"shutdown_timeout"`
	ProbeInterval        time.Duration `yaml:"probe_interval"`
	ProbeTimeout         time.Duration `yaml:"probe_timeout"`
	MaxReceiveBytes      int           `yaml:"max_receive_bytes"`
	MaxHeaderBytes       uint32        `yaml:"max_header_bytes"`
	MaxConcurrentStreams uint32        `yaml:"max_concurrent_streams"`
}

func New(cfg Config, log *slog.Logger, interceptors ...grpc.UnaryServerInterceptor) (*grpc.Server, error) {
	var err error
	cfg, err = cfg.normalized()
	if err != nil {
		return nil, err
	}
	interceptor := func(ctx context.Context, req any, info *grpc.UnaryServerInfo, next grpc.UnaryHandler) (response any, err error) {
		started := time.Now()
		candidate := ""
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			if values := md.Get(requestcontext.MetadataKey); len(values) == 1 {
				candidate = values[0]
			}
		}
		ctx, err = requestcontext.New(ctx, candidate)
		if err != nil {
			return nil, status.Error(codes.Internal, "request context unavailable")
		}
		requestLog := logger.WithContext(ctx, log)
		headers := metadata.Pairs(requestcontext.MetadataKey, requestcontext.ID(ctx))
		if span := trace.SpanContextFromContext(ctx); span.IsValid() {
			headers.Set("x-trace-id", span.TraceID().String())
		}
		_ = grpc.SetHeader(ctx, headers)
		defer func() {
			if recovered := recover(); recovered != nil {
				requestLog.ErrorContext(ctx, "RPC panic", "method", info.FullMethod)
				response, err = nil, status.Error(codes.Internal, "internal server error")
			}
			level := slog.LevelInfo
			switch status.Code(err) {
			case codes.OK:
			case codes.InvalidArgument, codes.NotFound, codes.AlreadyExists, codes.Unauthenticated, codes.PermissionDenied, codes.ResourceExhausted, codes.Canceled, codes.FailedPrecondition, codes.OutOfRange:
				level = slog.LevelWarn
			default:
				level = slog.LevelError
			}
			if !healthcheck.IsGRPC(info.FullMethod) {
				requestLog.Log(ctx, level, "RPC", "method", info.FullMethod, "code", status.Code(err).String(), "duration_ms", time.Since(started).Milliseconds())
			}
		}()
		ctx, cancel := context.WithTimeout(ctx, cfg.RequestTimeout)
		defer cancel()
		return next(ctx, req)
	}
	options := []grpc.ServerOption{
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
		grpc.ChainUnaryInterceptor(append([]grpc.UnaryServerInterceptor{interceptor}, interceptors...)...),
		grpc.MaxRecvMsgSize(cfg.MaxReceiveBytes),
		grpc.MaxHeaderListSize(cfg.MaxHeaderBytes),
		grpc.MaxConcurrentStreams(cfg.MaxConcurrentStreams),
		grpc.WaitForHandlers(true),
	}
	certFile, keyFile := cfg.CertFile, cfg.KeyFile
	if certFile != "" || keyFile != "" {
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return nil, err
		}
		options = append(options, grpc.Creds(credentials.NewTLS(&tls.Config{
			MinVersion:   tls.VersionTLS12,
			Certificates: []tls.Certificate{cert},
		})))
	} else if !cfg.AllowInsecure {
		return nil, fmt.Errorf("gRPC TLS is required unless insecure transport is explicitly enabled")
	}
	return grpc.NewServer(options...), nil
}

func DefaultConfig() Config {
	return Config{
		RequestTimeout:       5 * time.Second,
		ShutdownTimeout:      10 * time.Second,
		ProbeInterval:        2 * time.Second,
		ProbeTimeout:         time.Second,
		MaxReceiveBytes:      32 << 10,
		MaxHeaderBytes:       16 << 10,
		MaxConcurrentStreams: 100,
	}
}

func Run(ctx context.Context, cfg Config, server *grpc.Server, log *slog.Logger, probe func(context.Context) error, workers ...func(context.Context)) error {
	var err error
	cfg, err = cfg.normalized()
	if err != nil {
		return err
	}
	if cfg.Address == "" {
		return fmt.Errorf("gRPC listen address is required")
	}
	listener, err := net.Listen("tcp", cfg.Address)
	if err != nil {
		return err
	}
	defer listener.Close()
	h := health.NewServer()
	healthpb.RegisterHealthServer(server, h)
	h.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	workerCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	var wg sync.WaitGroup
	if probe != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ticker := time.NewTicker(cfg.ProbeInterval)
			defer ticker.Stop()
			for {
				bounded, cancel := context.WithTimeout(workerCtx, cfg.ProbeTimeout)
				err := probe(bounded)
				cancel()
				if workerCtx.Err() != nil {
					return
				}
				state := healthpb.HealthCheckResponse_SERVING
				if err != nil {
					state = healthpb.HealthCheckResponse_NOT_SERVING
				}
				h.SetServingStatus("", state)
				select {
				case <-workerCtx.Done():
					return
				case <-ticker.C:
				}
			}
		}()
	}

	for _, worker := range workers {
		wg.Add(1)
		go func(fn func(context.Context)) {

			defer wg.Done()
			fn(workerCtx)
		}(worker)
	}
	failures := make(chan error, 1)
	go func() {

		failures <- server.Serve(listener)
	}()
	log.Info("gRPC listening", "address", listener.Addr().String())
	var serveErr error
	received := false
	select {
	case <-ctx.Done():
	case serveErr = <-failures:
		received = true
	}
	h.Shutdown()
	cancel()
	stopped := make(chan struct{})
	go func() {

		server.GracefulStop()
		close(stopped)
	}()
	timer := time.NewTimer(cfg.ShutdownTimeout)
	defer timer.Stop()
	select {
	case <-stopped:
	case <-timer.C:
		log.Warn("forcing gRPC shutdown")
		server.Stop()
		<-stopped
	}
	wg.Wait()
	if !received {
		serveErr = <-failures
	}
	if errors.Is(serveErr, grpc.ErrServerStopped) {
		return nil
	}
	return serveErr
}

func (c Config) normalized() (Config, error) {
	d := DefaultConfig()
	if c.RequestTimeout == 0 {
		c.RequestTimeout = d.RequestTimeout
	}
	if c.ShutdownTimeout == 0 {
		c.ShutdownTimeout = d.ShutdownTimeout
	}
	if c.ProbeInterval == 0 {
		c.ProbeInterval = d.ProbeInterval
	}
	if c.ProbeTimeout == 0 {
		c.ProbeTimeout = d.ProbeTimeout
	}
	if c.MaxReceiveBytes == 0 {
		c.MaxReceiveBytes = d.MaxReceiveBytes
	}
	if c.MaxHeaderBytes == 0 {
		c.MaxHeaderBytes = d.MaxHeaderBytes
	}
	if c.MaxConcurrentStreams == 0 {
		c.MaxConcurrentStreams = d.MaxConcurrentStreams
	}
	if c.RequestTimeout < 0 || c.ShutdownTimeout < 0 || c.ProbeInterval < 0 || c.ProbeTimeout < 0 || c.MaxReceiveBytes < 0 {
		return c, fmt.Errorf("invalid gRPC server limits")
	}
	return c, nil
}

// Package httpserver owns HTTP timeouts, listener lifetime and graceful request draining.
package httpserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"
)

type Config struct {
	Address           string        `yaml:"address"`
	ReadHeaderTimeout time.Duration `yaml:"read_header_timeout"`
	ReadTimeout       time.Duration `yaml:"read_timeout"`
	WriteTimeout      time.Duration `yaml:"write_timeout"`
	IdleTimeout       time.Duration `yaml:"idle_timeout"`
	ShutdownTimeout   time.Duration `yaml:"shutdown_timeout"`
	MaxHeaderBytes    int           `yaml:"max_header_bytes"`
}

func DefaultConfig() Config {
	return Config{
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
		ShutdownTimeout:   10 * time.Second,
		MaxHeaderBytes:    16 << 10,
	}
}

func Run(ctx context.Context, cfg Config, handler http.Handler, log *slog.Logger) error {
	var err error
	cfg, err = cfg.normalized()
	if err != nil {
		return err
	}
	if cfg.Address == "" {
		return fmt.Errorf("HTTP listen address is required")
	}
	listener, err := net.Listen("tcp", cfg.Address)
	if err != nil {
		return err
	}
	defer listener.Close()
	return serve(ctx, cfg, listener, handler, log)
}

func (c Config) normalized() (Config, error) {
	d := DefaultConfig()
	if c.ReadHeaderTimeout == 0 {
		c.ReadHeaderTimeout = d.ReadHeaderTimeout
	}
	if c.ReadTimeout == 0 {
		c.ReadTimeout = d.ReadTimeout
	}
	if c.WriteTimeout == 0 {
		c.WriteTimeout = d.WriteTimeout
	}
	if c.IdleTimeout == 0 {
		c.IdleTimeout = d.IdleTimeout
	}
	if c.ShutdownTimeout == 0 {
		c.ShutdownTimeout = d.ShutdownTimeout
	}
	if c.MaxHeaderBytes == 0 {
		c.MaxHeaderBytes = d.MaxHeaderBytes
	}
	if c.ReadHeaderTimeout < 0 || c.ReadTimeout < 0 || c.WriteTimeout < 0 || c.IdleTimeout < 0 || c.ShutdownTimeout < 0 || c.MaxHeaderBytes < 0 {
		return c, fmt.Errorf("invalid HTTP server limits")
	}
	return c, nil
}

func serve(ctx context.Context, cfg Config, listener net.Listener, handler http.Handler, log *slog.Logger) error {
	var err error
	cfg, err = cfg.normalized()
	if err != nil {
		return err
	}
	requestCtx, cancelRequests := context.WithCancel(context.Background())
	defer cancelRequests()
	server := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
		MaxHeaderBytes:    cfg.MaxHeaderBytes,
		BaseContext: func(net.Listener) context.Context {

			return requestCtx
		},
	}
	failures := make(chan error, 1)
	go func() {

		failures <- server.Serve(listener)
	}()
	log.Info("HTTP listening", "address", listener.Addr().String())
	received := false
	var serveErr error
	select {
	case <-ctx.Done():
	case serveErr = <-failures:
		received = true
	}
	shutdown, cancelShutdown := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancelShutdown()
	if err := server.Shutdown(shutdown); err != nil {
		log.Warn("forcing HTTP shutdown", "error", err)
		cancelRequests()
		_ = server.Close()
	}
	cancelRequests()
	if !received {
		serveErr = <-failures
	}
	if errors.Is(serveErr, http.ErrServerClosed) {
		return nil
	}
	return serveErr
}

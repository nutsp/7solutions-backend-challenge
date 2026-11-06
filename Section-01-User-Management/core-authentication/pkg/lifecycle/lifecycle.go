// Package lifecycle handles operating-system cancellation and bounded resource cleanup.
package lifecycle

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type Config struct {
	CleanupTimeout time.Duration `yaml:"cleanup_timeout"`
	Signals        []os.Signal   `yaml:"-" json:"-"`
}

func SignalContext(cfg Config) (context.Context, context.CancelFunc) {
	signals := cfg.Signals
	if len(signals) == 0 {
		signals = []os.Signal{os.Interrupt, syscall.SIGTERM}
	}
	return signal.NotifyContext(context.Background(), signals...)
}

func Cleanup(cfg Config, work func(context.Context) error, log *slog.Logger) {
	timeout := cfg.CleanupTimeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := work(ctx); err != nil {
		log.Error("cleanup failed", "error", err)
	}
}

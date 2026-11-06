package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/gin-gonic/gin"

	coreauthadapter "github.com/nutsp/7solutions-backend-challenge/orch-user-access/internal/adapter/core-authentication"
	coreuseradapter "github.com/nutsp/7solutions-backend-challenge/orch-user-access/internal/adapter/core-user-manage"
	"github.com/nutsp/7solutions-backend-challenge/orch-user-access/internal/appconfig"
	httphandler "github.com/nutsp/7solutions-backend-challenge/orch-user-access/internal/handlers/http"
	services "github.com/nutsp/7solutions-backend-challenge/orch-user-access/internal/services"
	"github.com/nutsp/7solutions-backend-challenge/orch-user-access/pkg/grpcclient"
	"github.com/nutsp/7solutions-backend-challenge/orch-user-access/pkg/httpserver"
	"github.com/nutsp/7solutions-backend-challenge/orch-user-access/pkg/lifecycle"
	"github.com/nutsp/7solutions-backend-challenge/orch-user-access/pkg/logger"
	"github.com/nutsp/7solutions-backend-challenge/orch-user-access/pkg/tracing"
)

func main() {
	gin.SetMode(gin.ReleaseMode)
	log := slog.Default()
	cfg, err := appconfig.Load()
	if err != nil {
		log.Error("configuration failed", "error", err)
		os.Exit(1)
	}
	log, err = logger.New(cfg.Logger)
	if err != nil {
		slog.Error("logger configuration failed", "error", err)
		os.Exit(1)
	}

	ctx, stop := lifecycle.SignalContext(cfg.Lifecycle)
	defer stop()

	if err := run(ctx, cfg, log); err != nil {
		log.Error("gateway stopped", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, cfg appconfig.Config, log *slog.Logger) error {
	provider, err := tracing.Setup(tracing.Config{
		ServiceName: cfg.Logger.Service,
	})
	if err != nil {
		return err
	}
	defer lifecycle.Cleanup(cfg.Lifecycle, provider.Shutdown, log)

	identity, err := grpcclient.Dial(cfg.CoreUser)
	if err != nil {
		return err
	}
	defer identity.Close()

	auth, err := grpcclient.Dial(cfg.CoreAuth)
	if err != nil {
		return err
	}
	defer auth.Close()
	handler := httphandler.New(services.New(coreuseradapter.New(identity), coreauthadapter.New(auth)), log)
	return httpserver.Run(ctx, cfg.HTTP, handler, log)
}

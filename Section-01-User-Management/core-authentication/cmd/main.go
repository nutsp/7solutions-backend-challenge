package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	coreuseradapter "github.com/nutsp/7solutions-backend-challenge/core-authentication/internal/adapter/core-user-manage"
	securityadapter "github.com/nutsp/7solutions-backend-challenge/core-authentication/internal/adapter/security"
	"github.com/nutsp/7solutions-backend-challenge/core-authentication/internal/appconfig"
	grpchandler "github.com/nutsp/7solutions-backend-challenge/core-authentication/internal/handlers/grpc"
	redisrepository "github.com/nutsp/7solutions-backend-challenge/core-authentication/internal/repository/redis"
	services "github.com/nutsp/7solutions-backend-challenge/core-authentication/internal/services"
	"github.com/nutsp/7solutions-backend-challenge/core-authentication/pkg/grpcclient"
	"github.com/nutsp/7solutions-backend-challenge/core-authentication/pkg/grpcserver"
	"github.com/nutsp/7solutions-backend-challenge/core-authentication/pkg/hash"
	"github.com/nutsp/7solutions-backend-challenge/core-authentication/pkg/jwt"
	"github.com/nutsp/7solutions-backend-challenge/core-authentication/pkg/lifecycle"
	"github.com/nutsp/7solutions-backend-challenge/core-authentication/pkg/logger"
	authpb "github.com/nutsp/7solutions-backend-challenge/core-authentication/pkg/proto/core-authentication"
	pb "github.com/nutsp/7solutions-backend-challenge/core-authentication/pkg/proto/core-user-manage"
	"github.com/nutsp/7solutions-backend-challenge/core-authentication/pkg/redisclient"
	"github.com/nutsp/7solutions-backend-challenge/core-authentication/pkg/tracing"
)

func main() {
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

	if len(os.Args) > 1 && os.Args[1] == "--healthcheck" {
		if err := grpcclient.Healthcheck(cfg.Healthcheck); err != nil {
			log.Error("healthcheck", "error", err)
			os.Exit(1)
		}
		return
	}
	ctx, stop := lifecycle.SignalContext(cfg.Lifecycle)
	defer stop()
	if err := run(ctx, cfg, log); err != nil {
		log.Error("service stopped", "error", err)
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

	crypto, err := securityadapter.New(hash.Config{
		Cost: 10,
	}, jwt.Config{
		Secret:   cfg.JWTSecret,
		Issuer:   cfg.JWTIssuer,
		Audience: cfg.JWTAudience,
		TTL:      time.Duration(cfg.AccessTokenSeconds) * time.Second,
	})
	if err != nil {
		return err
	}
	cache, err := redisclient.New(ctx, cfg.Redis)
	if err != nil {
		return err
	}
	defer cache.Close()

	conn, err := grpcclient.Dial(cfg.CoreUser)
	if err != nil {
		return err
	}
	defer conn.Close()
	auth, err := services.New(redisrepository.New(cache), coreuseradapter.New(pb.NewIdentityServiceClient(conn)), crypto)
	if err != nil {
		return err
	}
	server, err := grpcserver.New(cfg.GRPCServer, log)
	if err != nil {
		return err
	}
	authpb.RegisterAuthenticationServiceServer(server, grpchandler.New(auth, log))
	return grpcserver.Run(ctx, cfg.GRPCServer, server, log, func(ctx context.Context) error {

		return cache.Ping(ctx).Err()
	})
}

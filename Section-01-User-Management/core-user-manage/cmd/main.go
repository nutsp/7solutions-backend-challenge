package main

import (
	"context"
	"flag"
	"log/slog"
	"os"

	"github.com/nutsp/7solutions-backend-challenge/core-user-manage/internal/appconfig"
	grpchandler "github.com/nutsp/7solutions-backend-challenge/core-user-manage/internal/handlers/grpc"
	mongorepository "github.com/nutsp/7solutions-backend-challenge/core-user-manage/internal/repository/mongodb"
	services "github.com/nutsp/7solutions-backend-challenge/core-user-manage/internal/services"
	"github.com/nutsp/7solutions-backend-challenge/core-user-manage/internal/worker"
	"github.com/nutsp/7solutions-backend-challenge/core-user-manage/pkg/grpcclient"
	"github.com/nutsp/7solutions-backend-challenge/core-user-manage/pkg/grpcserver"
	"github.com/nutsp/7solutions-backend-challenge/core-user-manage/pkg/hash"
	"github.com/nutsp/7solutions-backend-challenge/core-user-manage/pkg/lifecycle"
	"github.com/nutsp/7solutions-backend-challenge/core-user-manage/pkg/logger"
	"github.com/nutsp/7solutions-backend-challenge/core-user-manage/pkg/mongoclient"
	pb "github.com/nutsp/7solutions-backend-challenge/core-user-manage/pkg/proto/core-user-manage"
	"github.com/nutsp/7solutions-backend-challenge/core-user-manage/pkg/tracing"
)

func main() {
	healthcheck := flag.Bool("healthcheck", false, "Check service health and exit")
	flag.Parse()

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

	if *healthcheck {
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

	passwords, err := hash.New(hash.Config{
		Cost: 10,
	})
	if err != nil {
		return err
	}
	client, err := mongoclient.New(ctx, cfg.Mongo)
	if err != nil {
		return err
	}
	defer lifecycle.Cleanup(cfg.Lifecycle, client.Disconnect, log)
	userCipher, auditCipher, lookup, err := protection(cfg)
	if err != nil {
		return err
	}
	repo := mongorepository.New(client.Database(cfg.Mongo.Database), userCipher, lookup)
	users := services.New(repo, passwords, mongorepository.NewAudits(client.Database(cfg.Mongo.Database), auditCipher), mongorepository.NewTransactions(client))
	server, err := grpcserver.New(cfg.GRPCServer, log)
	if err != nil {
		return err
	}
	pb.RegisterIdentityServiceServer(server, grpchandler.New(users, log))
	return grpcserver.Run(ctx, cfg.GRPCServer, server, log, func(ctx context.Context) error {

		return client.Ping(ctx, nil)
	}, func(ctx context.Context) {

		worker.Run(ctx, users, log)
	})
}

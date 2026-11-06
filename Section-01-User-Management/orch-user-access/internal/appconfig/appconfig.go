package appconfig

import (
	"fmt"
	"time"

	"github.com/nutsp/7solutions-backend-challenge/orch-user-access/pkg/config"
	"github.com/nutsp/7solutions-backend-challenge/orch-user-access/pkg/grpcclient"
	"github.com/nutsp/7solutions-backend-challenge/orch-user-access/pkg/httpserver"
	"github.com/nutsp/7solutions-backend-challenge/orch-user-access/pkg/lifecycle"
	"github.com/nutsp/7solutions-backend-challenge/orch-user-access/pkg/logger"
)

type Config struct {
	Logger    logger.Config     `yaml:"logger"`
	Lifecycle lifecycle.Config  `yaml:"lifecycle"`
	HTTP      httpserver.Config `yaml:"http"`
	CoreUser  grpcclient.Config `yaml:"core_user_grpc"`
	CoreAuth  grpcclient.Config `yaml:"core_auth_grpc"`
}

func Load() (Config, error) {
	cfg := Config{
		Logger: logger.DefaultConfig(),
		Lifecycle: lifecycle.Config{
			CleanupTimeout: 5 * time.Second,
		},
	}
	cfg.Logger.Service = "orch-user-access"
	cfg.HTTP = httpserver.DefaultConfig()
	cfg.HTTP.Address = "127.0.0.1:8080"
	cfg.CoreAuth = grpcclient.DefaultConfig()
	cfg.CoreAuth.Address = "localhost:9092"
	cfg.CoreAuth.ServiceName = "orch"
	cfg.CoreUser = grpcclient.DefaultConfig()
	cfg.CoreUser.Address = "localhost:9091"
	cfg.CoreUser.ServiceName = "orch"
	if err := config.Load(config.Config{
		Path: config.Path(),
	}, &cfg); err != nil {
		return cfg, err
	}

	if cfg.HTTP.MaxHeaderBytes < 1 || cfg.HTTP.MaxHeaderBytes > 16777216 {
		return cfg, fmt.Errorf("configuration integer outside allowed range")
	}
	return cfg, nil
}

package appconfig

import (
	"fmt"
	"time"

	"github.com/nutsp/7solutions-backend-challenge/core-authentication/pkg/config"
	"github.com/nutsp/7solutions-backend-challenge/core-authentication/pkg/grpcclient"
	"github.com/nutsp/7solutions-backend-challenge/core-authentication/pkg/grpcserver"
	"github.com/nutsp/7solutions-backend-challenge/core-authentication/pkg/lifecycle"
	"github.com/nutsp/7solutions-backend-challenge/core-authentication/pkg/logger"
	"github.com/nutsp/7solutions-backend-challenge/core-authentication/pkg/redisclient"
)

type Config struct {
	Logger             logger.Config      `yaml:"logger"`
	Lifecycle          lifecycle.Config   `yaml:"lifecycle"`
	GRPCServer         grpcserver.Config  `yaml:"grpc_server"`
	Healthcheck        grpcclient.Config  `yaml:"grpc_healthcheck"`
	CoreUser           grpcclient.Config  `yaml:"core_user_grpc"`
	Redis              redisclient.Config `yaml:"redis"`
	JWTSecret          string             `yaml:"jwt_secret" json:"-"`
	JWTIssuer          string             `yaml:"jwt_issuer"`
	JWTAudience        string             `yaml:"jwt_audience"`
	AccessTokenSeconds int                `yaml:"access_token_seconds"`
}

func Load() (Config, error) {
	cfg := Config{
		Logger: logger.DefaultConfig(),
		Lifecycle: lifecycle.Config{
			CleanupTimeout: 5 * time.Second,
		},
	}
	cfg.Logger.Service = "core-authentication"
	cfg.Redis = redisclient.DefaultConfig()
	cfg.CoreUser = grpcclient.DefaultConfig()
	cfg.CoreUser.Address = "localhost:9091"
	cfg.CoreUser.ServiceName = "auth"
	cfg.GRPCServer = grpcserver.DefaultConfig()
	cfg.GRPCServer.Address = ":9092"
	cfg.Healthcheck = grpcclient.DefaultConfig()
	cfg.JWTIssuer = "core-authentication"
	cfg.JWTAudience = "orch-user-access"
	cfg.AccessTokenSeconds = 900
	if err := config.Load(config.Config{
		Path: config.Path(),
	}, &cfg); err != nil {
		return cfg, err
	}
	if cfg.Healthcheck.Address == "" {
		cfg.Healthcheck.Address = cfg.GRPCServer.Address
	}
	if cfg.GRPCServer.MaxReceiveBytes < 1 ||
		cfg.GRPCServer.MaxReceiveBytes > 16777216 ||
		cfg.Redis.Port < 1 ||
		cfg.Redis.Port > 65535 ||
		cfg.Redis.DB < 0 ||
		cfg.Redis.DB > 2147483647 ||
		cfg.Redis.PoolSize < 1 ||
		cfg.Redis.PoolSize > 100000 ||
		cfg.Redis.MinIdleConns < 0 ||
		cfg.Redis.MinIdleConns > 100000 ||
		cfg.Redis.MaxIdleConns < 0 ||
		cfg.Redis.MaxIdleConns > 100000 ||
		cfg.Redis.MaxActiveConns < 0 ||
		cfg.Redis.MaxActiveConns > 100000 ||
		cfg.AccessTokenSeconds < 1 ||
		cfg.AccessTokenSeconds > 100000 {
		return cfg, fmt.Errorf("configuration integer outside allowed range")
	}
	return cfg, nil
}

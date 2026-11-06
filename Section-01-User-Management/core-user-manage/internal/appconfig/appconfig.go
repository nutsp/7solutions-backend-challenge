package appconfig

import (
	"fmt"
	"time"

	"github.com/nutsp/7solutions-backend-challenge/core-user-manage/pkg/config"
	"github.com/nutsp/7solutions-backend-challenge/core-user-manage/pkg/grpcclient"
	"github.com/nutsp/7solutions-backend-challenge/core-user-manage/pkg/grpcserver"
	"github.com/nutsp/7solutions-backend-challenge/core-user-manage/pkg/lifecycle"
	"github.com/nutsp/7solutions-backend-challenge/core-user-manage/pkg/logger"
	"github.com/nutsp/7solutions-backend-challenge/core-user-manage/pkg/mongoclient"
)

type ProtectionConfig struct {
	UserKeyID  string `yaml:"user_key_id"`
	AuditKeyID string `yaml:"audit_key_id"`
}

type Config struct {
	UserDataKeys   string             `yaml:"user_data_keys" json:"-"`
	UserAuditKeys  string             `yaml:"user_audit_keys" json:"-"`
	EmailLookupKey string             `yaml:"email_lookup_key" json:"-"`
	Cryptography   ProtectionConfig   `yaml:"cryptography"`
	Logger         logger.Config      `yaml:"logger"`
	Lifecycle      lifecycle.Config   `yaml:"lifecycle"`
	GRPCServer     grpcserver.Config  `yaml:"grpc_server"`
	Healthcheck    grpcclient.Config  `yaml:"grpc_healthcheck"`
	Mongo          mongoclient.Config `yaml:"mongo"`
}

func Load() (Config, error) {
	cfg := Config{
		Logger: logger.DefaultConfig(),
		Lifecycle: lifecycle.Config{
			CleanupTimeout: 5 * time.Second,
		},
	}
	cfg.Logger.Service = "core-user-manage"
	cfg.Cryptography = ProtectionConfig{
		UserKeyID:  "v1",
		AuditKeyID: "v1",
	}
	cfg.GRPCServer = grpcserver.DefaultConfig()
	cfg.GRPCServer.Address = ":9091"
	cfg.Healthcheck = grpcclient.DefaultConfig()
	cfg.Mongo = mongoclient.DefaultConfig()
	cfg.Mongo.Database = "identity"
	cfg.Mongo.AppName = "core-user-manage"
	if err := config.Load(config.Config{
		Path: config.Path(),
	}, &cfg); err != nil {
		return cfg, err
	}
	if cfg.Healthcheck.Address == "" {
		cfg.Healthcheck.Address = cfg.GRPCServer.Address
	}
	if cfg.GRPCServer.MaxReceiveBytes < 1 || cfg.GRPCServer.MaxReceiveBytes > 16777216 {
		return cfg, fmt.Errorf("configuration integer outside allowed range")
	}
	return cfg, nil
}

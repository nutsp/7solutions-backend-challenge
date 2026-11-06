package mongoclient

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

type Config struct {
	URI                    string        `yaml:"uri" json:"-"`
	Database               string        `yaml:"database"`
	AppName                string        `yaml:"app_name"`
	Username               string        `yaml:"username"`
	Password               string        `yaml:"password" json:"-"`
	AuthSource             string        `yaml:"auth_source"`
	MaxPoolSize            uint64        `yaml:"max_pool_size"`
	MinPoolSize            uint64        `yaml:"min_pool_size"`
	MaxConnecting          uint64        `yaml:"max_connecting"`
	MaxConnIdleTime        time.Duration `yaml:"max_conn_idle_time"`
	ConnectTimeout         time.Duration `yaml:"connect_timeout"`
	ServerSelectionTimeout time.Duration `yaml:"server_selection_timeout"`
	OperationTimeout       time.Duration `yaml:"operation_timeout"`
	PingTimeout            time.Duration `yaml:"ping_timeout"`
}

func New(ctx context.Context, cfg Config) (*mongo.Client, error) {
	opts, err := cfg.options()
	if err != nil {
		return nil, err
	}
	client, err := mongo.Connect(opts)
	if err != nil {
		return nil, err
	}
	timeout := cfg.PingTimeout
	if timeout == 0 {
		timeout = DefaultConfig().PingTimeout
	}
	bounded, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := client.Ping(bounded, readpref.Primary()); err != nil {
		cleanup, c := context.WithTimeout(context.Background(), 5*time.Second)
		defer c()
		_ = client.Disconnect(cleanup)
		return nil, fmt.Errorf("MongoDB unavailable: %w", err)
	}
	return client, nil
}

func DefaultConfig() Config {
	return Config{
		MaxPoolSize:            50,
		MinPoolSize:            2,
		MaxConnecting:          4,
		MaxConnIdleTime:        5 * time.Minute,
		ConnectTimeout:         5 * time.Second,
		ServerSelectionTimeout: 5 * time.Second,
		OperationTimeout:       5 * time.Second,
		PingTimeout:            10 * time.Second,
	}
}

func (c Config) options() (*options.ClientOptions, error) {
	d := DefaultConfig()
	if c.MaxPoolSize == 0 {
		c.MaxPoolSize = d.MaxPoolSize
	}
	if c.MaxConnecting == 0 {
		c.MaxConnecting = d.MaxConnecting
	}
	if c.MaxConnIdleTime == 0 {
		c.MaxConnIdleTime = d.MaxConnIdleTime
	}
	if c.ConnectTimeout == 0 {
		c.ConnectTimeout = d.ConnectTimeout
	}
	if c.ServerSelectionTimeout == 0 {
		c.ServerSelectionTimeout = d.ServerSelectionTimeout
	}
	if c.OperationTimeout == 0 {
		c.OperationTimeout = d.OperationTimeout
	}
	if c.URI == "" {
		return nil, fmt.Errorf("MongoDB URI is required")
	}
	if c.MinPoolSize > c.MaxPoolSize {
		return nil, fmt.Errorf("MongoDB minimum pool exceeds maximum pool")
	}
	if c.MaxConnIdleTime < 0 || c.ConnectTimeout < 0 || c.ServerSelectionTimeout < 0 || c.OperationTimeout < 0 || c.PingTimeout < 0 {
		return nil, fmt.Errorf("MongoDB timeouts must not be negative")
	}
	if c.Password != "" && c.Username == "" {
		return nil, fmt.Errorf("MongoDB username is required with password")
	}
	opts := options.Client().ApplyURI(c.URI).SetAppName(c.AppName).SetMaxPoolSize(c.MaxPoolSize).SetMinPoolSize(c.MinPoolSize).SetMaxConnecting(c.MaxConnecting).SetMaxConnIdleTime(c.MaxConnIdleTime).SetConnectTimeout(c.ConnectTimeout).SetServerSelectionTimeout(c.ServerSelectionTimeout).SetTimeout(c.OperationTimeout)
	if c.Username != "" {
		opts.SetAuth(options.Credential{
			Username:   c.Username,
			Password:   c.Password,
			AuthSource: c.AuthSource,
		})
	}
	return opts, nil
}

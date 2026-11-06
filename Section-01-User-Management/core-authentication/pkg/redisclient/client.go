// Package redisclient constructs Redis connections from explicit, typed settings.
package redisclient

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type Config struct {
	Host            string        `yaml:"host"`
	Port            int           `yaml:"port"`
	Username        string        `yaml:"username"`
	Password        string        `yaml:"password" json:"-"`
	DB              int           `yaml:"db"`
	PoolSize        int           `yaml:"pool_size"`
	MinIdleConns    int           `yaml:"min_idle_conns"`
	MaxIdleConns    int           `yaml:"max_idle_conns"`
	MaxActiveConns  int           `yaml:"max_active_conns"`
	PoolTimeout     time.Duration `yaml:"pool_timeout"`
	DialTimeout     time.Duration `yaml:"dial_timeout"`
	ReadTimeout     time.Duration `yaml:"read_timeout"`
	WriteTimeout    time.Duration `yaml:"write_timeout"`
	ConnMaxIdleTime time.Duration `yaml:"conn_max_idle_time"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime"`
	PingTimeout     time.Duration `yaml:"ping_timeout"`
	TLS             bool          `yaml:"tls"`
	CAFile          string        `yaml:"ca_file"`
	ServerName      string        `yaml:"server_name"`
}

func New(ctx context.Context, cfg Config) (*redis.Client, error) {
	options, err := cfg.options()
	if err != nil {
		return nil, err
	}
	client := redis.NewClient(options)
	timeout := cfg.PingTimeout
	if timeout == 0 {
		timeout = DefaultConfig().PingTimeout
	}
	bounded, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := client.Ping(bounded).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("Redis unavailable: %w", err)
	}
	return client, nil
}

func DefaultConfig() Config {
	return Config{
		Host:            "127.0.0.1",
		Port:            6379,
		PoolSize:        20,
		PoolTimeout:     2 * time.Second,
		DialTimeout:     3 * time.Second,
		ReadTimeout:     time.Second,
		WriteTimeout:    time.Second,
		ConnMaxIdleTime: 30 * time.Minute,
		PingTimeout:     5 * time.Second,
	}
}

func (c Config) options() (*redis.Options, error) {
	defaults := DefaultConfig()
	if c.Port == 0 {
		c.Port = defaults.Port
	}
	if c.PoolSize == 0 {
		c.PoolSize = defaults.PoolSize
	}
	if c.MaxActiveConns == 0 {
		c.MaxActiveConns = c.PoolSize
	}
	if c.PoolTimeout == 0 {
		c.PoolTimeout = defaults.PoolTimeout
	}
	if c.DialTimeout == 0 {
		c.DialTimeout = defaults.DialTimeout
	}
	if c.ReadTimeout == 0 {
		c.ReadTimeout = defaults.ReadTimeout
	}
	if c.WriteTimeout == 0 {
		c.WriteTimeout = defaults.WriteTimeout
	}
	if c.ConnMaxIdleTime == 0 {
		c.ConnMaxIdleTime = defaults.ConnMaxIdleTime
	}
	if strings.TrimSpace(c.Host) == "" || strings.Contains(c.Host, "://") || c.Port < 1 || c.Port > 65535 || c.DB < 0 {
		return nil, fmt.Errorf("invalid Redis host, port or database")
	}
	if c.PoolSize < 1 || c.MinIdleConns < 0 || c.MinIdleConns > c.PoolSize || c.MaxIdleConns < 0 || c.MaxIdleConns > c.MaxActiveConns || c.MaxActiveConns < c.PoolSize || (c.MaxIdleConns > 0 && c.MinIdleConns > c.MaxIdleConns) {
		return nil, fmt.Errorf("invalid Redis pool settings")
	}
	if c.PoolTimeout < 0 || c.DialTimeout < 0 || c.ReadTimeout < 0 || c.WriteTimeout < 0 || c.ConnMaxIdleTime < 0 || c.ConnMaxLifetime < 0 || c.PingTimeout < 0 {
		return nil, fmt.Errorf("Redis timeouts must not be negative")
	}
	if !c.TLS && (c.CAFile != "" || c.ServerName != "") {
		return nil, fmt.Errorf("Redis TLS must be enabled for CA/server-name settings")
	}
	opts := &redis.Options{
		Addr:                  net.JoinHostPort(c.Host, strconv.Itoa(c.Port)),
		Username:              c.Username,
		Password:              c.Password,
		DB:                    c.DB,
		PoolSize:              c.PoolSize,
		MinIdleConns:          c.MinIdleConns,
		MaxIdleConns:          c.MaxIdleConns,
		MaxActiveConns:        c.MaxActiveConns,
		PoolTimeout:           c.PoolTimeout,
		DialTimeout:           c.DialTimeout,
		ReadTimeout:           c.ReadTimeout,
		WriteTimeout:          c.WriteTimeout,
		ConnMaxIdleTime:       c.ConnMaxIdleTime,
		ConnMaxLifetime:       c.ConnMaxLifetime,
		ContextTimeoutEnabled: true,
	}
	if c.TLS {
		opts.TLSConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
			ServerName: c.ServerName,
		}
		if c.CAFile != "" {
			pem, err := os.ReadFile(c.CAFile)
			if err != nil {
				return nil, err
			}
			roots := x509.NewCertPool()
			if !roots.AppendCertsFromPEM(pem) {
				return nil, fmt.Errorf("invalid Redis CA file")
			}
			opts.TLSConfig.RootCAs = roots
		}
	}
	return opts, nil
}

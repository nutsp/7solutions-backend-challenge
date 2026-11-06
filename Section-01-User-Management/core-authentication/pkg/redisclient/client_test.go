package redisclient

import (
	"context"
	"testing"
	"time"
)

func TestMalformedConfigurationRejected(t *testing.T) {
	tests := []struct {
		name string
		host string
	}{{"URL is not a host", "http://localhost:6379"}, {"empty host", ""}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if client, err := New(context.Background(), Config{
				Host: tt.host,
			}); err == nil {
				client.Close()
				t.Fatal("invalid Redis host accepted")
			}

		})
	}
}

func TestConfigReachesSDKOptions(t *testing.T) {
	tests := []struct {
		name    string
		host    string
		address string
	}{{"IPv6", "::1", "[::1]:6380"}, {"DNS", "redis", "redis:6380"}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{
				Host:            tt.host,
				Port:            6380,
				Username:        "worker",
				Password:        "private-value",
				DB:              4,
				PoolSize:        7,
				MinIdleConns:    2,
				MaxIdleConns:    5,
				MaxActiveConns:  11,
				PoolTimeout:     700 * time.Millisecond,
				DialTimeout:     2 * time.Second,
				ReadTimeout:     3 * time.Second,
				WriteTimeout:    4 * time.Second,
				ConnMaxIdleTime: 9 * time.Minute,
				ConnMaxLifetime: time.Hour,
			}
			opts, err := cfg.options()
			if err != nil {
				t.Fatal(err)
			}
			if opts.Addr != tt.address || opts.Username != cfg.Username || opts.Password != cfg.Password || opts.DB != 4 {
				t.Fatal("connection/authentication settings lost")
			}
			if opts.PoolSize != 7 || opts.MinIdleConns != 2 || opts.MaxIdleConns != 5 || opts.MaxActiveConns != 11 || opts.PoolTimeout != cfg.PoolTimeout {
				t.Fatal("pool settings lost")
			}
			if opts.DialTimeout != cfg.DialTimeout || opts.ReadTimeout != cfg.ReadTimeout || opts.WriteTimeout != cfg.WriteTimeout || opts.ConnMaxIdleTime != cfg.ConnMaxIdleTime || opts.ConnMaxLifetime != cfg.ConnMaxLifetime {
				t.Fatal("timeout/lifetime settings lost")
			}

		})
	}
}

func TestInvalidPoolRelationships(t *testing.T) {
	tests := []struct {
		name   string
		config Config
	}{
		{"idle minimum exceeds pool", Config{
			Host:         "localhost",
			PoolSize:     2,
			MinIdleConns: 3,
		}},
		{"active maximum below pool", Config{
			Host:           "localhost",
			PoolSize:       3,
			MaxActiveConns: 2,
		}},
		{"idle minimum exceeds maximum", Config{
			Host:         "localhost",
			PoolSize:     3,
			MinIdleConns: 2,
			MaxIdleConns: 1,
		}},
		{"invalid port", Config{
			Host: "localhost",
			Port: 70000,
		}},
		{"negative timeout", Config{
			Host:        "localhost",
			ReadTimeout: -time.Second,
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := tt.config.options(); err == nil {
				t.Fatal("invalid connection settings accepted")
			}
		})
	}
}

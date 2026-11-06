package mongoclient

import (
	"context"
	"testing"
	"time"
)

func TestInvalidPoolRejectedBeforeConnecting(t *testing.T) {
	tests := []struct {
		name    string
		minPool uint64
		maxPool uint64
	}{{"minimum exceeds maximum", 2, 1}, {"larger minimum exceeds maximum", 20, 10}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if client, err := New(context.Background(), Config{
				URI:         "mongodb://localhost:1",
				MaxPoolSize: tt.maxPool,
				MinPoolSize: tt.minPool,
			}); err == nil {
				client.Disconnect(context.Background())
				t.Fatal("invalid pool accepted")
			}

		})
	}
}

func TestConfigReachesDriverOptions(t *testing.T) {
	tests := []struct {
		name     string
		username string
	}{{"application user", "user"}, {"worker user", "worker"}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{
				URI:                    "mongodb://localhost:27017",
				AppName:                "unit-test",
				Username:               tt.username,
				Password:               "private-value",
				AuthSource:             "identity",
				MaxPoolSize:            17,
				MinPoolSize:            3,
				MaxConnecting:          2,
				MaxConnIdleTime:        time.Minute,
				ConnectTimeout:         2 * time.Second,
				ServerSelectionTimeout: 3 * time.Second,
				OperationTimeout:       4 * time.Second,
			}
			opts, err := cfg.options()
			if err != nil {
				t.Fatal(err)
			}
			if *opts.MaxPoolSize != 17 || *opts.MinPoolSize != 3 || *opts.MaxConnecting != 2 || *opts.MaxConnIdleTime != cfg.MaxConnIdleTime {
				t.Fatal("MongoDB pool settings lost")
			}
			if *opts.ConnectTimeout != cfg.ConnectTimeout || *opts.ServerSelectionTimeout != cfg.ServerSelectionTimeout || *opts.Timeout != cfg.OperationTimeout {
				t.Fatal("MongoDB timeout settings lost")
			}
			if opts.Auth == nil || opts.Auth.Username != cfg.Username || opts.Auth.Password != cfg.Password || opts.Auth.AuthSource != cfg.AuthSource {
				t.Fatal("MongoDB credential settings lost")
			}

		})
	}
}

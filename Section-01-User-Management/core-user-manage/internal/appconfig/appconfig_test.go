package appconfig

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUnknownFieldsRejected(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"unknown root field", "unknown_setting: true\n"},
		{"multiple YAML documents", "{}\n---\n{}\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(path, []byte(tt.body), 0600); err != nil {
				t.Fatal(err)
			}
			t.Setenv("CONFIG_FILE", path)
			if _, err := Load(); err == nil {
				t.Fatal("unknown or ambiguous configuration accepted")
			}
		})
	}
}

func TestEnvironmentOverridesYAML(t *testing.T) {
	tests := []struct {
		name    string
		address string
	}{{"first address", "127.0.0.1:8181"}, {"second address", "127.0.0.1:8282"}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(path, []byte("mongo:\n  max_pool_size: 50\n"), 0600); err != nil {
				t.Fatal(err)
			}
			t.Setenv("GRPC_SERVER_ADDRESS", tt.address)
			t.Setenv("CONFIG_FILE", path)
			cfg, err := Load()
			if err != nil {
				t.Fatal(err)
			}
			if cfg.GRPCServer.Address != tt.address {
				t.Fatal("environment override failed")
			}
		})
	}
}

func TestCredentialsFromYAMLAndEnvironment(t *testing.T) {
	for _, tt := range []struct {
		name     string
		override bool
	}{{"YAML", false}, {"environment overrides YAML", true}} {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			body := "mongo:\n  password: file-credential-value-with-32-bytes\nuser_data_keys: file-credential-value-with-32-bytes\nuser_audit_keys: file-credential-value-with-32-bytes\nemail_lookup_key: file-credential-value-with-32-bytes\n"
			if err := os.WriteFile(path, []byte(body), 0600); err != nil {
				t.Fatal(err)
			}
			t.Setenv("CONFIG_FILE", path)
			want := "file-credential-value-with-32-bytes"
			if tt.override {
				want = "environment-credential-with-32-bytes"
				t.Setenv("MONGO_PASSWORD", want)
				t.Setenv("USER_DATA_KEYS", want)
				t.Setenv("USER_AUDIT_KEYS", want)
				t.Setenv("EMAIL_LOOKUP_KEY", want)
			}
			cfg, err := Load()
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Mongo.Password != want || cfg.UserDataKeys != want || cfg.UserAuditKeys != want || cfg.EmailLookupKey != want {
				t.Fatal("credential configuration did not match source")
			}
		})
	}
}

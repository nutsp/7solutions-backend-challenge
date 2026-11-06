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
			if err := os.WriteFile(path, []byte("http:\n  address: \"127.0.0.1:8080\"\n"), 0600); err != nil {
				t.Fatal(err)
			}
			t.Setenv("HTTP_ADDRESS", tt.address)
			t.Setenv("CONFIG_FILE", path)
			cfg, err := Load()
			if err != nil {
				t.Fatal(err)
			}
			if cfg.HTTP.Address != tt.address {
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
			body := "core_user_grpc:\n  service_token: file-credential-value-with-32-bytes\ncore_auth_grpc:\n  service_token: file-credential-value-with-32-bytes\n"
			if err := os.WriteFile(path, []byte(body), 0600); err != nil {
				t.Fatal(err)
			}
			t.Setenv("CONFIG_FILE", path)
			want := "file-credential-value-with-32-bytes"
			if tt.override {
				want = "environment-credential-with-32-bytes"
				t.Setenv("CORE_USER_GRPC_SERVICE_TOKEN", want)
				t.Setenv("CORE_AUTH_GRPC_SERVICE_TOKEN", want)
			}
			cfg, err := Load()
			if err != nil {
				t.Fatal(err)
			}
			if cfg.CoreUser.ServiceToken != want || cfg.CoreAuth.ServiceToken != want {
				t.Fatal("credential configuration did not match source")
			}
		})
	}
}

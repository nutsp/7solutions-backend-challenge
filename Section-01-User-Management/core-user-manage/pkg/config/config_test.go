package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

type settings struct {
	Name string `yaml:"name"`
}

func TestStrictSingleDocument(t *testing.T) {
	tests := []struct {
		name  string
		body  string
		valid bool
	}{
		{"valid", "name: example\n", true}, {"unknown field", "other: example\n", false}, {"multiple documents", "name: first\n---\nname: second\n", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(path, []byte(tt.body), 0600); err != nil {
				t.Fatal(err)
			}
			var out settings
			if err := Load(Config{
				Path: path,
			}, &out); (err == nil) != tt.valid {
				t.Fatalf("valid=%v error=%v", tt.valid, err)
			}
		})
	}
}

func TestAutomaticEnvironment(t *testing.T) {
	tests := []struct {
		name        string
		yaml        string
		env         string
		present     bool
		valid       bool
		wantName    string
		wantCount   int
		wantDelay   time.Duration
		wantEnabled bool
	}{
		{
			name:      "destination defaults",
			yaml:      "{}",
			valid:     true,
			wantName:  "default",
			wantCount: 5,
			wantDelay: time.Second,
		},
		{
			name:        "file overrides defaults",
			yaml:        "name: file\ncount: 7\ndelay: 2s\nenabled: true",
			valid:       true,
			wantName:    "file",
			wantCount:   7,
			wantDelay:   2 * time.Second,
			wantEnabled: true,
		},
		{
			name:      "env overrides file",
			yaml:      "name: file",
			env:       "environment",
			present:   true,
			valid:     true,
			wantName:  "environment",
			wantCount: 5,
			wantDelay: time.Second,
		},
		{
			name:      "env overrides without YAML key",
			yaml:      "{}",
			env:       "environment",
			present:   true,
			valid:     true,
			wantName:  "environment",
			wantCount: 5,
			wantDelay: time.Second,
		},
		{
			name:      "explicit empty env",
			yaml:      "name: file",
			present:   true,
			valid:     true,
			wantCount: 5,
			wantDelay: time.Second,
		},
		{
			name: "invalid integer",
			yaml: "count: invalid",
		},
		{
			name: "invalid duration",
			yaml: "delay: invalid",
		},
		{
			name: "invalid boolean",
			yaml: "enabled: invalid",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(path, []byte(tt.yaml), 0600); err != nil {
				t.Fatal(err)
			}
			t.Setenv("NAME", tt.env)
			if !tt.present {
				if err := os.Unsetenv("NAME"); err != nil {
					t.Fatal(err)
				}
			}
			out := struct {
				Name    string        `yaml:"name"`
				Count   int           `yaml:"count"`
				Delay   time.Duration `yaml:"delay"`
				Enabled bool          `yaml:"enabled"`
			}{
				Name:  "default",
				Count: 5,
				Delay: time.Second,
			}
			err := Load(Config{
				Path: path,
			}, &out)
			if (err == nil) != tt.valid {
				t.Fatalf("valid=%v error=%v", tt.valid, err)
			}
			if !tt.valid {
				return
			}
			if out.Name != tt.wantName || out.Count != tt.wantCount || out.Delay != tt.wantDelay || out.Enabled != tt.wantEnabled {
				t.Fatalf("unexpected decoded configuration: %+v", out)
			}
		})
	}
}

func TestEnvironmentOnlyValues(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    string
		present bool
	}{
		{
			name: "missing uses fallback",
			want: "fallback",
		},
		{
			name:    "explicit empty",
			present: true,
		},
		{
			name:    "explicit value",
			present: true,
			value:   "configured",
			want:    "configured",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("PKG_TEST_VALUE", tt.value)
			if !tt.present {
				if err := os.Unsetenv("PKG_TEST_VALUE"); err != nil {
					t.Fatal(err)
				}
			}
			if got := Env("PKG_TEST_VALUE", "fallback"); got != tt.want {
				t.Fatalf("got %q; want %q", got, tt.want)
			}
		})
	}
}

func TestAutomaticNestedEnvironment(t *testing.T) {
	tests := []struct {
		name        string
		pool        string
		timeout     string
		enabled     string
		valid       bool
		wantPool    uint64
		wantTimeout time.Duration
		wantEnabled bool
	}{
		{"env-only fields absent from YAML", "7", "2s", "true", true, 7, 2 * time.Second, true},
		{"explicit zero and false", "0", "0s", "false", true, 0, 0, false},
		{
			name:    "invalid unsigned integer",
			pool:    "-1",
			timeout: "2s",
			enabled: "true",
		},
		{
			name:    "invalid duration",
			pool:    "7",
			timeout: "bad",
			enabled: "true",
		},
		{
			name:    "invalid boolean",
			pool:    "7",
			timeout: "2s",
			enabled: "bad",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(path, []byte("{}"), 0600); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PKG_TEST_POOL_SIZE", tt.pool)
			t.Setenv("PKG_TEST_TIMEOUT", tt.timeout)
			t.Setenv("PKG_TEST_ENABLED", tt.enabled)
			var out struct {
				Settings struct {
					PoolSize uint64        `yaml:"pool_size"`
					Timeout  time.Duration `yaml:"timeout"`
					Enabled  bool          `yaml:"enabled"`
				} `yaml:"pkg_test"`
			}
			err := Load(Config{
				Path: path,
			}, &out)
			if (err == nil) != tt.valid {
				t.Fatalf("valid=%v error=%v", tt.valid, err)
			}
			if err == nil && (out.Settings.PoolSize != tt.wantPool || out.Settings.Timeout != tt.wantTimeout || out.Settings.Enabled != tt.wantEnabled) {
				t.Fatal("nested environment value not decoded")
			}
		})
	}
}

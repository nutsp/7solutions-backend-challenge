// Package config loads typed settings using an isolated Viper instance per load.
package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/go-viper/mapstructure/v2"
	"github.com/spf13/viper"
)

type Config struct {
	Path string
}

func Path() string {

	return Env("CONFIG_FILE", "config/config.yaml")
}

// Load uses environment > YAML > destination defaults. Unknown keys and invalid
// values fail startup. Struct binding discovers env-only fields even when absent
// from YAML. Environment names are uppercased YAML paths with dots replaced by _.
// Errors intentionally omit configuration values and secrets.
func Load(cfg Config, out any) error {
	v := viper.NewWithOptions(viper.WithDecoderRegistry(singleDocumentYAML{}), viper.ExperimentalBindStruct())
	v.SetConfigType("yaml")
	if cfg.Path == "" {
		cfg.Path = "config/config.yaml"
	}
	v.SetConfigFile(cfg.Path)
	v.AllowEmptyEnv(true)
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	if err := v.ReadInConfig(); err != nil {
		return fmt.Errorf("read configuration file")
	}
	if err := v.UnmarshalExact(out, func(dc *mapstructure.DecoderConfig) {

		dc.TagName = "yaml"
	}); err != nil {
		return fmt.Errorf("invalid configuration fields or values")
	}
	return nil
}

// Env reads process settings such as CONFIG_FILE before loading Viper.
func Env(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

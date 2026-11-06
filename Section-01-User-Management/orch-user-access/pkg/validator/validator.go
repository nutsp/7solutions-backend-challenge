// Package validator constructs validator/v10 engines without business-field rules.
package validator

import playground "github.com/go-playground/validator/v10"

type Config struct {
	TagName string
}

// New returns an engine for the caller to configure before sharing it.
// Request tags, normalization and custom field rules belong to inbound adapters.
func New(cfg Config) *playground.Validate {
	engine := playground.New(playground.WithRequiredStructEnabled())
	if cfg.TagName != "" {
		engine.SetTagName(cfg.TagName)
	}
	return engine
}

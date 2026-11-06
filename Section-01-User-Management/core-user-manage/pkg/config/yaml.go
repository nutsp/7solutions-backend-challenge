package config

import (
	"bytes"
	"fmt"
	"io"

	"github.com/spf13/viper"
	"go.yaml.in/yaml/v3"
)

// Viper's default YAML decoder silently ignores trailing documents. Keep the
// existing single-document policy while Viper owns loading, binding and decoding.
type singleDocumentYAML struct{}

func (singleDocumentYAML) Decoder(format string) (viper.Decoder, error) {
	if format != "yaml" && format != "yml" {
		return nil, fmt.Errorf("unsupported configuration format")
	}
	return singleDocumentYAML{}, nil
}

func (singleDocumentYAML) Decode(data []byte, values map[string]any) error {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&values); err != nil {
		return err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("configuration requires exactly one YAML document")
	}
	return nil
}

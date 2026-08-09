package config

import (
	"log/slog"

	"github.com/ondatra-ai/true-bdd/src/internal/pkg/errors"
	"github.com/spf13/viper"
)

type ViperConfig struct {
	viper *viper.Viper
}

func NewViperConfig() (*ViperConfig, error) {
	viperInstance := viper.New()

	viperInstance.SetConfigFile("./true-bdd/true-bdd.yaml")
	viperInstance.SetConfigType("yaml")

	err := viperInstance.ReadInConfig()
	if err != nil {
		slog.Error("Failed to read config file", "error", err)

		return nil, errors.ErrReadConfigFailed(err)
	}

	return &ViperConfig{viper: viperInstance}, nil
}

func (c *ViperConfig) GetString(key string) string {
	if !c.viper.IsSet(key) {
		return ""
	}

	return c.viper.GetString(key)
}

// GetStringMapString reads a nested string→string mapping, e.g. the
// `engine.models` tier table. An unset key yields nil so callers can
// tell "absent" from "present but empty".
func (c *ViperConfig) GetStringMapString(key string) map[string]string {
	if !c.viper.IsSet(key) {
		return nil
	}

	return c.viper.GetStringMapString(key)
}

// GetStringSlice reads a YAML list of strings, e.g. the
// `paths.test_write_globs` write roots. An unset key yields nil so
// callers can apply their own default.
func (c *ViperConfig) GetStringSlice(key string) []string {
	if !c.viper.IsSet(key) {
		return nil
	}

	return c.viper.GetStringSlice(key)
}

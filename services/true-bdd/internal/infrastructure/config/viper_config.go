package config

import (
	"fmt"
	"log/slog"
	"strconv"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/pkg/errors"
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

		return nil, errors.ErrReadConfigFailed(errors.ProjectFile("./true-bdd/true-bdd.yaml", err))
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

// PromptLimitBytes reads the positive host override; absence keeps the engine default.
func (c *ViperConfig) PromptLimitBytes(fallback int) (int, error) {
	const key = "engine.max_prompt_bytes"
	if !c.viper.IsSet(key) {
		return fallback, nil
	}

	limit, err := strconv.Atoi(c.viper.GetString(key))
	if err != nil || limit <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer: %w", key, strconv.ErrSyntax)
	}

	return limit, nil
}

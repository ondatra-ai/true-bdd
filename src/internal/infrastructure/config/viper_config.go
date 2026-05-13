package config

import (
	"log/slog"

	"bdd-cli/src/internal/pkg/errors"
	"github.com/spf13/viper"
)

type ViperConfig struct {
	viper *viper.Viper
}

func NewViperConfig() (*ViperConfig, error) {
	viperInstance := viper.New()

	viperInstance.SetConfigFile("./bdd-cli/bdd-cli.yaml")
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

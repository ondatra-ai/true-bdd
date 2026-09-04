package claudecli

import (
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"gopkg.in/yaml.v3"
)

// configPath is read relative to the repo root, which every scripts/ binary
// is run from.
const configPath = "true-bdd/true-bdd.yaml"

// models is the `scripts.models` block: a default, and two override maps.
type models struct {
	Default    string            `yaml:"default"`
	PerScript  map[string]string `yaml:"per_script"`
	PerCommand map[string]string `yaml:"per_command"`
}

//nolint:gochecknoglobals // the config is read once per process, by design.
var (
	loadOnce sync.Once
	loaded   models
)

// load reads the config once. A missing or unreadable file is not fatal: an
// empty result leaves every turn on whatever model its call site named, which
// is exactly the behaviour of a repo that has not configured one.
func load() models {
	loadOnce.Do(func() {
		raw, err := disk.Read(configPath)
		if err != nil {
			slog.Debug("no scripts model config", "path", configPath, "error", err)

			return
		}

		var config struct {
			Scripts struct {
				Models models `yaml:"models"`
			} `yaml:"scripts"`
		}

		err = yaml.Unmarshal(raw, &config)
		if err != nil {
			slog.Warn("scripts model config unparseable", "path", configPath, "error", err)

			return
		}

		loaded = config.Scripts.Models
	})

	return loaded
}

// Model resolves the model for one turn — the command's, else the running
// script's, else the default. Empty leaves the CLI's own choice.
func Model(command string) string {
	return resolve(load(), script(), command)
}

// resolve is the precedence itself, kept apart from reading the file so the
// order can be asserted without a working directory.
func resolve(config models, script, command string) string {
	if model, ok := config.PerCommand[command]; ok {
		return model
	}

	if model, ok := config.PerScript[script]; ok {
		return model
	}

	return config.Default
}

// script is the binary's own name, which is the scripts/cmd/<name> it was
// built from under both `go run` and a built ./bin entry.
func script() string {
	return filepath.Base(os.Args[0])
}

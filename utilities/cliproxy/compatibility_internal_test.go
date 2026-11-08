package main

import (
	"errors"
	"sync"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"gopkg.in/yaml.v3"
)

// The legacy hash examples use the host's declaration; production has no default recording rules.
//
//nolint:gochecknoglobals // immutable test evidence loaded once from this checkout's Specification.
var compatibilityRecording = sync.OnceValue(func() recording {
	data, err := disk.Read("../../docs/architecture/architecture.yaml")
	if err != nil {
		panic(err)
	}

	var document struct {
		Architecture struct {
			Testing struct {
				Commands struct {
					Record struct {
						Exclude []string `yaml:"exclude"`
					} `yaml:"record"`
				} `yaml:"commands"`
			} `yaml:"testing"`
			Services []struct {
				Dependencies []struct {
					Name      string    `yaml:"name"`
					Recording recording `yaml:"recording"`
				} `yaml:"dependencies"`
			} `yaml:"services"`
		} `yaml:"architecture"`
	}

	err = yaml.Unmarshal(data, &document)
	if err != nil {
		panic(err)
	}

	for _, service := range document.Architecture.Services {
		for _, dependency := range service.Dependencies {
			if dependency.Name == "codex" {
				dependency.Recording.Unread = append(dependency.Recording.Unread,
					document.Architecture.Testing.Commands.Record.Exclude...)

				err = dependency.Recording.prepare()
				if err != nil {
					panic(err)
				}

				return dependency.Recording
			}
		}
	}

	panic(errors.New("no compatibility recording declaration"))
})

func normalizeText(text, cwd string) string { return normalize(text, cwd, compatibilityRecording()) }
func normalizeArgv(argv []string, cwd string) []string {
	return normalizedArgs(argv, cwd, compatibilityRecording())
}
func requestHash(argv []string, stdin []byte, cwd string) string {
	return hashRequest(argv, stdin, cwd, compatibilityRecording())
}
func denormalize(text, cwd, run string) (string, error) {
	return restoreText(text, cwd, map[string]string{"{{RUN_DIR}}": run})
}
func isEngineOwned(relative string) bool { return compatibilityRecording().omits(relative) }

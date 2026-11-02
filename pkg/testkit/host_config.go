package testkit

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"gopkg.in/yaml.v3"
)

type hostConfig struct {
	path         string
	registry     string
	architecture string
}

func loadHostConfig(root string) (hostConfig, error) {
	config := hostConfig{path: filepath.Join(root, "true-bdd", "true-bdd.yaml")}

	raw, err := disk.Read(config.path)
	if err != nil {
		return config, fmt.Errorf("read host config: %w", err)
	}

	var doc struct {
		Documents struct {
			Registry     string `yaml:"scenarios_yaml"`
			Architecture string `yaml:"architecture_yaml"`
		} `yaml:"documents"`
	}

	err = yaml.Unmarshal(raw, &doc)
	if err != nil {
		return config, fmt.Errorf("parse host config: %w", err)
	}

	config.registry = filepath.Join(root, doc.Documents.Registry)
	config.architecture = filepath.Join(root, doc.Documents.Architecture)

	return config, nil
}

type serviceBuild struct {
	Name     string `yaml:"name"`
	Path     string `yaml:"path"`
	Language string `yaml:"language"`
}

var errServiceBuild = errors.New("harness requires a declared Go service path")

func loadService(architecture, name string) (serviceBuild, error) {
	raw, err := disk.Read(architecture)
	if err != nil {
		return serviceBuild{}, fmt.Errorf("read service architecture: %w", err)
	}

	var doc struct {
		Architecture struct {
			Services []serviceBuild `yaml:"services"`
		} `yaml:"architecture"`
	}

	err = yaml.Unmarshal(raw, &doc)
	if err != nil {
		return serviceBuild{}, fmt.Errorf("parse service architecture: %w", err)
	}

	for _, service := range doc.Architecture.Services {
		if service.Name == name {
			if service.Path == "" || service.Language != "go" {
				return serviceBuild{}, fmt.Errorf("%w: %s (%s, %s)", errServiceBuild, name, service.Path, service.Language)
			}

			return service, nil
		}
	}

	return serviceBuild{}, fmt.Errorf("%w: %s", errServiceBuild, name)
}

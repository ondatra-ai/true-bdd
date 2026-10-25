package ai

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

func codexRuntimePackage(host, executable string) (string, error) {
	root := filepath.Dir(filepath.Dir(executable))
	if filepath.Base(filepath.Dir(executable)) != "bin" {
		return "", fmt.Errorf("runtime is outside package bin: %w", os.ErrPermission)
	}

	data, err := disk.Read(filepath.Join(root, "package.json"))
	if err != nil {
		return "", fmt.Errorf("read runtime package: %w", err)
	}

	var manifest struct {
		Name string `json:"name"`
	}

	err = json.Unmarshal(data, &manifest)
	if err != nil {
		return "", fmt.Errorf("decode runtime package: %w", err)
	}

	if manifest.Name != "npm" {
		return "", fmt.Errorf("runtime package is not npm: %w", os.ErrPermission)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("locate runtime home: %w", err)
	}

	protected := []string{host, filepath.Join(home, ".codex"), filepath.Join(home, ".claude"),
		filepath.Join(home, "work"), filepath.Join(home, ".ssh"), filepath.Join(home, ".aws"), filepath.Join(home, ".config")}
	if auth := os.Getenv("CODEX_HOME"); auth != "" {
		protected = append(protected, auth)
	}

	for _, path := range protected {
		resolved, err := resolveWriterPath(path)
		if err != nil {
			return "", err
		}

		if codexPathsOverlap(root, resolved) {
			return "", fmt.Errorf("runtime package overlaps protected root %q: %w", path, os.ErrPermission)
		}
	}

	return root, nil
}

func codexPathsOverlap(left, right string) bool {
	left, right = strings.ToLower(filepath.Clean(left)), strings.ToLower(filepath.Clean(right))

	return left == right || strings.HasPrefix(left, right+string(os.PathSeparator)) ||
		strings.HasPrefix(right, left+string(os.PathSeparator)) || left == string(os.PathSeparator)
}

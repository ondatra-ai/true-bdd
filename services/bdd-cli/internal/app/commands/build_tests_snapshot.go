package commands

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/disk"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/stepcoverage"
)

func (e *buildTestsExecution) snapshot() (map[string]string, error) {
	snapshots := make(map[string]string)

	for _, scenario := range e.scenarios {
		suite := stepcoverage.StepsDir(scenario.Path)
		if _, exists := snapshots[suite]; exists {
			continue
		}

		hash, err := stepsHash(filepath.Join(e.root, suite))
		if err != nil {
			return nil, err
		}

		snapshots[suite] = hash
	}

	return snapshots, nil
}

func stepsHash(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}

	if err != nil {
		return "", fmt.Errorf("read step definitions: %w", err)
	}

	hash := sha256.New()

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}

		source, err := disk.Read(filepath.Join(dir, entry.Name()))
		if err != nil {
			return "", fmt.Errorf("read step definition: %w", err)
		}

		_, _ = fmt.Fprintf(hash, "%s\x00%s\x00", entry.Name(), source)
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

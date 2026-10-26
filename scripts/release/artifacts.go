// Package release builds and rehearses release artifacts for this repository.
package release

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

var errRefused = errors.New("release refused")

// Metadata binds artifact names and the harness source to one exported commit.
type Metadata struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
}

func archiveNames(version string) []string {
	platforms := []string{"darwin_amd64", "darwin_arm64", "linux_amd64", "linux_arm64"}

	names := make([]string, 0, len(platforms))
	for _, platform := range platforms {
		names = append(names, "true-bdd_"+version+"_"+platform+".tar.gz")
	}

	return names
}

func readMetadata(artifacts string) (Metadata, error) {
	data, err := disk.Read(filepath.Join(artifacts, "release.json"))
	if err != nil {
		return Metadata{}, fmt.Errorf("reading artifact metadata: %w", err)
	}

	var metadata Metadata

	err = json.Unmarshal(data, &metadata)
	if err != nil {
		return metadata, fmt.Errorf("decoding artifact metadata: %w", err)
	}

	if !regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9.+_-]*$`).MatchString(metadata.Version) || metadata.Commit == "" {
		return metadata, fmt.Errorf("%w: invalid artifact metadata", errRefused)
	}

	return metadata, nil
}

func verifyArtifacts(artifacts string, metadata Metadata) error {
	data, err := disk.Read(filepath.Join(artifacts, "checksums.txt"))
	if err != nil {
		return fmt.Errorf("reading checksums: %w", err)
	}

	checksums := make(map[string]string)

	for line := range strings.SplitSeq(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}

		if len(fields) != 2 || checksums[fields[1]] != "" {
			return fmt.Errorf("%w: malformed checksums", errRefused)
		}

		checksums[fields[1]] = fields[0]
	}

	for _, name := range archiveNames(metadata.Version) {
		archive, readErr := disk.Read(filepath.Join(artifacts, name))
		if readErr != nil {
			return fmt.Errorf("reading archive %s: %w", name, readErr)
		}

		if fmt.Sprintf("%x", sha256.Sum256(archive)) != checksums[name] {
			return fmt.Errorf("%w: wrong checksum for %s", errRefused, name)
		}
	}

	return nil
}

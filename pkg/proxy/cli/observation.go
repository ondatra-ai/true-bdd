package cli

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

// PathSpec selects spawn metadata and explicitly requested regular-file contents.
type PathSpec struct {
	// Path names the observed location, expanding environment variables relative to the child's working directory.
	Path string `json:"path"`
	// Contents explicitly permits reading regular-file bytes.
	Contents bool `json:"contents,omitempty"`
	// ContentRoot confines content reads after symlink resolution; empty permits the explicitly named regular file.
	ContentRoot string `json:"content_root,omitempty"`
}

// Spawn is what a watched CLI received before it ran, independent of its recording mode.
type Spawn struct {
	// Binary names the watched dependency.
	Binary string `json:"binary"`
	// Args contains the exact invocation arguments.
	Args []string `json:"args"`
	// Environment records the environment passed to the watched process.
	Environment map[string]string `json:"environment"`
	// Paths maps requested locations to their spawn observations.
	Paths map[string]PathObservation `json:"paths"`
}

// PathObservation describes a requested path at spawn, omitting symlink and unrequested file contents.
type PathObservation struct {
	// Path is the requested location after environment expansion.
	Path string `json:"path"`
	// Canonical is the symlink-resolved location.
	Canonical string `json:"canonical"`
	// Mode contains only the observed filesystem permission bits.
	Mode uint32 `json:"mode"`
	// Type identifies the observed filesystem entry kind.
	Type string `json:"type"`
	// Symlink reports whether the requested entry is a symbolic link.
	Symlink bool `json:"symlink"`
	// Entries lists the names in an observed directory.
	Entries []string `json:"entries,omitempty"`
	// Content holds explicitly requested regular-file bytes and is absent for symlinks.
	Content []byte `json:"content,omitempty"`
	// Error records why the requested observation failed.
	Error string `json:"error,omitempty"`
}

// Observe requests spawn evidence for binary on the next run, reading only regular files whose Contents flag is true.
func Observe(run interface{ SetEnv(entries ...string) }, binary string, paths []PathSpec) error {
	data, err := json.Marshal(struct {
		Binary string     `json:"binary"`
		Paths  []PathSpec `json:"paths"`
	}{Binary: binary, Paths: paths})
	if err != nil {
		return fmt.Errorf("encode spawn observation request: %w", err)
	}

	run.SetEnv("TRUE_BDD_AIPROXY_OBSERVE=" + string(data))

	return nil
}

// Spawns reads the complete spawn observations retained by this run's stand-ins.
func Spawns(run interface{ ObservationDirectory() string }) ([]Spawn, error) {
	files, err := filepath.Glob(filepath.Join(run.ObservationDirectory(), "spawn-*.json"))
	if err != nil {
		return nil, fmt.Errorf("find spawn observations: %w", err)
	}

	spawns := make([]Spawn, 0, len(files))
	for _, file := range files {
		data, readErr := disk.Read(file)
		if readErr != nil {
			return nil, fmt.Errorf("read spawn observation: %w", readErr)
		}

		var spawn Spawn

		err = json.Unmarshal(data, &spawn)
		if err != nil {
			return nil, fmt.Errorf("decode spawn observation: %w", err)
		}

		spawns = append(spawns, spawn)
	}

	return spawns, nil
}

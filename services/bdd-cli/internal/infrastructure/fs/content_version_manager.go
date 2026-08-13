package fs

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

const contentFilePermissions = 0o644

// ContentVersionManager manages versioned copies of raw content in tmp directory.
// Each operation creates a new version (v01, v02, v03...) for audit trail.
type ContentVersionManager struct {
	runDir   *RunDirectory
	id       string
	prefix   string // e.g., "story" or "test"
	currentV int
}

// NewContentVersionManager creates a new version manager for raw content.
func NewContentVersionManager(runDir *RunDirectory, id, prefix string) *ContentVersionManager {
	return &ContentVersionManager{
		runDir:   runDir,
		id:       id,
		prefix:   prefix,
		currentV: 0,
	}
}

// SaveInitialVersion saves the original content as v01.
func (m *ContentVersionManager) SaveInitialVersion(data []byte) error {
	m.currentV = 1

	return m.saveVersion(data)
}

// SaveNextVersion increments version and saves the content.
func (m *ContentVersionManager) SaveNextVersion(data []byte) (string, error) {
	m.currentV++

	err := m.saveVersion(data)
	if err != nil {
		return "", err
	}

	return m.GetLatestPath(), nil
}

// LoadLatest loads the most recent version as raw bytes.
func (m *ContentVersionManager) LoadLatest() ([]byte, error) {
	path := m.GetLatestPath()

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read version file %s: %w", path, err)
	}

	slog.Info("Loaded content version", "path", path, "version", m.currentV)

	return data, nil
}

// GetLatestPath returns the file path for the current version.
func (m *ContentVersionManager) GetLatestPath() string {
	return m.getVersionPath(m.currentV)
}

// GetCurrentVersion returns the current version number.
func (m *ContentVersionManager) GetCurrentVersion() int {
	return m.currentV
}

func (m *ContentVersionManager) saveVersion(data []byte) error {
	path := m.GetLatestPath()

	err := os.WriteFile(path, data, contentFilePermissions)
	if err != nil {
		return fmt.Errorf("failed to write version file %s: %w", path, err)
	}

	slog.Info("Saved content version", "path", path, "version", m.currentV)

	return nil
}

func (m *ContentVersionManager) getVersionPath(version int) string {
	filename := fmt.Sprintf("%s-%s-v%02d.yaml", m.prefix, m.id, version)

	return filepath.Join(m.runDir.GetTmpOutPath(), filename)
}

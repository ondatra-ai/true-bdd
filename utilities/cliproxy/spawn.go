package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

type observedPath struct {
	Path      string   `json:"path"`
	Canonical string   `json:"canonical"`
	Mode      uint32   `json:"mode"`
	Type      string   `json:"type"`
	Symlink   bool     `json:"symlink"`
	Entries   []string `json:"entries,omitempty"`
	Content   []byte   `json:"content,omitempty"`
	Error     string   `json:"error,omitempty"`
}

type watchPath struct {
	Path        string `json:"path"`
	Contents    bool   `json:"contents,omitempty"`
	ContentRoot string `json:"content_root,omitempty"`
}

func recordSpawn(cfg config, name string, argv []string) error {
	encoded := os.Getenv("TRUE_BDD_AIPROXY_OBSERVE")
	if encoded == "" {
		return nil
	}

	var watch struct {
		Binary string      `json:"binary"`
		Paths  []watchPath `json:"paths"`
	}

	err := json.Unmarshal([]byte(encoded), &watch)
	if err != nil {
		return fmt.Errorf("decode spawn observation request: %w", err)
	}

	if watch.Binary != name {
		return nil
	}

	record := struct {
		Binary      string                  `json:"binary"`
		Args        []string                `json:"args"`
		Environment map[string]string       `json:"environment"`
		Paths       map[string]observedPath `json:"paths"`
	}{Binary: name, Args: argv, Environment: map[string]string{}, Paths: map[string]observedPath{}}

	for _, entry := range os.Environ() {
		key, value, _ := strings.Cut(entry, "=")
		record.Environment[key] = value
	}

	for _, path := range watch.Paths {
		record.Paths[path.Path] = observePath(path)
	}

	index, err := nextCallIndex(cfg.StateDir, "spawn-"+name)
	if err != nil {
		return err
	}

	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("encode spawn observation: %w", err)
	}

	file := filepath.Join(cfg.StateDir, fmt.Sprintf("spawn-%s-%06d.json", name, index))

	err = disk.Write(file, data, disk.Private)
	if err != nil {
		return fmt.Errorf("write spawn observation: %w", err)
	}

	return nil
}

func observePath(spec watchPath) observedPath {
	path, err := filepath.Abs(os.ExpandEnv(spec.Path))

	result := observedPath{Path: path}
	if err != nil {
		result.Error = err.Error()

		return result
	}

	info, err := os.Lstat(path)
	if err != nil {
		result.Error = err.Error()
		result.Type = "missing"

		return result
	}

	result.Mode = uint32(info.Mode().Perm())
	result.Symlink = info.Mode()&os.ModeSymlink != 0

	result.Canonical, _ = filepath.EvalSymlinks(path)
	switch {
	case result.Symlink:
		result.Type = "symlink"
	case info.IsDir():
		result.Type = "directory"
		result.Entries, result.Error = observedEntries(path)
	case info.Mode().IsRegular():
		result.Type = "file"
		if spec.Contents {
			result.Content, result.Error = observedContent(path, result.Canonical, spec.ContentRoot)
		}
	default:
		result.Type = "other"
	}

	return result
}

func observedEntries(path string) ([]string, string) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err.Error()
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}

	return names, ""
}

func withinContentRoot(canonical, root string) bool {
	resolved, err := filepath.EvalSymlinks(os.ExpandEnv(root))
	if err != nil {
		return false
	}

	relative, err := filepath.Rel(resolved, canonical)

	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func observedContent(path, canonical, root string) ([]byte, string) {
	if root != "" && !withinContentRoot(canonical, root) {
		return nil, "content path is outside the declared content root"
	}

	content, err := disk.Read(path)
	if err != nil {
		return nil, err.Error()
	}

	return content, ""
}

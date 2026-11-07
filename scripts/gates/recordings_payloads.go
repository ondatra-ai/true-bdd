package gates

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/ondatra-ai/true-bdd/pkg/cli/git"
	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

var errRecordingPayload = errors.New("recording fs-diff payload is not a git-tracked file; " +
	"re-record and stage every payload")

func checkRecordingPayloads(files []string) error {
	paths, err := git.IndexFiles()
	if err != nil {
		return fmt.Errorf("read recording payload index: %w", err)
	}

	tracked := make(map[string]bool, len(paths))
	for _, path := range paths {
		tracked[path] = true
	}

	for _, path := range files {
		if filepath.Base(path) != "meta.json" {
			continue
		}

		err = checkCassettePayloads(path, tracked)
		if err != nil {
			return err
		}
	}

	return nil
}

func checkCassettePayloads(path string, tracked map[string]bool) error {
	raw, err := disk.Read(path)
	if err != nil {
		return fmt.Errorf("read cassette %s: %w", path, err)
	}

	var manifest struct {
		FSDiff struct {
			Created  []string `json:"created"`
			Modified []string `json:"modified"`
		} `json:"fsdiff"`
	}

	err = json.Unmarshal(raw, &manifest)
	if err != nil {
		return fmt.Errorf("decode cassette %s: %w", path, err)
	}

	for _, name := range append(manifest.FSDiff.Created, manifest.FSDiff.Modified...) {
		payload := filepath.ToSlash(filepath.Join(filepath.Dir(path), "fsdiff/after", name))
		if !tracked[payload] {
			return fmt.Errorf("%w: %s names %s", errRecordingPayload, path, payload)
		}
	}

	return nil
}

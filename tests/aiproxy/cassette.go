package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ondatra-ai/true-bdd/tests/internal/fstree"
)

const metaSchemaVersion = 1

// Cassette file names. stdout/stderr hold the recorded streams
// (stream-json for claude, plain text for crush) with run-volatile
// paths NORMALIZED ({{CWD}}, {{RUN_DIR}}) — replay denormalizes them
// onto the live run before emitting, because the engine parses result
// files out of the response by their per-run tmp path. meta.json is
// written LAST via rename — its presence marks the cassette finalized,
// the same convention harness.json uses.
const (
	metaFile   = "meta.json"
	stdinFile  = "stdin"
	stdoutFile = "stdout"
	stderrFile = "stderr"
	fsdiffDir  = "fsdiff"
	afterDir   = "after"
)

// meta is the cassette manifest for one recorded call.
type meta struct {
	Schema      int        `json:"schema"`
	Binary      string     `json:"binary"`
	Argv        []string   `json:"argv"` // normalized ({{CWD}}, {{RUN_DIR}})
	ExitCode    int        `json:"exit_code"`
	DurationMS  int64      `json:"duration_ms"`
	RequestHash string     `json:"request_hash"`
	FSDiff      fsDiffMeta `json:"fsdiff"`
	RecordedAt  string     `json:"recorded_at"`
}

// fsDiffMeta lists the working-tree paths the call created, modified,
// or deleted. Contents for created/modified live under fsdiff/after/.
type fsDiffMeta struct {
	Created  []string `json:"created"`
	Modified []string `json:"modified"`
	Deleted  []string `json:"deleted"`
}

// cassetteName is the per-call directory name: <binary>-<NNN>.
func cassetteName(name string, idx int) string {
	return fmt.Sprintf("%s-%03d", name, idx)
}

func readMeta(dir string) (*meta, error) {
	raw, err := os.ReadFile(filepath.Join(dir, metaFile))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", metaFile, err)
	}

	var decoded meta

	err = json.Unmarshal(raw, &decoded)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", metaFile, err)
	}

	return &decoded, nil
}

// writeCassette persists one recorded call. Streams and fs-diff first,
// meta.json last (write-then-rename), so an interrupted recording
// leaves a dir without meta.json — ignored, never half-replayed.
func writeCassette(dir string, manifest *meta, stdin, stdout, stderr []byte, changes []fstree.Change) error {
	err := os.RemoveAll(dir)
	if err != nil {
		return fmt.Errorf("clean cassette dir %s: %w", dir, err)
	}

	err = os.MkdirAll(dir, dirPerm)
	if err != nil {
		return fmt.Errorf("create cassette dir %s: %w", dir, err)
	}

	err = writeStreams(dir, stdin, stdout, stderr)
	if err != nil {
		return err
	}

	manifest.FSDiff, err = writeFSDiff(dir, changes)
	if err != nil {
		return err
	}

	return writeMeta(dir, manifest)
}

func writeStreams(dir string, stdin, stdout, stderr []byte) error {
	files := map[string][]byte{stdinFile: stdin, stdoutFile: stdout}

	// stderr only when non-empty — most calls have none, and its
	// absence keeps cassette dirs reviewable at a glance.
	if len(stderr) > 0 {
		files[stderrFile] = stderr
	}

	for name, data := range files {
		err := os.WriteFile(filepath.Join(dir, name), data, filePerm)
		if err != nil {
			return fmt.Errorf("write cassette %s: %w", name, err)
		}
	}

	return nil
}

// writeFSDiff stores post-call contents for created/modified paths
// under fsdiff/after/ and returns the manifest lists.
func writeFSDiff(dir string, changes []fstree.Change) (fsDiffMeta, error) {
	diff := fsDiffMeta{Created: []string{}, Modified: []string{}, Deleted: []string{}}

	for _, change := range changes {
		switch change.Kind {
		case "created":
			diff.Created = append(diff.Created, change.Path)
		case "modified":
			diff.Modified = append(diff.Modified, change.Path)
		case "deleted":
			diff.Deleted = append(diff.Deleted, change.Path)

			continue
		}

		target := filepath.Join(dir, fsdiffDir, afterDir, change.Path)

		err := os.MkdirAll(filepath.Dir(target), dirPerm)
		if err != nil {
			return diff, fmt.Errorf("create fsdiff dir for %s: %w", change.Path, err)
		}

		err = os.WriteFile(target, change.After, filePerm)
		if err != nil {
			return diff, fmt.Errorf("write fsdiff %s: %w", change.Path, err)
		}
	}

	return diff, nil
}

func writeMeta(dir string, manifest *meta) error {
	encoded, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("encode %s: %w", metaFile, err)
	}

	tmpPath := filepath.Join(dir, metaFile+".tmp")

	err = os.WriteFile(tmpPath, append(encoded, '\n'), filePerm)
	if err != nil {
		return fmt.Errorf("write %s: %w", tmpPath, err)
	}

	err = os.Rename(tmpPath, filepath.Join(dir, metaFile))
	if err != nil {
		return fmt.Errorf("finalize %s: %w", metaFile, err)
	}

	return nil
}

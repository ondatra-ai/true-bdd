package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

func requestCassettes(root, binary string) ([]string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("list cassettes: %w", err)
	}

	var dirs []string

	for _, entry := range entries {
		suffix, ok := strings.CutPrefix(entry.Name(), binary+"-")

		_, parseErr := strconv.Atoi(suffix)
		if !ok || parseErr != nil || !entry.IsDir() {
			continue
		}

		dirs = append(dirs, filepath.Join(root, entry.Name()))
	}

	sort.Slice(dirs, func(left, right int) bool {
		first, _ := strconv.Atoi(strings.TrimPrefix(filepath.Base(dirs[left]), binary+"-"))
		second, _ := strconv.Atoi(strings.TrimPrefix(filepath.Base(dirs[right]), binary+"-"))

		return first < second
	})

	return dirs, nil
}

func requestSize(dirs []string, cwd string) (int, error) {
	want := 0

	for _, dir := range dirs {
		data, err := disk.Read(filepath.Join(dir, stdinFile))
		if err != nil {
			return 0, fmt.Errorf("read recorded stdin: %w", err)
		}

		live, err := denormalize(string(data), cwd, findCurrentRunDir(cwd))
		if err != nil {
			return 0, err
		}

		want = max(want, len(live))
	}

	return want, nil
}

// Identical requests consume occurrences in recorded-number order. The claim
// and consumed set update share one Hold across all replaying processes.
func claimRequest(cfg config, binary string, argv []string, input []byte, cwd string) (string, *meta, error) {
	dirs, err := requestCassettes(cfg.Cassettes, binary)
	if err != nil {
		return "", nil, err
	}

	var (
		selected string
		manifest *meta
	)

	err = disk.Update(filepath.Join(cfg.StateDir, "consumed-"+binary), disk.Shared, func(before []byte) ([]byte, error) {
		var consumed []string
		if len(before) > 0 {
			decodeErr := json.Unmarshal(before, &consumed)
			if decodeErr != nil {
				return nil, fmt.Errorf("read consumed cassettes: %w", decodeErr)
			}
		}

		for _, dir := range dirs {
			if slices.Contains(consumed, filepath.Base(dir)) {
				continue
			}

			candidate, readErr := readMeta(dir)
			if readErr != nil {
				return nil, readErr
			}

			args, stdin := canonicalGoRequest(argv, input, candidate.NormalizeGoDurations)
			if requestHash(args, stdin, cwd) != candidate.RequestHash {
				continue
			}

			selected, manifest = dir, candidate

			encoded, encodeErr := json.Marshal(append(consumed, filepath.Base(dir)))
			if encodeErr != nil {
				return nil, fmt.Errorf("encode consumed cassettes: %w", encodeErr)
			}

			return encoded, nil
		}

		return nil, fmt.Errorf("%w (%s, no unconsumed request match)", errCassetteStale, binary)
	})

	return selected, manifest, err
}

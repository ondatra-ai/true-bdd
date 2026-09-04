package main

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

// nextCallIndex returns this call's 1-based sequence number for the given
// binary name. disk.Update is what makes the read and the write one step, so
// two proxies racing for the same binary cannot hand out the same index.
func nextCallIndex(stateDir, name string) (int, error) {
	path := filepath.Join(stateDir, "cursor-"+name)

	next := 0

	err := disk.Update(path, disk.Shared, func(before []byte) ([]byte, error) {
		next = 1

		trimmed := strings.TrimSpace(string(before))
		if trimmed != "" {
			parsed, parseErr := strconv.Atoi(trimmed)
			if parseErr != nil {
				return nil, fmt.Errorf("parse cursor %s (%q): %w", path, trimmed, parseErr)
			}

			next = parsed + 1
		}

		return []byte(strconv.Itoa(next)), nil
	})
	if err != nil {
		return 0, err
	}

	return next, nil
}

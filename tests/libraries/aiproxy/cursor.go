package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// nextCallIndex returns this call's 1-based sequence number for the given
// binary name, atomically incrementing a flock-guarded cursor file under
// the state dir.
func nextCallIndex(stateDir, name string) (int, error) {
	err := os.MkdirAll(stateDir, dirPerm)
	if err != nil {
		return 0, fmt.Errorf("create state dir: %w", err)
	}

	path := filepath.Join(stateDir, "cursor-"+name)

	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, filePerm)
	if err != nil {
		return 0, fmt.Errorf("open cursor %s: %w", path, err)
	}

	// Close releases the flock together with the descriptor.
	defer func() { _ = file.Close() }()

	err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX)
	if err != nil {
		return 0, fmt.Errorf("lock cursor %s: %w", path, err)
	}

	raw, err := io.ReadAll(file)
	if err != nil {
		return 0, fmt.Errorf("read cursor %s: %w", path, err)
	}

	next := 1

	trimmed := strings.TrimSpace(string(raw))
	if trimmed != "" {
		parsed, parseErr := strconv.Atoi(trimmed)
		if parseErr != nil {
			return 0, fmt.Errorf("parse cursor %s (%q): %w", path, trimmed, parseErr)
		}

		next = parsed + 1
	}

	err = file.Truncate(0)
	if err != nil {
		return 0, fmt.Errorf("truncate cursor %s: %w", path, err)
	}

	_, err = file.WriteAt([]byte(strconv.Itoa(next)), 0)
	if err != nil {
		return 0, fmt.Errorf("write cursor %s: %w", path, err)
	}

	return next, nil
}

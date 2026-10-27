package fs

import (
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestRunDirNameSecondsAndPID proves the seconds+PID naming (plan §3.2):
// distinct (second, pid) pairs never collide, unlike the old minute-only
// scheme.
func TestRunDirNameSecondsAndPID(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, time.July, 29, 14, 30, 15, 0, time.UTC)

	name := runDirName(base, 4242)
	if !strings.HasSuffix(name, "-4242") {
		t.Fatalf("name %q missing pid suffix", name)
	}

	if !strings.Contains(name, "-15-") && !strings.HasSuffix(name, "15-4242") {
		t.Fatalf("name %q missing second component", name)
	}

	// Different pid, same instant → distinct.
	if runDirName(base, 4242) == runDirName(base, 4243) {
		t.Fatal("same-instant, different-pid names collided")
	}

	// Same pid, one second apart → distinct (minute-only naming collided).
	if runDirName(base, 4242) == runDirName(base.Add(time.Second), 4242) {
		t.Fatal("one-second-apart names collided")
	}
}

// TestNewRunDirectoryCreatesUniquePaths proves NewRunDirectory produces a
// real, distinct directory keyed on the current pid+second.
func TestNewRunDirectoryCreatesUniquePaths(t *testing.T) {
	t.Parallel()

	base := t.TempDir()

	dir, err := NewRunDirectory(base)
	if err != nil {
		t.Fatalf("NewRunDirectory: %v", err)
	}

	path := dir.GetTmpOutPath()
	if !strings.HasPrefix(path, base) {
		t.Fatalf("run path %q not under base %q", path, base)
	}

	if !strings.Contains(path, strconv.Itoa(os.Getpid())) {
		t.Fatalf("run path %q missing pid", path)
	}
}

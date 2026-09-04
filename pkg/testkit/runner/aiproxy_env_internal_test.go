package runner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A caller's own shim leads its PATH; every other caller's is scrubbed
// out of it, because the child inherits this environment.
func TestAIProxyEnvScrubsTheOtherCallersShim(t *testing.T) {
	target, tests := t.TempDir(), t.TempDir()
	t.Setenv("PATH", tests+string(os.PathListSeparator)+"/usr/bin")

	entries := AIProxyEnv(ProxyModeReplay, target, "/shelf", "/state",
		ShimDirs{Target: target, Tests: tests}.All())

	path := valueOf(t, entries, "PATH")
	dirs := filepath.SplitList(path)

	if dirs[0] != target {
		t.Errorf("PATH leads with %q, want the caller's own shim %q", dirs[0], target)
	}

	for _, dir := range dirs {
		if dir == tests {
			t.Errorf("PATH still carries the other caller's shim %q: %s", tests, path)
		}
	}
}

// Every shim dir is named, so a recording shim skips all of them rather
// than finding another shim and exec'ing it.
func TestAIProxyEnvNamesEveryShim(t *testing.T) {
	target, tests := t.TempDir(), t.TempDir()
	t.Setenv("PATH", "/usr/bin")

	entries := AIProxyEnv(ProxyModeRecord, target, "/shelf", "/state",
		ShimDirs{Target: target, Tests: tests}.All())

	known := valueOf(t, entries, EnvKnownShims)
	for _, want := range []string{target, tests} {
		if !strings.Contains(known, want) {
			t.Errorf("%s = %q, missing %q", EnvKnownShims, known, want)
		}
	}
}

// Arming is scoped: what the judge sets must not survive into the next
// fixture's engine subprocess.
func TestArmProcessRestores(t *testing.T) {
	t.Setenv("TRUE_BDD_AIPROXY_MODE", "before")

	_, absent := os.LookupEnv("TRUE_BDD_AIPROXY_CASSETTES")
	if absent {
		t.Skip("the environment already carries a cassettes dir")
	}

	restore := ArmProcess([]string{
		"TRUE_BDD_AIPROXY_MODE=replay",
		"TRUE_BDD_AIPROXY_CASSETTES=/shelf",
	})

	if got := os.Getenv("TRUE_BDD_AIPROXY_MODE"); got != "replay" {
		t.Fatalf("armed mode = %q, want replay", got)
	}

	restore()

	if got := os.Getenv("TRUE_BDD_AIPROXY_MODE"); got != "before" {
		t.Errorf("restored mode = %q, want before", got)
	}

	if _, set := os.LookupEnv("TRUE_BDD_AIPROXY_CASSETTES"); set {
		t.Error("a variable that was absent before arming is still set")
	}
}

func valueOf(t *testing.T, entries []string, key string) string {
	t.Helper()

	for _, entry := range entries {
		name, value, _ := strings.Cut(entry, "=")
		if name == key {
			return value
		}
	}

	t.Fatalf("%s missing from %v", key, entries)

	return ""
}

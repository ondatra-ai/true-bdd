package runner

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// cassetteName matches the shim's `<binary>-<NNN>` directory naming.
var cassetteNamePattern = regexp.MustCompile(`^([a-z]+)-(\d{3})$`)

// CheckCassettesConsumed reports whether the run used every recorded
// call, per binary.
//
// This is the one divergence the per-call request hash cannot catch. The
// hash fires when a call arrives that does not match its cassette; it
// says nothing about a call that never arrives. An engine that stops one
// turn early — a loop bound that changed, a validation that now
// short-circuits — consumes a prefix of the cassettes, matches every one
// of them, and finishes quietly. Counting what was left on the shelf is
// what turns that silence into a failure.
//
// The shim's cursor file holds the index of the last call it served, so
// the cursor and the cassette count must agree exactly.
func CheckCassettesConsumed(cassettesDir, stateDir string) []string {
	recorded, err := countCassettes(cassettesDir)
	if err != nil {
		return []string{"cassettes: " + err.Error()}
	}

	var failures []string

	for _, binary := range sortedKeys(recorded) {
		served := readCursor(stateDir, binary)

		if served == recorded[binary] {
			continue
		}

		failures = append(failures, fmt.Sprintf(
			"cassettes: the run made %d %s call(s) but %d were recorded — "+
				"the engine diverged from the recording without tripping a request hash",
			served, binary, recorded[binary]))
	}

	return failures
}

// countCassettes tallies the recorded calls per binary. Entries that are
// not cassettes (golden.json, a keep file) are ignored by construction.
func countCassettes(dir string) (map[string]int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read cassettes dir: %w", err)
	}

	counts := map[string]int{}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		match := cassetteNamePattern.FindStringSubmatch(entry.Name())
		if match == nil {
			continue
		}

		counts[match[1]]++
	}

	return counts, nil
}

// readCursor reads how many calls the shim served for one binary. A
// missing cursor means the binary was never spawned, which is zero
// served — and a real failure when cassettes exist for it.
func readCursor(stateDir, binary string) int {
	blob, err := os.ReadFile(filepath.Join(stateDir, "cursor-"+binary))
	if err != nil {
		return 0
	}

	served, err := strconv.Atoi(strings.TrimSpace(string(blob)))
	if err != nil {
		return 0
	}

	return served
}

func sortedKeys(counts map[string]int) []string {
	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	return keys
}

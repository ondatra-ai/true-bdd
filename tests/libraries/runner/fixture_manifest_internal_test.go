package runner

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// writeManifest drops a fixture.yaml in a temp dir and returns its path.
func writeManifest(t *testing.T, body string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "fixture.yaml")

	err := os.WriteFile(path, []byte(body), filePerm)
	if err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	return path
}

// A fixture that declares a timeout is asking for MORE budget than the
// suite's deliberately tight default. Silently falling back to that
// default on a typo would reintroduce exactly the failure the field
// exists to prevent — a correct fixture killed mid-`docker build` — so
// every non-blank value has to be a positive duration or a load error.
func TestParseFixtureTimeout(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		raw   string
		want  time.Duration
		valid bool
	}{
		{name: "absent means the suite default", raw: "", want: 0, valid: true},
		{name: "whitespace means the suite default", raw: "   ", want: 0, valid: true},
		{name: "minutes", raw: "15m", want: 15 * time.Minute, valid: true},
		{name: "seconds", raw: "90s", want: 90 * time.Second, valid: true},
		{name: "compound", raw: "1h30m", want: 90 * time.Minute, valid: true},
		{name: "surrounding whitespace is trimmed", raw: " 15m ", want: 15 * time.Minute, valid: true},
		{name: "zero is not a budget", raw: "0s"},
		{name: "bare zero", raw: "0"},
		{name: "negative", raw: "-5m"},
		{name: "unitless number", raw: "15"},
		{name: "prose", raw: "fifteen minutes"},
		{name: "unknown unit", raw: "15x"},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseFixtureTimeout(testCase.raw)

			if !testCase.valid {
				if !errors.Is(err, ErrTimeoutInvalid) {
					t.Fatalf("parseFixtureTimeout(%q) error = %v, want ErrTimeoutInvalid",
						testCase.raw, err)
				}

				return
			}

			if err != nil {
				t.Fatalf("parseFixtureTimeout(%q) = %v, want nil", testCase.raw, err)
			}

			if got != testCase.want {
				t.Errorf("parseFixtureTimeout(%q) = %v, want %v", testCase.raw, got, testCase.want)
			}
		})
	}
}

// A rejected timeout has to fail the whole manifest load, not just the
// helper — otherwise the fixture runs on the default budget and the
// typo is invisible until something times out.
func TestLoadFixtureManifestRejectsBadTimeout(t *testing.T) {
	t.Parallel()

	path := writeManifest(t, `
input: input
timeout: "-1m"
expected:
  judge: |
    anything
`)

	_, err := LoadFixtureManifest(path)
	if !errors.Is(err, ErrTimeoutInvalid) {
		t.Fatalf("LoadFixtureManifest() error = %v, want ErrTimeoutInvalid", err)
	}
}

// A manifest with no timeout must still load, leaving the field zero so
// the caller applies the suite default.
func TestLoadFixtureManifestWithoutTimeout(t *testing.T) {
	t.Parallel()

	path := writeManifest(t, `
input: input
expected:
  judge: |
    anything
`)

	manifest, err := LoadFixtureManifest(path)
	if err != nil {
		t.Fatalf("LoadFixtureManifest() = %v, want nil", err)
	}

	if manifest.Timeout != "" {
		t.Errorf("Timeout = %q, want empty", manifest.Timeout)
	}
}

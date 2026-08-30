package alint_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ondatra-ai/true-bdd/pkg/cli/alint"
)

// A rule that fires only under a scope, so a run's env is visible in its report.
const scopedConfig = `version: 1
rules:
  - id: scoped-probe
    kind: command
    level: error
    when: "env.TRUEBDD_SCOPE"
    paths: "**/*.go"
    scope_filter:
      include_manifest_paths:
        source: "{{env.TRUEBDD_SCOPE | default('tmp/none.txt')}}"
        extract: { lines: {} }
    command: ["sh", "-c", "echo saw {path}; exit 1"]
`

// Fix names one file and alint must see that file and no other — which proves
// the manifest, its ../ escape and the scope variable all agree.
func TestFixScopesToTheNamedPaths(t *testing.T) {
	root := fakeRepo(t)

	report, err := alint.Fix([]string{"a.go"})
	if err != nil {
		t.Fatalf("Fix: %v", err)
	}

	if len(report.Findings) != 1 || report.Findings[0].Path != "a.go" {
		t.Fatalf("scope did not bind: got %+v, want exactly a.go", report.Findings)
	}

	if report.Findings[0].Status != alint.Unfixable {
		t.Errorf("status: got %q, want %q", report.Findings[0].Status, alint.Unfixable)
	}

	// The manifest is this process's alone, so nothing may survive the call.
	left, _ := os.ReadDir(filepath.Join(root, "tmp"))
	for _, entry := range left {
		t.Errorf("manifest left behind: %s", entry.Name())
	}
}

// The invariant ADR 0006 rests on: a checking run carries no scope, so the
// rules that fix cannot fire under it.
func TestCheckCarriesNoScope(t *testing.T) {
	fakeRepo(t)

	report, err := alint.Check()
	if err != nil {
		t.Fatalf("Check: %v", err)
	}

	if len(report.Findings) != 0 {
		t.Errorf("a check must not set %s: got %+v", alint.ScopeVar, report.Findings)
	}
}

// fakeRepo is a tree with the scoped rule and two Go files, made the working
// directory, so alint answers about it rather than about this repository.
func fakeRepo(t *testing.T) string {
	t.Helper()

	err := alint.Available()
	if err != nil {
		t.Skipf("alint is not on PATH: %v", err)
	}

	root := t.TempDir()
	write(t, filepath.Join(root, ".alint.yml"), scopedConfig)
	write(t, filepath.Join(root, "a.go"), "package x\n")
	write(t, filepath.Join(root, "b.go"), "package x\n")

	t.Chdir(root)

	return root
}

func write(t *testing.T, path, body string) {
	t.Helper()

	err := os.WriteFile(path, []byte(body), 0o600)
	if err != nil {
		t.Fatal(err)
	}
}

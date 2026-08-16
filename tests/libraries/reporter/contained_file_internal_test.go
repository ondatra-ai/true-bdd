package reporter

import (
	"os"
	"path/filepath"
	"testing"
)

// The ordinary case: a path the engine actually writes.
func TestContainedFileAcceptsRunPaths(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	write(t, dir, "true-bdd/checklists/us-apply.yaml", "sections: []\n")

	body, ok := ReadContained(dir, "true-bdd/checklists/us-apply.yaml")
	if !ok || body != "sections: []\n" {
		t.Errorf("read = %q ok=%v", body, ok)
	}
}

// A readable file with no bytes in it is present. Collapsing it into
// "absent" is how a truncated target silently loses its provenance
// reference — the one case where the emptiness is the finding.
func TestContainedFileKeepsEmptyFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	write(t, dir, "tmp/scenarios.yaml", "")

	body, ok := ReadContained(dir, "tmp/scenarios.yaml")
	if !ok || body != "" {
		t.Errorf("empty file: read = %q ok=%v, want present and empty", body, ok)
	}
}

// Shapes the engine never writes, all refused. The log is data on disk,
// so the report must not open whatever it names.
func TestContainedFileRefusesEscapes(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "secret.yaml")

	err := os.WriteFile(outside, []byte("secret\n"), 0o600)
	if err != nil {
		t.Fatalf("write outside: %v", err)
	}

	cases := []struct {
		name string
		rel  string
	}{
		{"empty", ""},
		{"absolute", outside},
		{"parent traversal", "../" + filepath.Base(filepath.Dir(outside)) + "/secret.yaml"},
		{"traversal mid-path", "true-bdd/../../secret.yaml"},
		{"does not exist", "true-bdd/missing.yaml"},
	}

	for _, testCase := range cases {
		if _, ok := ReadContained(dir, testCase.rel); ok {
			t.Errorf("%s: %q was accepted", testCase.name, testCase.rel)
		}
	}
}

// A lexical check passes a path whose every component sits inside the
// fixture while the open follows a link straight out of it. Both the
// final component and an intermediate directory can be that link.
func TestContainedFileRefusesSymlinkEscapes(t *testing.T) {
	t.Parallel()

	secrets := t.TempDir()

	err := os.WriteFile(filepath.Join(secrets, "secret.yaml"), []byte("secret\n"), 0o600)
	if err != nil {
		t.Fatalf("write secret: %v", err)
	}

	dir := t.TempDir()
	write(t, dir, "true-bdd/real.yaml", "sections: []\n")

	// Final component: <dir>/leak.yaml → <secrets>/secret.yaml
	err = os.Symlink(filepath.Join(secrets, "secret.yaml"), filepath.Join(dir, "leak.yaml"))
	if err != nil {
		t.Fatalf("symlink file: %v", err)
	}

	// Intermediate directory: <dir>/elsewhere → <secrets>
	err = os.Symlink(secrets, filepath.Join(dir, "elsewhere"))
	if err != nil {
		t.Fatalf("symlink dir: %v", err)
	}

	for _, rel := range []string{"leak.yaml", "elsewhere/secret.yaml"} {
		if body, ok := ReadContained(dir, rel); ok {
			t.Errorf("%q was accepted, read %q", rel, body)
		}
	}

	// A symlink that stays inside is still fine — the rule is
	// containment, not "no symlinks".
	err = os.Symlink(filepath.Join(dir, "true-bdd", "real.yaml"), filepath.Join(dir, "inside.yaml"))
	if err != nil {
		t.Fatalf("symlink inside: %v", err)
	}

	if _, ok := ReadContained(dir, "inside.yaml"); !ok {
		t.Error("a symlink that resolves inside the fixture must be readable")
	}
}

// The checklist loader is the caller that matters: a log naming a path
// outside the run must degrade to "no checklist", not read it.
func TestLoadChecklistDocRefusesEscapes(t *testing.T) {
	t.Parallel()

	outsideDir := t.TempDir()

	err := os.WriteFile(filepath.Join(outsideDir, "evil.yaml"),
		[]byte("sections:\n  - id: leaked\n    name: \"Leaked\"\n    validation_prompts:\n      - Q: \"x\"\n"), 0o600)
	if err != nil {
		t.Fatalf("write outside checklist: %v", err)
	}

	dir := t.TempDir()

	for _, rel := range []string{
		filepath.Join(outsideDir, "evil.yaml"),
		"../" + filepath.Base(outsideDir) + "/evil.yaml",
	} {
		if doc := loadChecklistDoc(dir, rel); doc.Loaded {
			t.Errorf("%q loaded %d prompt(s) from outside the fixture", rel, len(doc.Prompts))
		}
	}
}

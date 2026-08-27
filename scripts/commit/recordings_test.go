package commit_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ondatra-ai/true-bdd/scripts/commit"
)

// plant writes content to a temp file and returns the path, so each case
// asserts against a real file the sweep has to read.
func plant(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "cassette.json")

	err := os.WriteFile(path, []byte(content), 0o600)
	if err != nil {
		t.Fatalf("planting the file: %v", err)
	}

	return path
}

func hits(t *testing.T, report, content string) []string {
	t.Helper()

	sweep := commit.Sweep(report)
	if sweep == nil {
		t.Fatalf("no sweep reports %q; the table has %v", report, commit.Reports())
	}

	return sweep([]string{plant(t, content)})
}

// Not $HOME literally: a cassette recorded by someone else carries THEIR path.
func TestHomeDirectorySweepCatchesAnyUser(t *testing.T) {
	t.Parallel()

	const report = "a home directory path survived normalization"

	for name, content := range map[string]string{
		"macOS": `{"cwd":"/Users/someone-else/work/repo"}`,
		"linux": `{"cwd":"/home/other_dev/work/repo"}`,
	} {
		if found := hits(t, report, content); len(found) != 1 {
			t.Errorf("%s: a home path went unreported", name)
		}
	}

	if found := hits(t, report, `{"cwd":"{{CWD}}/work/repo"}`); len(found) != 0 {
		t.Errorf("a normalized path was reported: %v", found)
	}
}

func TestSessionInventorySweep(t *testing.T) {
	t.Parallel()

	const report = "session inventory survived sanitizing"

	for _, key := range []string{"mcp_servers", "slash_commands", "memory_paths", "apiKeySource"} {
		if found := hits(t, report, `{"`+key+`":[]}`); len(found) != 1 {
			t.Errorf("%q went unreported", key)
		}
	}

	if found := hits(t, report, `{"model":"claude","tools":[]}`); len(found) != 0 {
		t.Errorf("an ordinary cassette was reported: %v", found)
	}
}

func TestCredentialSweep(t *testing.T) {
	t.Parallel()

	const report = "credential-shaped string in a recording"

	for name, content := range map[string]string{
		"api key":     `{"k":"sk-abcdefghijklmnopqrstuv"}`,
		"github pat":  `{"k":"ghp_abcdefghijklmnopqrstuvwx"}`,
		"slack token": `{"k":"xoxb-1234567890abcdef"}`,
		"private key": `-----BEGIN RSA PRIVATE KEY-----`,
	} {
		if found := hits(t, report, content); len(found) != 1 {
			t.Errorf("%s went unreported", name)
		}
	}

	if found := hits(t, report, `{"note":"sk-short"}`); len(found) != 0 {
		t.Errorf("a short non-credential was reported: %v", found)
	}
}

// The addresses this project legitimately ships clear a file; a real one does not.
func TestEmailSweepAllowsOnlyTheShippedForms(t *testing.T) {
	t.Parallel()

	const report = "e-mail address in a recording"

	clean := "noreply@github.com\nuser@example.com\nnobody@example.org\n"
	if found := hits(t, report, clean); len(found) != 0 {
		t.Errorf("the shipped addresses were reported: %v", found)
	}

	if found := hits(t, report, "{\"author\":\"someone@real-domain.dev\"}\n"); len(found) != 1 {
		t.Errorf("a real address went unreported")
	}
}

// The committed recordings must be clean, which is the gate's whole job.
func TestCommittedRecordingsAreClean(t *testing.T) {
	t.Parallel()

	root := filepath.Join("..", "..")

	dirs, err := filepath.Glob(filepath.Join(root, commit.RecordingsGlob))
	if err != nil {
		t.Fatalf("globbing: %v", err)
	}

	var files []string

	for _, dir := range dirs {
		err = filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
			if err == nil && !entry.IsDir() {
				files = append(files, path)
			}

			return nil
		})
		if err != nil {
			t.Fatalf("walking %s: %v", dir, err)
		}
	}

	if len(files) == 0 {
		t.Skip("no recordings in this checkout")
	}

	for _, report := range commit.Reports() {
		if found := commit.Sweep(report)(files); len(found) != 0 {
			t.Errorf("%s: %v", report, found)
		}
	}
}

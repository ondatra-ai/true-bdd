package remote

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFixtureFile(t *testing.T, folder, relPath, content string) {
	t.Helper()

	abs := filepath.Join(folder, filepath.FromSlash(relPath))

	mkdirErr := os.MkdirAll(filepath.Dir(abs), 0o755)
	if mkdirErr != nil {
		t.Fatalf("mkdir: %v", mkdirErr)
	}

	writeErr := os.WriteFile(abs, []byte(content), 0o644)
	if writeErr != nil {
		t.Fatalf("write fixture: %v", writeErr)
	}
}

func newTestDocStore(t *testing.T) (*docStore, string) {
	t.Helper()

	folder := t.TempDir()
	writeFixtureFile(t, folder, archYAMLPath, "version: 1\nterms:\n  - name: A\n")
	writeFixtureFile(t, folder, prdYAMLPath, "title: t\n")
	writeFixtureFile(t, folder, featuresYAMLPath, "features:\n  - id: a\n    description: d\n")
	writeFixtureFile(t, folder, scenariosYAMLPath, "scenarios:\n  X-1:\n    description: d\n")
	writeFixtureFile(t, folder, "docs/prd/stories/60.1-a.yaml", "story:\n  id: \"60.1\"\n")

	return newDocStore(folder), folder
}

func TestDocTreeListsFixedDocsAndStoryFiles(t *testing.T) {
	store, _ := newTestDocStore(t)

	tree := store.tree()

	if len(tree.Docs) != 5 {
		t.Fatalf("expected 5 docs (4 fixed + 1 story), got %d: %+v", len(tree.Docs), tree.Docs)
	}

	found := false

	for _, node := range tree.Docs {
		if node.Path == "docs/prd/stories/60.1-a.yaml" {
			found = true

			if !node.Exists || node.Revision == docExistenceAbsent {
				t.Fatalf("story node should exist with a real revision: %+v", node)
			}
		}
	}

	if !found {
		t.Fatalf("story file missing from doc_tree: %+v", tree.Docs)
	}
}

func TestDocTreeReflectsNewStoryFileLive(t *testing.T) {
	store, folder := newTestDocStore(t)

	before := store.tree()
	if len(before.Docs) != 5 {
		t.Fatalf("expected 5 docs before, got %d", len(before.Docs))
	}

	writeFixtureFile(t, folder, "docs/prd/stories/60.2-b.yaml", "story:\n  id: \"60.2\"\n")

	after := store.tree()
	if len(after.Docs) != 6 {
		t.Fatalf("expected 6 docs after a new story file appears on disk, got %d", len(after.Docs))
	}
}

func TestDocReadRoundTrip(t *testing.T) {
	store, _ := newTestDocStore(t)

	result, err := store.read(archYAMLPath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	if !result.Exists || result.ParseStatus != parseStatusOK || result.Revision == docExistenceAbsent {
		t.Fatalf("unexpected read result: %+v", result)
	}
}

func TestDocReadMissingFile(t *testing.T) {
	store, _ := newTestDocStore(t)

	result, err := store.read("docs/prd/stories/60.9-missing.yaml")
	if err != nil {
		t.Fatalf("read of a not-yet-created (but allowed) story path must not error: %v", err)
	}

	if result.Exists || result.ParseStatus != parseStatusMissing || result.Revision != docExistenceAbsent {
		t.Fatalf("unexpected result for a missing file: %+v", result)
	}
}

func TestValidatePathRejectsTraversalAbsoluteNULAndWrongDirectory(t *testing.T) {
	store, _ := newTestDocStore(t)

	cases := []string{
		"../etc/passwd",
		"docs/../../etc/passwd",
		"/etc/passwd",
		"docs/architecture/architecture.yaml\x00.evil",
		"docs/prd/stories/../../../etc/passwd.yaml",
		"docs/prd/prd.yml",             // wrong extension
		"docs/prd/stories/sub/x.yaml",  // no subdirectories under stories/
		"docs/architecture/other.yaml", // similarly-named file, wrong directory
		"docs/scenarios.yaml.bak",      // not exactly the allowed name
	}

	for _, c := range cases {
		_, _, err := store.validatePath(c)
		if err == nil {
			t.Fatalf("validatePath(%q) should be rejected", c)
		}
	}
}

func TestValidatePathAcceptsExactAllowlistAndStoryGlob(t *testing.T) {
	store, _ := newTestDocStore(t)

	cases := []string{
		archYAMLPath,
		prdYAMLPath,
		featuresYAMLPath,
		scenariosYAMLPath,
		"docs/prd/stories/60.1-a.yaml",
		"docs/prd/stories/70.9-brand-new.yaml", // exclusive-creatable — does not yet exist
	}

	for _, c := range cases {
		_, _, err := store.validatePath(c)
		if err != nil {
			t.Fatalf("validatePath(%q) should be accepted: %v", c, err)
		}
	}
}

func TestValidatePathRejectsSymlinkEscape(t *testing.T) {
	store, folder := newTestDocStore(t)

	outside := t.TempDir()
	secretPath := filepath.Join(outside, "secret.yaml")

	writeErr := os.WriteFile(secretPath, []byte("leak: true\n"), 0o644)
	if writeErr != nil {
		t.Fatalf("write outside secret: %v", writeErr)
	}

	// Symlink the FINAL TARGET itself (not just a parent) to escape the root.
	linkPath := filepath.Join(folder, "docs", "prd", "stories", "60.5-evil.yaml")

	symlinkErr := os.Symlink(secretPath, linkPath)
	if symlinkErr != nil {
		t.Skipf("symlink not supported in this environment: %v", symlinkErr)
	}

	_, _, err := store.validatePath("docs/prd/stories/60.5-evil.yaml")
	if err == nil {
		t.Fatalf("a final-target symlink escaping the root must be rejected")
	}
}

func TestDocWriteHappyPathAndReceipt(t *testing.T) {
	store, folder := newTestDocStore(t)

	initial, err := store.read(archYAMLPath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	newContent := "version: 1\nterms:\n  - name: A\n  - name: B\n"
	outcome := store.write(docWritePayload{
		Path:         archYAMLPath,
		Content:      newContent,
		BaseRevision: initial.Revision,
		ClientToken:  "tok-1",
	})

	if outcome.kind != docWriteOK {
		t.Fatalf("expected ok, got %+v", outcome)
	}

	onDisk, readErr := os.ReadFile(filepath.Join(folder, archYAMLPath))
	if readErr != nil {
		t.Fatalf("read on disk: %v", readErr)
	}

	if string(onDisk) != newContent {
		t.Fatalf("on-disk content mismatch: %q", string(onDisk))
	}

	if outcome.receipt.Path != archYAMLPath {
		t.Fatalf("receipt path mismatch: %+v", outcome.receipt)
	}

	if outcome.receipt.ContentHash != revisionOf(true, []byte(newContent)) {
		t.Fatalf("receipt content_hash must match sha256 of the committed bytes: %+v", outcome.receipt)
	}
}

func TestDocWriteRejectsInvalidYAML(t *testing.T) {
	store, _ := newTestDocStore(t)

	initial, _ := store.read(archYAMLPath)
	outcome := store.write(docWritePayload{
		Path:         archYAMLPath,
		Content:      "broken: [",
		BaseRevision: initial.Revision,
	})

	if outcome.kind != docWriteInvalidYAML {
		t.Fatalf("expected invalid_yaml, got %+v", outcome)
	}
}

func TestDocWriteRejectsMultiDocumentStream(t *testing.T) {
	store, folder := newTestDocStore(t)

	initial, _ := store.read(archYAMLPath)
	before, _ := os.ReadFile(filepath.Join(folder, archYAMLPath))

	// A valid first document followed by a `---` separator and a second document:
	// Go's lenient decoder would accept the first, but the browser's strict
	// single-document YAML.parse() rejects it — the CLI gate must too, or it would
	// commit content the workspace immediately considers invalid.
	outcome := store.write(docWritePayload{
		Path:         archYAMLPath,
		Content:      "version: 1\n---\nrogue: 2\n",
		BaseRevision: initial.Revision,
	})

	if outcome.kind != docWriteInvalidYAML {
		t.Fatalf("expected invalid_yaml for a multi-document stream, got %+v", outcome)
	}

	after, _ := os.ReadFile(filepath.Join(folder, archYAMLPath))
	if string(after) != string(before) {
		t.Fatalf("a rejected multi-document write must not touch disk")
	}
}

func TestIsValidYAMLSingleDocumentAndEmptyStillValid(t *testing.T) {
	valid := []string{
		"version: 1\nterms:\n  - name: A\n",
		"---\nversion: 1\n", // a leading single-document marker is fine
		"# comment only\n",   // no document ⇒ valid (matches YAML.parse("") ⇒ null)
		"",
		"\n\n",
	}
	for _, c := range valid {
		if !isValidYAML([]byte(c)) {
			t.Fatalf("expected valid single/empty document: %q", c)
		}
	}

	invalid := []string{
		"a: 1\n---\nb: 2\n", // two documents
		"a: 1\n---\n",       // trailing separator ⇒ a second (empty) document
		"broken: [",         // genuinely malformed
	}
	for _, c := range invalid {
		if isValidYAML([]byte(c)) {
			t.Fatalf("expected rejection: %q", c)
		}
	}
}

func TestDocWriteRejectsStaleBaseRevision(t *testing.T) {
	store, _ := newTestDocStore(t)

	outcome := store.write(docWritePayload{
		Path:         archYAMLPath,
		Content:      "version: 1\nterms: []\n",
		BaseRevision: "stale-revision",
	})

	if outcome.kind != docWriteConflict {
		t.Fatalf("expected conflict for a stale base_revision, got %+v", outcome)
	}
}

func TestDocWriteExclusiveCreateNewStoryFile(t *testing.T) {
	store, folder := newTestDocStore(t)

	outcome := store.write(docWritePayload{
		Path:         "docs/prd/stories/70.1-new.yaml",
		Content:      "story:\n  id: \"70.1\"\n",
		BaseRevision: docExistenceAbsent,
	})

	if outcome.kind != docWriteOK {
		t.Fatalf("expected exclusive-create to succeed, got %+v", outcome)
	}

	_, statErr := os.Stat(filepath.Join(folder, "docs/prd/stories/70.1-new.yaml"))
	if statErr != nil {
		t.Fatalf("new story file must exist on disk: %v", statErr)
	}

	// A second create attempt with the SAME absent base_revision must now
	// conflict (the file exists).
	second := store.write(docWritePayload{
		Path:         "docs/prd/stories/70.1-new.yaml",
		Content:      "story:\n  id: \"70.1-changed\"\n",
		BaseRevision: docExistenceAbsent,
	})

	if second.kind != docWriteConflict {
		t.Fatalf("expected conflict on double-create, got %+v", second)
	}
}

func TestDocWriteIdempotentTokenReplayVsDifferingArgs(t *testing.T) {
	store, _ := newTestDocStore(t)

	initial, _ := store.read(archYAMLPath)
	payload := docWritePayload{
		Path:         archYAMLPath,
		Content:      "version: 1\nterms:\n  - name: A\n  - name: C\n",
		BaseRevision: initial.Revision,
		ClientToken:  "dup-token",
	}

	first := store.write(payload)
	if first.kind != docWriteOK {
		t.Fatalf("first write should succeed: %+v", first)
	}

	// Exact replay (same token, same args) → the SAME receipt, not a new commit.
	replay := store.write(payload)
	if replay.kind != docWriteOK || replay.receipt != first.receipt {
		t.Fatalf("identical replay must return the original receipt: %+v vs %+v", replay, first)
	}

	// Same token, DIFFERENT args → 409 conflict.
	payload.Content = "version: 1\nterms:\n  - name: DIFFERENT\n"
	payload.BaseRevision = first.receipt.CommittedRevision

	differing := store.write(payload)
	if differing.kind != docWriteConflict {
		t.Fatalf("same token with different args must conflict, got %+v", differing)
	}
}

func TestDocWriteRejectsDisallowedPath(t *testing.T) {
	store, _ := newTestDocStore(t)

	outcome := store.write(docWritePayload{
		Path:         "docs/architecture/other.yaml",
		Content:      "a: 1\n",
		BaseRevision: docExistenceAbsent,
	})

	if outcome.kind != docWriteInvalidPath {
		t.Fatalf("expected invalid_path, got %+v", outcome)
	}
}

// unreadableFileOrSkip makes abs unreadable (mode 0000) and skips the test when
// the environment cannot actually enforce that (root, or an fs/ACL that still
// permits the read) — keeping the guard test itself free of that branching.
func unreadableFileOrSkip(t *testing.T, abs string) {
	t.Helper()

	if os.Geteuid() == 0 {
		t.Skip("root bypasses file permission bits; the unreadable-target guard cannot be exercised as root")
	}

	chmodErr := os.Chmod(abs, 0o000)
	if chmodErr != nil {
		t.Fatalf("chmod 000: %v", chmodErr)
	}

	t.Cleanup(func() { _ = os.Chmod(abs, 0o644) })

	_, probeErr := os.ReadFile(abs)
	if probeErr == nil {
		t.Skip("environment allows reading a 0000 file (fs/ACL semantics); guard not exercisable here")
	}
}

func TestDocWriteRejectsUnreadableExistingTargetInsteadOfClobbering(t *testing.T) {
	store, folder := newTestDocStore(t)

	abs := filepath.Join(folder, prdYAMLPath)

	original, origErr := os.ReadFile(abs)
	if origErr != nil {
		t.Fatalf("read original: %v", origErr)
	}

	// Make the EXISTING file unreadable (directory stays writable) so the
	// pre-commit read fails with EACCES rather than ErrNotExist.
	unreadableFileOrSkip(t, abs)

	// A client that saw the file as "absent" (an EACCES read) and sends
	// base_revision=absent must NOT be allowed to overwrite the real file.
	outcome := store.write(docWritePayload{
		Path:         prdYAMLPath,
		Content:      "title: clobbered\n",
		BaseRevision: docExistenceAbsent,
	})

	if outcome.kind != docWriteIOError {
		t.Fatalf("expected io_error for an unreadable existing target, got %+v", outcome)
	}

	// Restore read access and prove the file was NOT clobbered.
	restoreErr := os.Chmod(abs, 0o644)
	if restoreErr != nil {
		t.Fatalf("restore chmod: %v", restoreErr)
	}

	after, readErr := os.ReadFile(abs)
	if readErr != nil {
		t.Fatalf("read back: %v", readErr)
	}

	if string(after) != string(original) {
		t.Fatalf("unreadable target must be left intact, got %q", string(after))
	}
}

func TestDocWritePreservesExistingFileMode(t *testing.T) {
	store, folder := newTestDocStore(t)

	abs := filepath.Join(folder, prdYAMLPath)

	chmodErr := os.Chmod(abs, 0o640)
	if chmodErr != nil {
		t.Fatalf("chmod: %v", chmodErr)
	}

	initial, _ := store.read(prdYAMLPath)
	outcome := store.write(docWritePayload{
		Path:         prdYAMLPath,
		Content:      "title: updated\n",
		BaseRevision: initial.Revision,
	})

	if outcome.kind != docWriteOK {
		t.Fatalf("expected ok, got %+v", outcome)
	}

	info, err := os.Stat(abs)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}

	if info.Mode().Perm() != 0o640 {
		t.Fatalf("expected the update to PRESERVE the existing file's mode 0640, got %o", info.Mode().Perm())
	}
}

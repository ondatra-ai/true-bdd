package remote

// docs.go implements the workspace document persistence backend (plan Slice
// 0 — S1): the `doc_tree` / `doc_read` / `doc_write` synchronous work items,
// parallel to the `query` handler in handlers.go, off the run machinery.
//
// doc_write is the S1 persistence backend: path-safety (workspace-relative,
// canonical containment under docs/, symlink resolution of the FINAL target,
// `.yaml`-only, an exact-pattern allowlist plus an exclusive-creatable story
// glob), a YAML gate, per-document serialization with a stale-`base_revision`
// conflict, an atomic temp+fsync+rename commit, and a write receipt. No
// loaders/caching — direct file I/O (CLAUDE.md's Core Data Flow Principle).

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// docExistenceAbsent is the revision value for a path that does not exist.
const docExistenceAbsent = "absent"

// storyGlobDir is the exclusive-creatable story directory (plan Target
// state): a NEW `docs/product/stories/*.yaml` is the only allowed CREATE target.
const storyGlobDir = "docs/product/stories/"

// existingFixedDocCount bounds tree()'s slice preallocation (the 4 fixed
// documents; story files are appended on top).
const existingFixedDocCount = 4

// newFilePerm is the mode a newly-CREATED document gets (an update instead
// PRESERVES the existing file's mode — plan Slice 0 (d)).
const newFilePerm = 0o644

// The fixed (non-glob) workspace document paths (plan Target state).
// Deliberately independent of the engine config: these are the workspace
// UI-manifest contract (doc_tree order, doc_read/doc_write allowlist)
// shared with the browser, which always addresses the canonical layout.
const (
	archYAMLPath      = "docs/architecture/architecture.yaml"
	productYAMLPath   = "docs/product/product.yaml"
	featuresYAMLPath  = "docs/product/features.yaml"
	scenariosYAMLPath = "docs/scenarios.yaml"
)

// docExactAllowed is the exact-pattern allowlist (plan Target state): a
// similarly-named file in the wrong directory is rejected.
var docExactAllowed = map[string]bool{ //nolint:gochecknoglobals // fixed document allowlist
	archYAMLPath:      true,
	productYAMLPath:   true,
	featuresYAMLPath:  true,
	scenariosYAMLPath: true,
}

// docFixedPaths lists the always-present (non-glob) workspace documents, in
// the manifest order the doc_tree reply uses.
var docFixedPaths = []string{ //nolint:gochecknoglobals // fixed document manifest order
	archYAMLPath,
	productYAMLPath,
	featuresYAMLPath,
	scenariosYAMLPath,
}

var (
	errDocInvalidPath = errors.New("invalid document path")
	errDocPathEscape  = errors.New("document path escapes the project root")
)

// docNode is one doc_tree manifest entry.
type docNode struct {
	Path     string `json:"path"`
	Exists   bool   `json:"exists"`
	Revision string `json:"revision"`
}

// docTreeResult is the doc_tree work item's reply body.
type docTreeResult struct {
	Docs []docNode `json:"docs"`
}

// docReadPayload is the body of a `doc_read` work item.
type docReadPayload struct {
	Path string `json:"path"`
}

// docReadResult is the reply to a `doc_read` work item.
type docReadResult struct {
	Content     string `json:"content"`
	Exists      bool   `json:"exists"`
	ParseStatus string `json:"parse_status"` // "ok" | "invalid" | "missing"
	Revision    string `json:"revision"`
}

// Parse-status values (browser contract).
const (
	parseStatusOK      = "ok"
	parseStatusInvalid = "invalid"
	parseStatusMissing = "missing"
)

// docWritePayload is the body of a `doc_write` work item.
type docWritePayload struct {
	Path         string `json:"path"`
	Content      string `json:"content"`
	BaseRevision string `json:"base_revision"`
	ClientToken  string `json:"client_token"`
}

// docWriteReceipt is the CLI's write receipt — the S1 proof (plan Slice 0
// (e)): matched against the on-disk SHA-256 by the browser AND the relay
// receipt-audit hook.
type docWriteReceipt struct {
	Path              string `json:"path"`
	CommittedRevision string `json:"committed_revision"`
	ContentHash       string `json:"content_hash"`
	Bytes             int    `json:"bytes"`
}

// docDedupEntry remembers ONE client_token's accepted write (plan r2 #17):
// an identical replay (same request hash) returns the stored receipt; the
// SAME token with DIFFERENT args is a 409 conflict. Bounded to one agent
// lifetime (in-memory; a restart clears it).
type docDedupEntry struct {
	requestHash string
	receipt     docWriteReceipt
}

// docStore is the per-agent document persistence backend, rooted at the
// canonical host folder.
type docStore struct {
	folder string

	mu        sync.Mutex
	pathLocks map[string]*sync.Mutex
	dedup     map[string]docDedupEntry
}

func newDocStore(folder string) *docStore {
	return &docStore{
		folder:    folder,
		pathLocks: map[string]*sync.Mutex{},
		dedup:     map[string]docDedupEntry{},
	}
}

// docAllowed reports whether a CLEAN (path.Clean'd, forward-slash) relative
// path is in the exact allowlist or matches the exclusive-creatable story
// glob `docs/product/stories/*.yaml` (no subdirectories).
func docAllowed(clean string) bool {
	if docExactAllowed[clean] {
		return true
	}

	dir, file := path.Split(clean)

	return dir == storyGlobDir && strings.HasSuffix(file, ".yaml") && file != ".yaml"
}

// malformedPath reports whether relPath fails a cheap, filesystem-free
// syntactic check (traversal/absolute/NUL/wrong-extension/wrong-directory) —
// split out of validatePath to keep its cyclomatic complexity in check. It is
// a flat sequence of independent one-line rejections; splitting it further
// would scatter the path-safety policy across more functions than it clarifies.
//
//nolint:cyclop // see above
func malformedPath(relPath string) (string, bool) {
	if relPath == "" || strings.ContainsRune(relPath, 0) {
		return "", true
	}

	if path.IsAbs(relPath) || filepath.IsAbs(relPath) || strings.HasPrefix(relPath, "\\") {
		return "", true
	}

	slashPath := filepath.ToSlash(relPath)

	clean := path.Clean(slashPath)
	if clean != slashPath || clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", true
	}

	if !strings.HasSuffix(clean, ".yaml") || !docAllowed(clean) {
		return "", true
	}

	return clean, false
}

// validatePath rejects traversal/absolute/NUL/wrong-extension/wrong-directory
// paths BEFORE any filesystem access, then resolves the FINAL target through
// any existing symlinks (not just parents) and enforces canonical containment
// under the project root (plan Slice 0 / r1 #12, r2 #12).
func (d *docStore) validatePath(relPath string) (string, string, error) {
	clean, bad := malformedPath(relPath)
	if bad {
		return "", "", errDocInvalidPath
	}

	resolved, err := resolveWithinRoot(d.folder, clean)
	if err != nil {
		return "", "", err
	}

	return clean, resolved, nil
}

// resolveWithinRoot resolves every symlink along the LONGEST EXISTING
// ancestor of folder/clean (covering the final target itself when it
// exists), appends any not-yet-existing suffix (the exclusive-create case),
// and rejects the result if it escapes the canonical root (plan r1 #12).
func resolveWithinRoot(folder, clean string) (string, error) {
	canonicalRoot, err := filepath.EvalSymlinks(folder)
	if err != nil {
		canonicalRoot = folder
	}

	existing, suffix := longestExistingAncestor(filepath.Join(canonicalRoot, filepath.FromSlash(clean)))

	realExisting, err := filepath.EvalSymlinks(existing)
	if err != nil {
		return "", errDocInvalidPath
	}

	resolved := realExisting
	for _, seg := range suffix {
		resolved = filepath.Join(resolved, seg)
	}

	rel, err := filepath.Rel(canonicalRoot, resolved)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errDocPathEscape
	}

	return resolved, nil
}

// longestExistingAncestor walks UP from target until it finds a path segment
// that exists on disk, returning that ancestor plus the not-yet-existing
// suffix segments (in order) — the exclusive-create case (a brand new story
// file has no existing leaf, but its parent directory does).
func longestExistingAncestor(target string) (string, []string) {
	existing := target

	var suffix []string

	for {
		_, statErr := os.Lstat(existing)
		if statErr == nil {
			return existing, suffix
		}

		parent := filepath.Dir(existing)
		if parent == existing {
			return existing, suffix
		}

		suffix = append([]string{filepath.Base(existing)}, suffix...)
		existing = parent
	}
}

// revisionOf is the content-derived revision (plan Slice 0): SHA-256(bytes)
// when the document exists, else the fixed absent sentinel. Recomputed fresh
// on every read/write — never an in-memory counter.
func revisionOf(exists bool, data []byte) string {
	if !exists {
		return docExistenceAbsent
	}

	sum := sha256.Sum256(data)

	return hex.EncodeToString(sum[:])
}

// isValidYAML reports whether data parses as EXACTLY ONE YAML document (the
// P10a/P10b gate). A single decoder pass — not yaml.Unmarshal — because
// Unmarshal silently accepts a multi-document stream by decoding only its FIRST
// document, whereas the browser's authoritative single-document YAML.parse()
// REJECTS multiple documents. Aligning the CLI gate with the client keeps the
// committed file re-parseable by the workspace (a stray `---` separator in a
// direct doc_write must not commit content the browser then treats as invalid).
// See chat.go's stripAccidentalDocumentMarkers for the same mismatch on the
// chat path.
func isValidYAML(data []byte) bool {
	dec := yaml.NewDecoder(bytes.NewReader(data))

	firstErr := dec.Decode(new(any))
	if firstErr != nil {
		// EOF here means no document at all (empty / whitespace / comment-only) —
		// still valid, matching the browser's YAML.parse("") ⇒ null.
		return errors.Is(firstErr, io.EOF)
	}

	// A second successful/parseable document ⇒ a multi-document stream ⇒ reject;
	// only io.EOF (no further document) is a single-document file.
	return errors.Is(dec.Decode(new(any)), io.EOF)
}

// tree lists every fixed document plus every current story file (a fresh
// directory scan — the live re-fetch a new story file needs, r2 #3).
func (d *docStore) tree() docTreeResult {
	paths := make([]string, 0, len(docFixedPaths)+existingFixedDocCount)
	paths = append(paths, docFixedPaths...)
	paths = append(paths, d.storyPaths()...)

	nodes := make([]docNode, 0, len(paths))

	for _, p := range paths {
		abs := filepath.Join(d.folder, filepath.FromSlash(p))

		data, err := os.ReadFile(abs) //nolint:gosec // p is drawn from the fixed allowlist + a directory scan under docs/
		exists := err == nil
		nodes = append(nodes, docNode{Path: p, Exists: exists, Revision: revisionOf(exists, data)})
	}

	return docTreeResult{Docs: nodes}
}

// storyPaths globs docs/product/stories/*.yaml directly off disk (no cache).
func (d *docStore) storyPaths() []string {
	dir := filepath.Join(d.folder, "docs", "product", "stories")

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var out []string

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}

		out = append(out, path.Join("docs/product/stories", entry.Name()))
	}

	sort.Strings(out)

	return out
}

// read implements doc_read: current bytes + existence/parse status + revision.
func (d *docStore) read(relPath string) (docReadResult, error) {
	_, resolved, err := d.validatePath(relPath)
	if err != nil {
		return docReadResult{}, err
	}

	data, readErr := os.ReadFile(resolved) //nolint:gosec // resolved is containment-checked by validatePath
	exists := readErr == nil

	status := parseStatusMissing

	if exists {
		if isValidYAML(data) {
			status = parseStatusOK
		} else {
			status = parseStatusInvalid
		}
	}

	return docReadResult{
		Content:     string(data),
		Exists:      exists,
		ParseStatus: status,
		Revision:    revisionOf(exists, data),
	}, nil
}

// docWriteOutcome carries the write handler's result kind.
type docWriteOutcome struct {
	kind    string // "ok" | "invalid_path" | "invalid_yaml" | "conflict" | "io_error"
	receipt docWriteReceipt
}

const (
	docWriteOK          = "ok"
	docWriteInvalidPath = "invalid_path"
	docWriteInvalidYAML = "invalid_yaml"
	docWriteConflict    = "conflict"
	docWriteIOError     = "io_error"
)

// write implements doc_write end-to-end: path-safety, YAML gate, idempotency
// dedup, per-document serialization, stale-revision conflict, and the atomic
// temp+fsync+rename commit (plan Slice 0).
func (d *docStore) write(payload docWritePayload) docWriteOutcome {
	clean, resolved, err := d.validatePath(payload.Path)
	if err != nil {
		return docWriteOutcome{kind: docWriteInvalidPath}
	}

	if !isValidYAML([]byte(payload.Content)) {
		return docWriteOutcome{kind: docWriteInvalidYAML}
	}

	reqHash := docRequestHash(clean, payload.Content, payload.BaseRevision)

	if payload.ClientToken != "" {
		if outcome, done := d.dedupOutcome(payload.ClientToken, reqHash); done {
			return outcome
		}
	}

	lock := d.lockFor(clean)
	lock.Lock()
	defer lock.Unlock()

	exists, currentRevision, ioErr := d.currentState(resolved)
	if ioErr {
		return docWriteOutcome{kind: docWriteIOError}
	}

	if payload.BaseRevision != currentRevision {
		return docWriteOutcome{kind: docWriteConflict}
	}

	content := []byte(payload.Content)

	writeErr := atomicWrite(resolved, content, targetMode(resolved, exists))
	if writeErr != nil {
		return docWriteOutcome{kind: docWriteIOError}
	}

	newRevision := revisionOf(true, content)
	receipt := docWriteReceipt{
		Path:              clean,
		CommittedRevision: newRevision,
		ContentHash:       newRevision,
		Bytes:             len(content),
	}

	if payload.ClientToken != "" {
		d.dedupStore(payload.ClientToken, reqHash, receipt)
	}

	return docWriteOutcome{kind: docWriteOK, receipt: receipt}
}

// targetMode is the mode the atomic commit applies up front (plan Slice 0 (d)):
// an UPDATE preserves the existing file's mode; a CREATE uses newFilePerm. A
// stat failure on an existing file falls back to newFilePerm.
func targetMode(resolved string, exists bool) os.FileMode {
	if !exists {
		return os.FileMode(newFilePerm)
	}

	info, statErr := os.Stat(resolved)
	if statErr != nil {
		return os.FileMode(newFilePerm)
	}

	return info.Mode().Perm()
}

// currentState reads the current on-disk revision under the caller's per-document
// lock. Only a genuine ErrNotExist counts as "absent" (existence false, absent
// revision) — any OTHER read error (EACCES, EISDIR, transient I/O) returns
// ioErr=true so the write is refused rather than collapsing to the absent
// revision, which a base_revision of "absent" would otherwise use to CLOBBER an
// existing-but-currently-unreadable file (bypassing the revision/conflict guard).
func (d *docStore) currentState(resolved string) (bool, string, bool) {
	existing, readErr := os.ReadFile(resolved) //nolint:gosec // resolved is containment-checked by validatePath
	if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
		return false, "", true
	}

	exists := readErr == nil

	return exists, revisionOf(exists, existing), false
}

// dedupOutcome checks a client_token against the dedup table. The second
// return is true when the caller must return the first immediately (either
// an identical replay's original receipt, or a 409 conflict for a reused
// token with different args); false means no prior record exists, so the
// caller should proceed with the write.
func (d *docStore) dedupOutcome(token, requestHash string) (docWriteOutcome, bool) {
	entry, ok := d.dedupLookup(token)
	if !ok {
		return docWriteOutcome{}, false
	}

	if entry.requestHash == requestHash {
		return docWriteOutcome{kind: docWriteOK, receipt: entry.receipt}, true
	}

	return docWriteOutcome{kind: docWriteConflict}, true
}

func (d *docStore) lockFor(clean string) *sync.Mutex {
	d.mu.Lock()
	defer d.mu.Unlock()

	lock, ok := d.pathLocks[clean]
	if !ok {
		lock = &sync.Mutex{}
		d.pathLocks[clean] = lock
	}

	return lock
}

func (d *docStore) dedupLookup(token string) (docDedupEntry, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()

	entry, ok := d.dedup[token]

	return entry, ok
}

func (d *docStore) dedupStore(token, requestHash string, receipt docWriteReceipt) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.dedup[token] = docDedupEntry{requestHash: requestHash, receipt: receipt}
}

// docRequestHash is the idempotency fingerprint over {canonical_path,
// content_hash, base_revision} (plan r2 #17, mirroring the dispatch
// arg-fingerprint in handlers.go).
func docRequestHash(cleanPath, content, baseRevision string) string {
	contentHash := revisionOf(true, []byte(content))
	sum := sha256.Sum256([]byte(cleanPath + "\x00" + contentHash + "\x00" + baseRevision))

	return hex.EncodeToString(sum[:])
}

// atomicWrite commits bytes durably (plan Slice 0 (d)): a same-dir temp file
// with the INTENDED mode applied up front → write → file fsync → atomic
// rename over the target → parent-dir fsync → clean up any abandoned temp.
func atomicWrite(target string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(target)

	tmp, err := os.CreateTemp(dir, ".true-bdd-docwrite-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}

	tmpName := tmp.Name()
	renamed := false

	defer func() {
		if !renamed {
			_ = os.Remove(tmpName)
		}
	}()

	chmodErr := tmp.Chmod(mode)
	if chmodErr != nil {
		_ = tmp.Close()

		return fmt.Errorf("chmod temp file: %w", chmodErr)
	}

	_, writeErr := tmp.Write(data)
	if writeErr != nil {
		_ = tmp.Close()

		return fmt.Errorf("write temp file: %w", writeErr)
	}

	syncErr := tmp.Sync()
	if syncErr != nil {
		_ = tmp.Close()

		return fmt.Errorf("fsync temp file: %w", syncErr)
	}

	closeErr := tmp.Close()
	if closeErr != nil {
		return fmt.Errorf("close temp file: %w", closeErr)
	}

	renameErr := os.Rename(tmpName, target)
	if renameErr != nil {
		return fmt.Errorf("rename into place: %w", renameErr)
	}

	renamed = true

	dirHandle, err := os.Open(dir) //nolint:gosec // dir is the target's own parent, already validated
	if err != nil {
		// The rename already succeeded; the dir-fsync below is best-effort
		// durability polish, not part of the commit itself.
		return nil
	}
	defer func() { _ = dirHandle.Close() }()

	_ = dirHandle.Sync()

	return nil
}

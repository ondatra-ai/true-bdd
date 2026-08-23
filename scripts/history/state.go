package history

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/ondatra-ai/true-bdd/scripts/internal/textutil"
)

const (
	stampLayout    = "2006-01-02T15:04:05Z"
	filenameLayout = "20060102-150405"
)

// Permissions for the history tree: a directory a person browses, files only
// this hook writes.
const (
	dirMode  = 0o755
	fileMode = 0o600
)

// loadCurrent reads the task file the session is appending to, or "" when
// none is active. Any read failure reads as "no current file" (unlike the
// Python, which only caught ENOENT) — recovery is the same either way.
func (h *Hook) loadCurrent() string {
	raw, err := os.ReadFile(h.stateFile())
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(raw))
}

// saveCurrent records the task file's name, atomically. The state file is
// shared across sessions, so a half-written name would strand every one of
// them.
func (h *Hook) saveCurrent(filename string) error {
	err := os.MkdirAll(h.historyDir(), dirMode)
	if err != nil {
		return fmt.Errorf("creating %s: %w", h.historyDir(), err)
	}

	temporary := fmt.Sprintf("%s.tmp.%d", h.stateFile(), os.Getpid())

	err = os.WriteFile(temporary, []byte(filename), fileMode)
	if err != nil {
		return fmt.Errorf("writing %s: %w", temporary, err)
	}

	err = os.Rename(temporary, h.stateFile())
	if err != nil {
		return fmt.Errorf("replacing %s: %w", h.stateFile(), err)
	}

	return nil
}

// openNewFile creates docs/history/<UTC-ts>-<session8>-<slug>.md and returns
// its name.
func (h *Hook) openNewFile(sessionID, firstPrompt string) (string, error) {
	err := os.MkdirAll(h.historyDir(), dirMode)
	if err != nil {
		return "", fmt.Errorf("creating %s: %w", h.historyDir(), err)
	}

	name := fmt.Sprintf("%s-%s-%s.md",
		time.Now().UTC().Format(filenameLayout),
		textutil.Truncate(sessionID, sessionIDWidth),
		slugify(firstPrompt))

	handle, err := os.OpenFile(filepath.Join(h.historyDir(), name), os.O_CREATE|os.O_WRONLY, fileMode)
	if err != nil {
		return "", fmt.Errorf("creating %s: %w", name, err)
	}

	err = handle.Close()
	if err != nil {
		return "", fmt.Errorf("closing %s: %w", name, err)
	}

	return name, nil
}

// appendEntry writes one heading, its stamp and its body to the task file.
func (h *Hook) appendEntry(filename, heading, body string) error {
	entry := fmt.Sprintf("## %s\n\n_%s · %s_\n\n%s\n\n",
		heading, time.Now().UTC().Format(stampLayout), h.gitSHA(), body)

	handle, err := os.OpenFile(
		filepath.Join(h.historyDir(), filename), os.O_APPEND|os.O_CREATE|os.O_WRONLY, fileMode)
	if err != nil {
		return fmt.Errorf("opening %s: %w", filename, err)
	}

	defer handle.Close() //nolint:errcheck // the write below reports the failure that matters.

	_, err = handle.WriteString(entry)
	if err != nil {
		return fmt.Errorf("appending to %s: %w", filename, err)
	}

	return nil
}

const (
	sessionIDWidth = 8
	slugSourceCap  = 120
	slugCap        = 40
)

var nonSlugRE = regexp.MustCompile(`[^a-z0-9]+`)

// slugify turns the first prompt into the task file's name.
func slugify(text string) string {
	lowered := strings.ToLower(textutil.Truncate(text, slugSourceCap))

	slug := strings.Trim(nonSlugRE.ReplaceAllString(lowered, "-"), "-")
	slug = strings.TrimRight(textutil.Truncate(slug, slugCap), "-")

	if slug == "" {
		return "msg"
	}

	return slug
}

package state

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/ondatra-ai/true-bdd/scripts/internal/textutil"
)

const (
	filenameLayout = "20060102-150405"

	sessionIDWidth = 8
	slugSourceCap  = 120
	slugCap        = 40
)

// HistoryFile is the Task's transcript, DERIVED from the stem rather than
// stored: two processes that find no file compute the same name, so lazy
// creation has no "who creates it" race.
func HistoryFile(repo, task string) string {
	return filepath.Join(HistoryDir(repo), task+".md")
}

// CursorKey names one session's slot. Truncated so a key stays an id rather
// than a payload, and so the same session always folds onto itself.
func CursorKey(sessionID string) string {
	name := textutil.Truncate(sessionID, sessionIDWidth)
	if name == "" {
		name = "unknown"
	}

	return "cursor:" + name
}

// Task returns the Task's stem, opening one from this prompt when none is
// active. It creates no file: everything the stem names is opened
// O_APPEND|O_CREATE by whichever writer gets there first.
func Task(repo, sessionID, firstPrompt string) (string, error) {
	current := Get(repo, TaskKey)
	if current != "" {
		return current, nil
	}

	stem := fmt.Sprintf("%s-%s-%s",
		time.Now().UTC().Format(filenameLayout),
		textutil.Truncate(sessionID, sessionIDWidth),
		slugify(firstPrompt))

	err := Set(repo, TaskKey, stem)
	if err != nil {
		return "", err
	}

	return stem, nil
}

var nonSlugRE = regexp.MustCompile(`[^a-z0-9]+`)

// slugify turns the first prompt into the readable half of the stem.
func slugify(text string) string {
	lowered := strings.ToLower(textutil.Truncate(text, slugSourceCap))

	slug := strings.Trim(nonSlugRE.ReplaceAllString(lowered, "-"), "-")
	slug = strings.TrimRight(textutil.Truncate(slug, slugCap), "-")

	if slug == "" {
		return "msg"
	}

	return slug
}

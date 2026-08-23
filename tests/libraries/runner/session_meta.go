package runner

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// SessionMetaFile is the session-level record's name. It sits at the
// session ROOT as a file, like harness.log.json, precisely so the
// reporter's directory scan cannot mistake it for a fixture.
const SessionMetaFile = "session.json"

// sessionMetaSchema versions the on-disk shape.
const sessionMetaSchema = 1

// SessionMeta is what is true of a whole `go test` invocation: which AI
// mode it ran under, and which fixtures it set out to run. Planned is
// the target fixture list, since a report can only see directories that already exist.
type SessionMeta struct {
	Schema    int       `json:"schema"`
	Mode      string    `json:"mode"`
	StartedAt time.Time `json:"started_at"`
	Planned   []string  `json:"planned"`
}

// SessionMetaPath is the session record's location for one session.
func SessionMetaPath(sessionRoot string) string {
	return filepath.Join(sessionRoot, SessionMetaFile)
}

// WriteSessionMeta records a session's mode and planned fixture list.
// Write-then-rename, like the harness record: the report server polls
// this tree periodically and must never read a half-written file.
func WriteSessionMeta(sessionRoot, mode string, planned []string) error {
	meta := SessionMeta{
		Schema:    sessionMetaSchema,
		Mode:      mode,
		StartedAt: time.Now(),
		Planned:   planned,
	}

	blob, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("encode session meta: %w", err)
	}

	final := SessionMetaPath(sessionRoot)
	tmp := final + ".tmp"

	err = os.WriteFile(tmp, append(blob, '\n'), filePerm)
	if err != nil {
		return fmt.Errorf("write session meta: %w", err)
	}

	err = os.Rename(tmp, final)
	if err != nil {
		return fmt.Errorf("publish session meta: %w", err)
	}

	return nil
}

// ReadSessionMeta loads one session's record. A session recorded before
// this file existed reports (nil, nil) — absent is not an error, it is
// an older run.
func ReadSessionMeta(sessionRoot string) (*SessionMeta, error) {
	blob, err := os.ReadFile(SessionMetaPath(sessionRoot))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil //nolint:nilnil // absent is a valid, expected state
		}

		return nil, fmt.Errorf("read session meta: %w", err)
	}

	var meta SessionMeta

	err = json.Unmarshal(blob, &meta)
	if err != nil {
		return nil, fmt.Errorf("decode session meta: %w", err)
	}

	return &meta, nil
}

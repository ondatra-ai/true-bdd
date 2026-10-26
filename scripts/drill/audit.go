package drill

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"github.com/ondatra-ai/true-bdd/pkg/enginelog"
)

const unknownValue = "unknown"

// Audit records references, their location, and gaps in the available evidence.
type Audit struct {
	At      time.Time `json:"at"`
	Entries []Access  `json:"entries"`
	Files   []string  `json:"files"`
	Unknown []string  `json:"unknown"`
}

// Access describes a reference; it never infers that a read succeeded.
type Access struct {
	Evidence string `json:"evidence"`
	Tool     string `json:"tool"`
	Target   string `json:"target"`
	Group    string `json:"group"`
	Decision string `json:"decision"`
	Raw      string `json:"raw"`
}

// Audit reads the engine JSON log and Codex event streams under the cut's tmp tree.
func (r Runner) Audit(_ context.Context, directory, name string) (Audit, error) {
	record, err := Read(directory)
	if err != nil {
		return Audit{}, err
	}

	cut, err := record.cut(name)
	if err != nil {
		return Audit{}, err
	}

	audit := Audit{At: time.Now().UTC(),
		Entries: []Access{},
		Files:   []string{},
		Unknown: []string{"References do not establish successful reads. Missing or unlogged calls cannot be recovered. " +
			"Network access remains open; this report is not a contamination verdict.",
			"Unstructured strings and shell commands are retained verbatim; " +
				"computed, encoded and ambiguous relative paths cannot always be identified.",
			"Engine denial summaries name tools, not individual calls; " +
				"they cannot be assigned to a target without an explicit decision."}}
	audit.engineLog(filepath.Join(cut.Directory, "tmp/true-bdd.log.json"), cut.Directory, record.AllowedRoots)
	audit.codexEvents(cut.Directory, record.AllowedRoots)
	audit.oracleLimitations(record.Runs, name)
	err = change(directory, func(record *Record) error {
		record.Audits[name] = append(record.Audits[name], audit)

		return nil
	})

	for _, path := range audit.Files {
		slog.Info("Drill audit evidence", "path", path)
	}

	for _, entry := range audit.Entries {
		slog.Info("Drill access reference",
			"evidence",
			entry.Evidence,
			"tool_name",
			entry.Tool,
			"target",
			entry.Target,
			"group",
			entry.Group,
			"decision",
			entry.Decision)
	}

	for _, unknown := range audit.Unknown {
		slog.Info("Drill audit limitation", "detail", unknown)
	}

	return audit, err
}

func (a *Audit) oracleLimitations(runs map[string]Attempt, cut string) {
	tags := make([]string, 0, len(runs))
	for tag, run := range runs {
		if run.Cut == cut && run.Oracle != nil {
			tags = append(tags, tag)
		}
	}

	slices.Sort(tags)

	for _, tag := range tags {
		directory := runs[tag].Oracle.Directory
		if directory == "" {
			directory = "not created"
		}

		a.Unknown = append(a.Unknown, fmt.Sprintf(
			"Oracle %s (directory: %s): not audited; -mode=services:replay,tests:replay runs no live AI turn, "+
				"so there is no AI access to audit. Replayed logs contain recording-time paths, not access evidence for this run.",
			tag, directory))
	}
}

func (a *Audit) engineLog(path, tree string, roots []string) {
	data, err := disk.Read(path)
	if err != nil {
		a.Unknown = append(a.Unknown, err.Error())

		return
	}

	a.Files = append(a.Files, path)
	decoder := json.NewDecoder(strings.NewReader(string(data)))

	for {
		var record map[string]any

		err := decoder.Decode(&record)
		if errors.Is(err, io.EOF) {
			return
		}

		if err != nil {
			a.Unknown = append(a.Unknown, fmt.Sprintf("%s: invalid log record: %s", path, err))

			return
		}

		message, _ := record["msg"].(string)
		if strings.Contains(message, "denied") {
			raw, err := json.Marshal(record)
			if err != nil {
				a.Unknown = append(a.Unknown, err.Error())

				continue
			}

			a.Unknown = append(a.Unknown, string(raw))
		}

		if message != enginelog.MsgToolUse {
			continue
		}

		a.engineCall(record, path, tree, roots)
	}
}

func decision(record map[string]any) string {
	if allowed, ok := record["allowed"].(bool); ok {
		if allowed {
			return "allowed"
		}

		return "denied"
	}

	if denied, ok := record["denied"].(bool); ok && denied {
		return "denied"
	}

	return unknownValue
}

func (a *Audit) engineCall(record map[string]any, path, tree string, roots []string) {
	content := record[enginelog.KeyContent]
	if value, ok := content.(string); ok {
		var nested any
		if json.Unmarshal([]byte(value), &nested) == nil {
			content = nested
		} else {
			a.addAccess(value, "", value, path, tree, roots, decision(record))

			return
		}
	}

	a.references(content, path, tree, roots, decision(record))
}

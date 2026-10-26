package codexrun

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

// Record retains both attempts and the original writer commit baseline.
type Record struct {
	Options

	Agent       string       `json:"agent"`
	CodexStatus string       `json:"codex_status,omitempty"`
	Status      string       `json:"status"`
	Reason      string       `json:"reason,omitempty"`
	Attempt     int          `json:"attempt"`
	PID         int          `json:"pid"`
	Worktree    string       `json:"worktree,omitempty"`
	Workdir     string       `json:"workdir"`
	Since       string       `json:"since,omitempty"`
	SessionID   string       `json:"session_id,omitempty"`
	Log         string       `json:"log"`
	Final       string       `json:"final"`
	ExitCode    int          `json:"exit_code"`
	StartedAt   time.Time    `json:"started_at"`
	EndedAt     time.Time    `json:"ended_at"`
	WeeklyLimit *WeeklyLimit `json:"weekly_limit"`
}

func load(path string) (Record, error) {
	data, err := disk.Read(path)
	if err != nil {
		return Record{}, err
	}

	var record Record

	err = json.Unmarshal(data, &record)
	if err != nil {
		return record, fmt.Errorf("reading run record: %w", err)
	}

	return record, nil
}

func (r Runner) save(record Record) error {
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding run record: %w", err)
	}

	return disk.Write(r.file(record.Name, "run.json"), append(data, '\n'), disk.Private)
}

func (r Runner) file(name, base string) string {
	return filepath.Join(r.Root, "tmp", agentCodex, name, base)
}

func (r Runner) finish(record Record, status, reason string, code int) (Record, error) {
	record.Status = status
	record.Reason = reason
	record.ExitCode = code

	err := r.stopMarked(record)
	if err != nil {
		record.Status, record.Reason = "failed", err.Error()
	} else if status == statusDone && record.writer() {
		problem := r.repository(record.Worktree).CompletionProblem(context.Background(), record.Branch, record.Since)
		if problem != "" {
			record.Status, record.Reason = "incomplete", problem
		}
	}

	record.EndedAt = time.Now().UTC()
	if record.Agent == agentCodex {
		data, _ := disk.Read(record.Log)

		match := regexp.MustCompile(`(?m)^session id:\s*(\S+)`).FindStringSubmatch(string(data))
		if len(match) > 1 {
			record.SessionID = match[1]
		}
	}

	return record, errors.Join(err, r.save(record))
}

// Summary is the single closing report Claude reads.
func (r Record) Summary() string {
	final, _ := disk.Read(r.Final)

	weekly := "unknown"
	if r.WeeklyLimit != nil {
		weekly = r.WeeklyLimit.String()
	}

	text := fmt.Sprintf("status: %s\nreason: %s\nagent: %s\nweekly limit: %s\n", r.Status, r.Reason, r.Agent, weekly)

	model := r.Model
	if model == "" {
		model = "configured default"
	}

	text += fmt.Sprintf("model: %s\neffort: %s\n", model, r.reasoningEffort())
	if r.CodexStatus != "" {
		text += "codex status: " + r.CodexStatus + "\n"
	}

	text += fmt.Sprintf("attempt: %d\nexit code: %d\nsession id: %s\n", r.Attempt, r.ExitCode, r.SessionID)
	if r.writer() {
		text += fmt.Sprintf("branch: %s\nworktree: %s\n", r.Branch, r.Worktree)
	}

	text += "log: " + r.Log + "\n"
	if r.Attempt == 2 && r.Status != "done" {
		text += "hand-back: Claude takes the work; read both attempt logs\n"
	}

	if len(strings.TrimSpace(string(final))) == 0 {
		final = []byte("(none)")
	}

	return text + "final message:\n" + string(final)
}

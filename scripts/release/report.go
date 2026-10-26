package release

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

// Step records a named operation, including the first failing installation step.
type Step struct {
	Host   string `json:"host,omitempty"`
	Name   string `json:"name"`
	Result string `json:"result"`
	Detail string `json:"detail,omitempty"`
}

// Turn preserves denial records verbatim, including any engine-reported targets.
type Turn struct {
	Dispatch json.RawMessage   `json:"dispatch"`
	Denials  []json.RawMessage `json:"denials"`
}

// HostResult is a host's verdict and evidence from its engine log.
type HostResult struct {
	Name          string `json:"name"`
	Stage         string `json:"stage"`
	Passed        bool   `json:"passed"`
	Skipped       string `json:"skipped,omitempty"`
	ExitCode      int    `json:"exit_code"`
	WallTime      string `json:"wall_time"`
	ClaudeSpend   string `json:"claude_spend"`
	CodexSpend    string `json:"codex_spend"`
	Turns         []Turn `json:"turns"`
	DenialTargets string `json:"denial_targets"`
	Log           string `json:"log,omitempty"`
	Transcript    string `json:"transcript,omitempty"`
}

// Report describes the install and every sample host's result.
type Report struct {
	Root    string       `json:"root"`
	Commit  string       `json:"commit"`
	Harness string       `json:"harness"`
	Live    bool         `json:"live"`
	Passed  bool         `json:"passed"`
	Steps   []Step       `json:"steps"`
	Hosts   []HostResult `json:"hosts"`
}

func newHostResult(name string) HostResult {
	return HostResult{Name: name, ExitCode: -1, WallTime: unavailableValue,
		ClaudeSpend: unavailableValue, CodexSpend: unavailableValue, DenialTargets: unavailableValue}
}

func (r *Report) step(host, name string, err error) error {
	result := Step{Host: host, Name: name, Result: "passed"}
	if err != nil {
		result.Result = "failed"
		result.Detail = err.Error()
	}

	r.Steps = append(r.Steps, result)
	slog.Info("Release rehearsal step", "host", host, "step", name, "result", result.Result, "detail", result.Detail)

	if err != nil {
		return fmt.Errorf("%s %s: %w", host, name, err)
	}

	return nil
}

func saveReport(artifacts string, report Report) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding rehearsal report: %w", err)
	}

	path := filepath.Join(artifacts, "rehearsal.json")

	err = disk.Write(path, append(data, '\n'), disk.Private)
	if err != nil {
		return fmt.Errorf("writing rehearsal report: %w", err)
	}

	slog.Info("Release rehearsal report", "path", path,
		"folder", report.Root, "passed", report.Passed, "harness", report.Harness)

	return nil
}

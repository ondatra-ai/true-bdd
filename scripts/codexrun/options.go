// Package codexrun drives persisted, supervised Codex delegation attempts.
package codexrun

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

const (
	kindCode      = "code"
	kindFix       = "fix"
	agentCodex    = "codex"
	statusRunning = "running"
	statusDone    = "done"
	maxAttempts   = 2
)

// Options is a new run's immutable brief and execution policy.
type Options struct {
	Name   string   `json:"name"`
	Kind   string   `json:"kind"`
	Brief  string   `json:"brief"`
	Branch string   `json:"branch,omitempty"`
	Base   string   `json:"base,omitempty"`
	Search bool     `json:"search"`
	Issues []string `json:"issues,omitempty"`
}

// ErrRefused means no attempt was consumed.
var ErrRefused = errors.New("run refused")

func refuse(reason string) error { return fmt.Errorf("%w: %s", ErrRefused, reason) }

func (o Options) writer() bool { return o.Kind == kindCode || o.Kind == kindFix }

func (o Options) validate() error {
	if !regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`).MatchString(o.Name) {
		return refuse("--name needs 1–64 lowercase letters, digits, dots, dashes or underscores")
	}

	switch o.Kind {
	case "plan-read", "review", "research", kindCode, kindFix:
	default:
		return refuse("--kind must be plan-read, review, research, code or fix")
	}

	for _, issue := range o.Issues {
		if !regexp.MustCompile(`^[1-9][0-9]*$`).MatchString(issue) {
			return refuse("--issue needs a positive issue number")
		}
	}

	return o.validateWorkspace()
}

func (o Options) validateWorkspace() error {
	if !o.writer() && (o.Branch != "" || o.Base != "") {
		return refuse("only writers take --branch and --base")
	}

	if o.writer() && invalidBranch(o.Branch) {
		return refuse("a writer needs --branch")
	}

	if o.Kind == kindCode && o.Base == "" {
		return refuse("a code run needs --base")
	}

	if o.Kind == kindFix && o.Base != "" {
		return refuse("a fix run takes no --base")
	}

	return nil
}

func invalidBranch(branch string) bool {
	return strings.TrimSpace(branch) == "" || strings.HasPrefix(branch, "-")
}

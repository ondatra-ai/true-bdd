// Package provider holds the engine's model-routing vocabulary: the
// named tiers a checklist selects (xhigh / high / coder) and the
// (cli, model) pair each tier resolves to.
package provider

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// CLI names the external agent binary that runs a prompt.
type CLI string

const (
	CLIClaude CLI = "claude"
	CLICrush  CLI = "crush"
	CLICodex  CLI = "codex"
)

// Model-reference parse failures. Every one is a startup error: a bad
// reference must kill the command, never silently downgrade a turn.
var (
	ErrModelRefNoSeparator = errors.New(`model reference must be "<cli>:<model>"`)
	ErrModelRefEmptyCLI    = errors.New("model reference has an empty cli")
	ErrModelRefEmptyModel  = errors.New("model reference has an empty model")
	ErrModelRefUnknownCLI  = errors.New("model reference names an unknown cli")
)

// SupportedCLIs lists every CLI the router can dispatch to.
func SupportedCLIs() []CLI {
	return []CLI{CLIClaude, CLICrush, CLICodex}
}

// IsSupported reports whether a provider exists for this CLI.
func (c CLI) IsSupported() bool {
	return slices.Contains(SupportedCLIs(), c)
}

// ModelRef is a resolved (cli, model) pair — the two things any
// provider needs to spawn one turn.
type ModelRef struct {
	CLI   CLI
	Model string
}

// ParseModelRef parses the `"<cli>:<model>"` form used by `engine.models`
// in true-bdd.yaml. Split on the FIRST colon only, so a model id containing
// a colon survives intact.
func ParseModelRef(raw string) (ModelRef, error) {
	cliPart, modelPart, found := strings.Cut(strings.TrimSpace(raw), ":")
	if !found {
		return ModelRef{}, fmt.Errorf("%w: got %q", ErrModelRefNoSeparator, raw)
	}

	cliPart = strings.TrimSpace(cliPart)
	modelPart = strings.TrimSpace(modelPart)

	if cliPart == "" {
		return ModelRef{}, fmt.Errorf("%w: got %q", ErrModelRefEmptyCLI, raw)
	}

	if modelPart == "" {
		return ModelRef{}, fmt.Errorf("%w: got %q", ErrModelRefEmptyModel, raw)
	}

	cli := CLI(cliPart)
	if !cli.IsSupported() {
		return ModelRef{}, fmt.Errorf("%w: %q (supported: %s)",
			ErrModelRefUnknownCLI, cliPart, joinCLIs(SupportedCLIs()))
	}

	return ModelRef{CLI: cli, Model: modelPart}, nil
}

// String renders the ref back in its `"<cli>:<model>"` config form.
func (r ModelRef) String() string {
	return string(r.CLI) + ":" + r.Model
}

// IsZero reports whether the ref was never resolved.
func (r ModelRef) IsZero() bool {
	return r.CLI == "" && r.Model == ""
}

// joinCLIs renders a CLI list for error messages.
func joinCLIs(clis []CLI) string {
	names := make([]string, 0, len(clis))
	for _, cli := range clis {
		names = append(names, string(cli))
	}

	return strings.Join(names, ", ")
}

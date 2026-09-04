package runner

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// Two processes spawn an AI CLI during a run, and each takes its own
// mode. One mode for both is what let `replay` mean "hermetic" while
// the judge still called out for real.
const (
	// CallerTarget is the service under test — the subprocess whose
	// third_party CLI dependencies get shimmed.
	CallerTarget = "target"
	// CallerTests is the test process itself, which reaches a model to
	// grade what the target did.
	CallerTests = "tests"
)

// ErrModeMalformed marks a `-mode` value that cannot be read. It is a
// refusal, never a fallback: a mode nobody chose is how a run bills real
// money while its operator believes it is replaying.
var ErrModeMalformed = errors.New("malformed -mode")

// Modes is one run's mode per caller. The zero value is the absent
// flag, which stays legal: the coverage guards run with no mode and
// bring no harness up, so the boot refuses an unset mode, not the parse.
type Modes struct {
	Target string
	Tests  string
}

// Set reports whether a mode was supplied.
func (m Modes) Set() bool { return m.Target != "" && m.Tests != "" }

// String renders the spec back, so a refusal or a record can quote what
// it was given rather than describing it.
func (m Modes) String() string {
	if !m.Set() {
		return ""
	}

	return CallerTarget + ":" + m.Target + "," + CallerTests + ":" + m.Tests
}

// callers is the closed set of processes that may be named.
func callers() []string { return []string{CallerTarget, CallerTests} }

// modes is the closed set of values a caller may take.
func modes() []string { return []string{ProxyModeLive, ProxyModeRecord, ProxyModeReplay} }

// ParseModes reads `target:<mode>,tests:<mode>`. No shorthand and no
// default: both make a run mean something its operator did not type.
// An empty spec is the absent flag and parses to the zero value.
func ParseModes(spec string) (Modes, error) {
	trimmed := strings.TrimSpace(spec)
	if trimmed == "" {
		return Modes{}, nil
	}

	seen := map[string]string{}

	for _, part := range strings.Split(trimmed, ",") {
		caller, mode, err := parseModePart(part)
		if err != nil {
			return Modes{}, err
		}

		if _, duplicate := seen[caller]; duplicate {
			return Modes{}, fmt.Errorf("%w: %s is named twice", ErrModeMalformed, caller)
		}

		seen[caller] = mode
	}

	return assembled(seen)
}

// parseModePart reads one `caller:mode` pair.
func parseModePart(part string) (string, string, error) {
	caller, mode, found := strings.Cut(part, ":")
	if !found {
		return "", "", fmt.Errorf("%w: %q is not caller:mode; want %s",
			ErrModeMalformed, strings.TrimSpace(part), ModeSpecForm())
	}

	caller, mode = strings.TrimSpace(caller), strings.TrimSpace(mode)

	if !slices.Contains(callers(), caller) {
		return "", "", fmt.Errorf("%w: no caller named %q; the callers are %s",
			ErrModeMalformed, caller, strings.Join(callers(), " and "))
	}

	if !slices.Contains(modes(), mode) {
		return "", "", fmt.Errorf("%w: no mode named %q for %s; the modes are %s",
			ErrModeMalformed, mode, caller, strings.Join(modes(), ", "))
	}

	return caller, mode, nil
}

// assembled refuses a spec that named some callers but not all of them.
func assembled(seen map[string]string) (Modes, error) {
	var unnamed []string

	for _, caller := range callers() {
		if seen[caller] == "" {
			unnamed = append(unnamed, caller)
		}
	}

	if len(unnamed) > 0 {
		return Modes{}, fmt.Errorf("%w: %s left unnamed — every caller states its own mode; want %s",
			ErrModeMalformed, strings.Join(unnamed, " and "), ModeSpecForm())
	}

	return Modes{Target: seen[CallerTarget], Tests: seen[CallerTests]}, nil
}

// ModeSpecForm is the accepted spelling, quoted by every refusal so the
// message carries its own fix.
func ModeSpecForm() string {
	return CallerTarget + ":<mode>," + CallerTests + ":<mode> with mode one of " +
		strings.Join(modes(), "|")
}

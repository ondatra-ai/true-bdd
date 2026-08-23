package testrunner

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
)

// ErrEmptyCommand signals a test-suite command that split to no tokens
// at all — an empty or whitespace-only string reaching a runner.
var ErrEmptyCommand = errors.New("test suite command is empty")

// ErrUnterminatedQuote signals a command whose quoting never closed.
// Refused rather than guessed: a half-quoted `-run '^Test` would
// otherwise silently become two arguments and quietly change what runs.
var ErrUnterminatedQuote = errors.New("unterminated quote in test suite command")

// ErrRerunSelectedNoTests signals a per-test rerun whose filter matched
// nothing. Reported rather than swallowed: a framework that ran no test
// reports no failure, so the walk would misread that as a fix that worked.
var ErrRerunSelectedNoTests = errors.New("test rerun selected no tests")

// ErrCommandNotMachineReadable signals a command missing the flag its
// framework needs for machine-readable output. Without it a runner parses
// a human report, finds no failures, and the walk converges on a false green.
var ErrCommandNotMachineReadable = errors.New(
	"test suite command must emit machine-readable output")

// The flag each framework needs so its runner parses results rather
// than prose.
const (
	goTestJSONFlag     = "-json"
	jestJSONFlag       = "--json"
	playwrightJSONFlag = "--reporter=json"
)

// jsonToken is the literal "json" wearing three hats: the flag name `go
// test`/jest use, the reporter Playwright needs, and captured-stdout's
// extension — one constant because the linter sees one string, not three meanings.
const jsonToken = "json"

// outputFlag describes how one framework asks for machine-readable output:
// the flag's name without dashes (`-json`/`--json` are the same flag), and
// the value that must be present when the flag takes one.
type outputFlag struct {
	name string
	// value is the item the flag's value list must contain, or "" when
	// the flag is a boolean and its presence is the whole signal.
	value string
	// spelling is the canonical form named in an error, so a refusal
	// tells the host what to write rather than what it wrote.
	spelling string
}

// machineReadableFlag describes the output flag a framework's runner
// depends on. ok is false for a framework this package does not route —
// the dispatcher reports that mistake, not a second error here.
func machineReadableFlag(framework string) (outputFlag, bool) {
	switch framework {
	case FrameworkGoTest:
		return outputFlag{name: jsonToken, spelling: goTestJSONFlag}, true
	case FrameworkJest:
		return outputFlag{name: jsonToken, spelling: jestJSONFlag}, true
	case FrameworkPlaywright:
		return outputFlag{
			name:     "reporter",
			value:    jsonToken,
			spelling: playwrightJSONFlag,
		}, true
	}

	return outputFlag{}, false
}

// SplitCommand splits a command line into argv, honouring single and double
// quotes: strings.Fields would shred `-run '^TestGreen$'` into two arguments,
// silently changing which tests run. Quoting is the only shell feature supported.
func SplitCommand(command string) ([]string, error) {
	var splitter commandSplitter

	for _, char := range command {
		splitter.consume(char)
	}

	return splitter.finish(command)
}

// commandSplitter is SplitCommand's state: the arguments closed so far,
// the one being accumulated, and the quote currently held open.
type commandSplitter struct {
	args    []string
	current strings.Builder
	quote   rune
	// started distinguishes "mid-argument" from "between arguments",
	// which is what lets `--grep ''` survive as an empty argument while
	// a run of spaces produces none.
	started bool
}

func (s *commandSplitter) consume(char rune) {
	switch {
	case s.quote != 0:
		s.consumeQuoted(char)
	case char == '\'' || char == '"':
		s.quote = char
		s.started = true
	case char == ' ' || char == '\t' || char == '\n':
		s.closeArg()
	default:
		s.current.WriteRune(char)

		s.started = true
	}
}

// consumeQuoted takes a character while a quote is open: only the
// matching quote closes it, so whitespace and the other quote character
// are ordinary content.
func (s *commandSplitter) consumeQuoted(char rune) {
	if char == s.quote {
		s.quote = 0

		return
	}

	s.current.WriteRune(char)
}

// closeArg ends the argument in progress, if any.
func (s *commandSplitter) closeArg() {
	if !s.started {
		return
	}

	s.args = append(s.args, s.current.String())
	s.current.Reset()

	s.started = false
}

// finish closes the trailing argument and rejects the two commands no
// runner can act on: one still inside a quote, and one with no tokens.
func (s *commandSplitter) finish(command string) ([]string, error) {
	if s.quote != 0 {
		return nil, fmt.Errorf("%w: %q", ErrUnterminatedQuote, command)
	}

	s.closeArg()

	// Both shapes are the same mistake: `''` closes an argument whose content
	// is empty, so token count alone would call it a command and hand exec an
	// empty binary name — a spawn failure where a startup refusal belongs.
	if len(s.args) == 0 || s.args[0] == "" {
		return nil, fmt.Errorf("%w: %q", ErrEmptyCommand, command)
	}

	return s.args, nil
}

// ValidateCommand checks one suite's command before anything is
// spawned: it must split, and it must carry its framework's
// machine-readable output flag.
func ValidateCommand(framework, command string) error {
	args, err := SplitCommand(command)
	if err != nil {
		return err
	}

	flag, routed := machineReadableFlag(framework)
	if !routed {
		return nil
	}

	if carriesOutputFlag(args, flag) {
		return nil
	}

	return fmt.Errorf("%w: %s command %q is missing %s",
		ErrCommandNotMachineReadable, framework, command, flag.spelling)
}

// carriesOutputFlag reports whether argv asks for machine-readable output.
// Deliberately generous about spelling — this check refuses a run outright,
// so `-json`, `--json`, `--reporter=json`, and `--reporter line,json` (space or `=`) all pass.
func carriesOutputFlag(args []string, flag outputFlag) bool {
	for idx, arg := range args {
		if !strings.HasPrefix(arg, "-") {
			continue
		}

		name, value, hasValue := strings.Cut(strings.TrimLeft(arg, "-"), "=")
		if name != flag.name {
			continue
		}

		if flag.value == "" {
			return true
		}

		// A value-taking flag may be spelled `--flag=v` or `--flag v`.
		if !hasValue && idx+1 < len(args) {
			value = args[idx+1]
		}

		if slices.Contains(strings.Split(value, ","), flag.value) {
			return true
		}
	}

	return false
}

// CommandDir reports the directory a suite's command runs in: `config:`'s
// directory if declared, else "" (inherit cwd). This matters because npx
// resolves the local install walking UP from cwd, so the wrong dir fetches a different registry version.
func CommandDir(cfg Config) string {
	if cfg.ConfigFile == "" {
		return ""
	}

	return filepath.Dir(cfg.ConfigFile)
}

// commandArgv splits a FailingTest's recorded command for a rerun,
// naming the test in the error so a malformed command found on the
// rerun path is still attributable to the suite that declared it.
func commandArgv(ft *FailingTest) ([]string, error) {
	argv, err := SplitCommand(ft.RunnerConfig.Command)
	if err != nil {
		return nil, fmt.Errorf("rerun %s: %w", ft.TestName, err)
	}

	return argv, nil
}

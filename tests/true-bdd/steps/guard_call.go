package steps

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

// guardDenyExitCode is claude's hook contract: 0 lets the tool run, 2
// stops it with stderr carrying the reason.
const guardDenyExitCode = 2

// unreadablePolicy is a policy written in a shape the guard cannot decode.
const unreadablePolicy = "write_roots: tests"

// ErrLinkEscapesTree is returned when a scenario plants a symbolic link
// whose own path lies outside the project tree.
var ErrLinkEscapesTree = errors.New("the link must be a path inside the project tree")

func registerGuardSteps(suite *bddgo.Suite[State]) {
	suite.Step(`^the project tree's "([^"]+)" is a symbolic link to "([^"]+)"$`, plantLink)
	suite.Step(`^the guard allows the tool call$`, assertGuardAllows)
	suite.Step(`^the guard denies the tool call$`, assertGuardDenies)
	suite.Step(`^the guard's reason matches (.+)$`, assertGuardReason)
}

// guardCall is the tool call a scenario has claude ask the guard about.
// Paths stay relative to the project tree until the When step renders them,
// since only then is the run directory they resolve against in use.
type guardCall struct {
	background   bool
	testCommand  string
	testSelector string
	testDir      string
	command      string
	workingDir   string
	roots        []string
	withheld     bool
	unreadable   bool
	noReadRoot   bool

	tool      string
	target    string
	inTree    bool
	piped     bool
	pipedTool string
	pipedFile string
	pipedPath string
}

// guardPolicy is the JSON shape the engine exports in guardPolicyVar.
type guardPolicy struct {
	ReadRoot     string   `json:"read_root"`
	WriteRoots   []string `json:"write_roots"`
	TestCommand  string   `json:"test_command,omitempty"`
	TestSelector string   `json:"test_selector,omitempty"`
	TestDir      string   `json:"test_dir,omitempty"`
}

// guard is this scenario's tool call, started empty: a policy no step
// shaped grants nothing, and a tool call no step named names nothing.
func (s *State) guard() *guardCall {
	if s.guardCall == nil {
		s.guardCall = &guardCall{}
	}

	return s.guardCall
}

func grantWriteRoot(state *State, args []string) error {
	call := state.guard()
	call.roots = append(call.roots, args[0])

	return nil
}

func withholdPolicy(state *State, _ []string) error {
	state.guard().withheld = true

	return nil
}

func garblePolicy(state *State, _ []string) error {
	state.guard().unreadable = true

	return nil
}

func omitReadRoot(state *State, _ []string) error {
	state.guard().noReadRoot = true

	return nil
}

func pipePathPayload(state *State, args []string) error {
	call := state.guard()
	call.piped, call.pipedTool, call.pipedPath = true, args[0], args[1]

	return nil
}

func nameTool(state *State, args []string) error {
	state.guard().tool = args[0]

	return nil
}

func nameTreeFile(state *State, args []string) error {
	call := state.guard()
	call.target, call.inTree = args[0], true

	return nil
}

func nameLiteralPath(state *State, args []string) error {
	call := state.guard()
	call.target, call.inTree = args[0], false

	return nil
}

func pipePayload(state *State, args []string) error {
	call := state.guard()
	call.piped, call.pipedTool, call.pipedFile = true, args[0], args[1]

	return nil
}

// plantLink makes the prep phase create the link before the pre-run
// snapshot. The snapshot refuses a link it walks into, so the link must
// sit in a directory it leaves out, such as node_modules.
func plantLink(state *State, args []string) error {
	if state.Fixture == nil {
		return state.fail("%w", ErrNoTreePrepared)
	}

	link, target := args[0], args[1]
	if filepath.IsAbs(link) || climbsOut(link) {
		return state.fail("%w: %q", ErrLinkEscapesTree, link)
	}

	state.Fixture.PrepCmds = append(state.Fixture.PrepCmds, fmt.Sprintf("mkdir -p %s && ln -s %s %s",
		shellQuote(filepath.Dir(link)), shellQuote(target), shellQuote(link)))

	return nil
}

// policy renders the write policy claude would export, its roots made
// absolute under the run directory as the engine does with write globs.
func (c *guardCall) policy(runDir string) (string, error) {
	switch {
	case c.withheld:
		return "", nil
	case c.unreadable:
		return unreadablePolicy, nil
	}

	roots := make([]string, 0, len(c.roots))
	for _, root := range c.roots {
		roots = append(roots, filepath.Join(runDir, root))
	}

	readRoot := runDir
	if c.noReadRoot {
		readRoot = ""
	}

	policy := guardPolicy{ReadRoot: readRoot, WriteRoots: roots}
	if c.testCommand != "" {
		policy.TestCommand, policy.TestSelector = c.testCommand, c.testSelector
		policy.TestDir = filepath.Join(runDir, c.testDir)
	}

	encoded, err := json.Marshal(policy)
	if err != nil {
		return "", fmt.Errorf("encode policy: %w", err)
	}

	return string(encoded), nil
}

// targetPath is the file the tool call names: absolute for a file of the
// project tree, verbatim for a path as written, blank when unnamed.
func (c *guardCall) targetPath(runDir string) string {
	if c.inTree {
		return filepath.Join(runDir, c.target)
	}

	return c.target
}

func assertGuardAllows(state *State, _ []string) error {
	return state.guardAnswered(0, "allow")
}

func assertGuardDenies(state *State, _ []string) error {
	return state.guardAnswered(guardDenyExitCode, "deny")
}

// guardAnswered pins the guard's verdict by its exit code, the only part
// of the answer claude acts on.
func (s *State) guardAnswered(want int, verb string) error {
	if s.Result == nil {
		return s.fail("%w", ErrNoRun)
	}

	if s.Fixture != nil {
		s.Fixture.ExpectedExitCode = want
	}

	if s.Result.ExitCode != want {
		s.dumpRun()

		return s.fail("expected the guard to %s the tool call (exit code %d), but it exited %d: %q",
			verb, want, s.Result.ExitCode, strings.TrimSpace(s.Result.Stderr))
	}

	return nil
}

// assertGuardReason matches the reason the guard wrote on stderr, the
// line claude shows for a denied tool: `stderr matches` in the guard's words.
func assertGuardReason(state *State, args []string) error {
	return assertStderr(state, args)
}

// shellQuote wraps a word in single quotes for the prep phase's bash.
func shellQuote(word string) string {
	return "'" + strings.ReplaceAll(word, "'", `'\''`) + "'"
}

func grantTestCommand(state *State, args []string) error {
	call := state.guard()
	call.testCommand, call.testSelector, call.testDir = args[0], args[1], args[2]

	return nil
}

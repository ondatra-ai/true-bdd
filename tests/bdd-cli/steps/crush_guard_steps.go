package steps

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
	"github.com/ondatra-ai/true-bdd/pkg/testkit/runner"
)

// guardCommand is the hidden engine subcommand crush runs as its
// PreToolUse hook before every tool call of a turn.
const guardCommand = "crush-guard"

// guardDenyExitCode is crush's hook contract: 0 lets the tool run, 2
// stops it with stderr carrying the reason.
const guardDenyExitCode = 2

// The environment crush hands its hook: the policy the engine exported for
// the turn, then the tool call itself.
const (
	guardPolicyVar   = "TRUE_BDD_CRUSH_POLICY"
	guardToolVar     = "CRUSH_TOOL_NAME"
	guardFilePathVar = "CRUSH_TOOL_INPUT_FILE_PATH"
)

// unreadablePolicy is a policy written in a shape the guard cannot decode.
const unreadablePolicy = "write_roots: tests"

// ErrLinkEscapesTree is returned when a scenario plants a symbolic link
// whose own path lies outside the project tree.
var ErrLinkEscapesTree = errors.New("the link must be a path inside the project tree")

// registerCrushGuardSteps binds the steps that drive `true-bdd crush-guard`
// the way crush does: environment variables and a hook payload in, an exit
// code and a reason out.
func registerCrushGuardSteps(suite *bddgo.Suite[State]) {
	suite.Step(`^crush's write policy lets the turn write under "([^"]+)"$`, grantWriteRoot)
	suite.Step(`^crush's write policy lets the turn run the shell$`, grantShell)
	suite.Step(`^crush hands the guard no write policy$`, withholdPolicy)
	suite.Step(`^crush hands the guard an unreadable write policy$`, garblePolicy)
	suite.Step(`^crush hands the guard a write policy with no read root$`, omitReadRoot)
	suite.Step(`^crush pipes the hook payload naming the tool "([^"]+)" and the path "([^"]+)"$`, pipePathPayload)
	suite.Step(`^crush's write policy grants the test command "([^"]+)" with selector "([^"]+)" in "([^"]+)"$`,
		grantTestCommand)
	suite.Step(`^crush pipes a shell call in directory "([^"]+)" with command (.+)$`, pipeShellCommand)
	suite.Step(`^crush names the tool "([^"]+)"$`, nameTool)
	suite.Step(`^crush names the file "([^"]+)" of the project tree$`, nameTreeFile)
	suite.Step(`^crush names the path "([^"]+)" as written$`, nameLiteralPath)
	suite.Step(`^crush pipes the hook payload naming the tool "([^"]+)" and the file "([^"]+)"$`, pipePayload)
	suite.Step(`^the project tree's "([^"]+)" is a symbolic link to "([^"]+)"$`, plantLink)
	suite.Step(`^crush asks the guard about the tool call$`, askGuard)
	suite.Step(`^the guard allows the tool call$`, assertGuardAllows)
	suite.Step(`^the guard denies the tool call$`, assertGuardDenies)
	suite.Step(`^the guard's reason matches (.+)$`, assertGuardReason)
}

// guardCall is the tool call a scenario has crush ask the guard about.
// Paths stay relative to the project tree until the When step renders them,
// since only then is the run directory they resolve against in use.
type guardCall struct {
	testCommand  string
	testSelector string
	testDir      string
	command      string
	workingDir   string
	roots        []string
	shell        bool
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
	AllowBash    bool     `json:"allow_bash"`
	TestCommand  string   `json:"test_command,omitempty"`
	TestSelector string   `json:"test_selector,omitempty"`
	TestDir      string   `json:"test_dir,omitempty"`
}

// guardPayload is the subset of the JSON crush pipes to a hook on stdin.
type guardPayload struct {
	ToolName  string             `json:"tool_name"`
	Input     guardPayloadInput  `json:"input"`
	ToolInput *guardPayloadInput `json:"tool_input,omitempty"`
}

type guardPayloadInput struct {
	FilePath   string `json:"file_path,omitempty"`
	Path       string `json:"path,omitempty"`
	Command    string `json:"command,omitempty"`
	WorkingDir string `json:"working_dir,omitempty"`
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

func grantShell(state *State, _ []string) error {
	state.guard().shell = true

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

// askGuard runs the guard once, as crush does before a tool call: every
// variable is set, blank when no step named it, so nothing the test
// process happens to export reaches the guard.
func askGuard(state *State, _ []string) error {
	if state.Fixture == nil {
		return state.fail("%w", ErrNoTreePrepared)
	}

	runDir := runner.RunDir(state.Harness.SessionRoot, state.Fixture.Name)
	call := state.guard()

	policy, err := call.policy(runDir)
	if err != nil {
		return state.fail("rendering the write policy: %w", err)
	}

	if call.piped {
		input := call.payloadInput(runDir)

		hook := guardPayload{ToolName: call.pipedTool, Input: input}
		if call.command != "" {
			hook.ToolInput = &input
		}

		payload, payloadErr := json.Marshal(hook)
		if payloadErr != nil {
			return state.fail("rendering the hook payload: %w", payloadErr)
		}

		state.Fixture.Stdin = payload
	}

	return runEngine(state, guardCommand,
		guardPolicyVar+"="+policy,
		guardToolVar+"="+call.tool,
		guardFilePathVar+"="+call.targetPath(runDir),
		"CRUSH_TOOL_INPUT_COMMAND=")
}

func (c *guardCall) payloadInput(runDir string) guardPayloadInput {
	if c.command != "" {
		return guardPayloadInput{Command: c.command, WorkingDir: c.workingDir}
	}

	if c.pipedPath != "" {
		return guardPayloadInput{Path: c.pipedPath}
	}

	return guardPayloadInput{FilePath: filepath.Join(runDir, c.pipedFile)}
}

// policy renders the write policy crush would export, its roots made
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

	policy := guardPolicy{ReadRoot: readRoot, WriteRoots: roots, AllowBash: c.shell}
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
// of the answer crush acts on.
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
// line crush shows for a denied tool: `stderr matches` in the guard's words.
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

func pipeShellCommand(state *State, args []string) error {
	call := state.guard()
	call.piped, call.pipedTool = true, "bash"
	call.workingDir, call.command = args[0], args[1]

	return nil
}

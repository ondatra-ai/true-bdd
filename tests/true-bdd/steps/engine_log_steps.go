package steps

import (
	"encoding/json"
	"errors"
	"github.com/ondatra-ai/true-bdd/pkg/enginelog"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

// engineLogRelPath is where the CLI writes its structured log, relative
// to a run's tmpdir (paths.tmp_dir in the engine config seed). The engine
// runs with the run dir as cwd, so this resolves under Result.TmpDir.
const engineLogRelPath = "tmp/true-bdd.log.json"

// Log messages the engine emits once per event: "Spawning test runner"
// before every framework subprocess fork, "Dispatching AI turn" before
// every model turn — the engine's own vocabulary, not a stdout phrasing.
const (
	engineMsgSpawnTestRunner = "Spawning test runner"
	engineMsgDispatchAITurn  = enginelog.MsgDispatch
)

// engineLogRecord is the slice of one slog JSON line these assertions
// need: the message, a test-runner spawn's binary and args, an AI-turn
// dispatch's role and CLI. Timings, byte counts and cost stay undecoded.
type engineLogRecord struct {
	Level   string   `json:"level"`
	Version string   `json:"version"`
	Commit  string   `json:"commit"`
	Msg     string   `json:"msg"`
	Binary  string   `json:"binary"`
	Args    []string `json:"args"`
	Role    string   `json:"role"`
	CLI     string   `json:"cli"`
}

// turnRoles maps the kind of turn a step names to the `role` the engine
// stamps on its dispatch record.
func turnRoles() map[string]string {
	return map[string]string{"validation": "prompt", "fix": "fix", "apply": "apply"}
}

// ErrNoEngineLog means a run left no structured log at all — distinct
// from a log recording zero spawns. Treating them the same would let a
// run that never reached bootstrap falsely pass "dispatched no AI turns".
var ErrNoEngineLog = errors.New("the run wrote no engine log")

// readEngineLog parses the CLI's slog file (newline-delimited JSON) into
// records. A blank trailing line or malformed line is skipped, not
// failed: the file is append-mode and only whole records matter here.
func readEngineLog(state *State) ([]engineLogRecord, error) {
	if state.Result == nil {
		return nil, state.fail("%w", ErrNoRun)
	}

	path := filepath.Join(state.Result.TmpDir, engineLogRelPath)

	data, err := disk.Read(path)
	if err != nil {
		return nil, state.fail("%w: %s: %v", ErrNoEngineLog, path, err)
	}

	var records []engineLogRecord

	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		var record engineLogRecord

		if json.Unmarshal([]byte(trimmed), &record) != nil {
			continue
		}

		records = append(records, record)
	}

	return records, nil
}

// countEngineLog counts the records whose msg equals the given message.
func countEngineLog(records []engineLogRecord, msg string) int {
	count := 0

	for _, record := range records {
		if record.Msg == msg {
			count++
		}
	}

	return count
}

// assertTestRunnerSpawned pins that the engine spawned a named framework
// runner at least once — e.g. the "Cannot run …" case, where the spawn
// record is the only proof the binary was reached before the fork failed.
func assertTestRunnerSpawned(state *State, args []string) error {
	want := args[0]

	records, err := readEngineLog(state)
	if err != nil {
		return err
	}

	var spawned []string

	for _, record := range records {
		if record.Msg != engineMsgSpawnTestRunner {
			continue
		}

		if record.Binary == want {
			return nil
		}

		spawned = append(spawned, record.Binary)
	}

	return state.fail(
		"expected the engine to spawn test runner %q, but it spawned %v", want, spawned)
}

// assertTestRunnerArgs pins the argv a spawned runner carried: passes
// when any spawn record's args, space-joined, match the given regexp —
// proof the engine ran the exact command declared, not merely *a* runner.
func assertTestRunnerArgs(state *State, args []string) error {
	pattern, err := regexp.Compile(args[0])
	if err != nil {
		return state.fail("test-runner args pattern %q does not compile: %w", args[0], err)
	}

	records, err := readEngineLog(state)
	if err != nil {
		return err
	}

	var seen []string

	for _, record := range records {
		if record.Msg != engineMsgSpawnTestRunner {
			continue
		}

		joined := strings.Join(record.Args, " ")
		if pattern.MatchString(joined) {
			return nil
		}

		seen = append(seen, joined)
	}

	return state.fail(
		"expected a spawned test runner whose arguments match %q, but the spawns carried %v",
		args[0], seen)
}

// assertTestRunnerCount pins how many framework runners the engine
// spawned — what separates "discovery failed and stopped" (one) from
// "it went on to rerun a fixed test" (another).
func assertTestRunnerCount(state *State, args []string) error {
	want, err := numberSlot(args[0])
	if err != nil {
		return state.fail("test-runner count %q is not a number: %w", args[0], err)
	}

	records, err := readEngineLog(state)
	if err != nil {
		return err
	}

	got := countEngineLog(records, engineMsgSpawnTestRunner)
	if got != want {
		return state.fail(
			"expected the engine to spawn %d test runner(s), but it spawned %d", want, got)
	}

	return nil
}

// assertNoTestRunnerSpawned pins that the engine spawned NO framework
// runner — the startup-refusal case. The engine still wrote a log (its
// "Refusing to start" record), so this passes on zero spawns, not a missing file.
func assertNoTestRunnerSpawned(state *State, _ []string) error {
	records, err := readEngineLog(state)
	if err != nil {
		return err
	}

	var spawned []string

	for _, record := range records {
		if record.Msg == engineMsgSpawnTestRunner {
			spawned = append(spawned, record.Binary)
		}
	}

	if len(spawned) > 0 {
		return state.fail(
			"expected the engine to spawn no test runner, but it spawned %d: %v",
			len(spawned), spawned)
	}

	return nil
}

// assertAITurnCount pins how many model turns the engine dispatched,
// counted at the router's dispatch record so it is one number regardless
// of which CLI a tier named. Zero is the read-only walk that never fixes.
func assertAITurnCount(state *State, args []string) error {
	want, err := numberSlot(args[0])
	if err != nil {
		return state.fail("AI-turn count %q is not a number: %w", args[0], err)
	}

	records, err := readEngineLog(state)
	if err != nil {
		return err
	}

	got := countEngineLog(records, engineMsgDispatchAITurn)
	if got != want {
		return state.fail(
			"expected the engine to dispatch %d AI turn(s), but it dispatched %d", want, got)
	}

	return nil
}

// assertAITurnsTo pins how many turns of one kind (validation, fix or
// apply) the engine dispatched to one AI CLI, read off the dispatch
// record's `role` and `cli`: proof of where a tier routed each turn.
func assertAITurnsTo(state *State, args []string) error {
	want, err := numberSlot(args[0])
	if err != nil {
		return state.fail("AI-turn count %q is not a number: %w", args[0], err)
	}

	kind, cli := args[1], args[2]

	role, known := turnRoles()[kind]
	if !known {
		return state.fail("%q is no kind of AI turn: want validation, fix or apply", kind)
	}

	records, err := readEngineLog(state)
	if err != nil {
		return err
	}

	got := 0
	routed := map[string]int{}

	for _, record := range records {
		if record.Msg != engineMsgDispatchAITurn || record.Role != role {
			continue
		}

		routed[record.CLI]++

		if record.CLI == cli {
			got++
		}
	}

	if got != want {
		return state.fail(
			"expected the engine to dispatch %d %s AI turn(s) to %q, but it dispatched %d; "+
				"its %s turns went to %v", want, kind, cli, got, kind, routed)
	}

	return nil
}

func assertPrebuiltEnginePath(state *State, _ []string) error {
	path, err := filepath.Abs(state.Harness.BinPath)
	if err != nil {
		return state.fail("absolute engine path: %w", err)
	}

	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		return state.fail("resolve engine path: %w", err)
	}

	records, err := readEngineLog(state)
	if err != nil {
		return err
	}

	for _, record := range records {
		if record.Msg != engineMsgSpawnTestRunner {
			continue
		}

		for _, argument := range record.Args {
			if argument == "-engine-bin="+path {
				return nil
			}
		}
	}

	return state.fail("no runner received the running engine path %q as one argument", path)
}

func assertTestRunnerMatchingCount(state *State, args []string) error {
	want, err := numberSlot(args[0])
	if err != nil {
		return state.fail("test-runner count: %w", err)
	}

	pattern, err := regexp.Compile(args[1])
	if err != nil {
		return state.fail("test-runner argument pattern: %w", err)
	}

	records, err := readEngineLog(state)
	if err != nil {
		return err
	}

	got := 0

	for _, record := range records {
		if record.Msg == engineMsgSpawnTestRunner && pattern.MatchString(strings.Join(record.Args, " ")) {
			got++
		}
	}

	if got != want {
		return state.fail("expected %d test runner(s) matching %q, got %d", want, args[1], got)
	}

	return nil
}

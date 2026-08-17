package steps

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// engineLogRelPath is where the CLI under test writes its structured log,
// relative to a run's tmpdir: paths.tmp_dir ("./tmp" in the engine config
// seed) joined with the file the bootstrap logger opens. The engine runs
// with the run dir as its cwd, so this resolves under Result.TmpDir.
const engineLogRelPath = "tmp/true-bdd.log.json"

// Log messages the engine emits once per event, at the two seams these
// assertions read. "Spawning test runner" is written by the testrunner
// package before every framework subprocess fork; "Dispatching AI turn"
// by the AI router before every model turn. Matching on the message binds
// these steps to the engine's own vocabulary rather than to a stdout
// phrasing or a byte count.
const (
	engineMsgSpawnTestRunner = "Spawning test runner"
	engineMsgDispatchAITurn  = "Dispatching AI turn"
)

// engineLogRecord is the slice of one slog JSON line these assertions
// need: the message that names the event, and the binary a test-runner
// spawn carries. Every other field the engine logs — timings, byte
// counts, cost — is left undecoded.
type engineLogRecord struct {
	Msg    string   `json:"msg"`
	Binary string   `json:"binary"`
	Args   []string `json:"args"`
}

// ErrNoEngineLog is returned when a run left no structured log at all,
// which is distinct from a log that recorded no spawns: a run that never
// reached bootstrap cannot answer what the engine did, and a silent zero
// would pass a "dispatched no AI turns" clause for the wrong reason.
var ErrNoEngineLog = errors.New("the run wrote no engine log")

// readEngineLog parses the CLI's slog file (newline-delimited JSON, one
// record per line) into records. A blank trailing line and any line that
// is not a JSON object are skipped rather than failing the read: the file
// is append-mode and these assertions only ask about whole, well-formed
// records.
func readEngineLog(state *State) ([]engineLogRecord, error) {
	if state.Result == nil {
		return nil, state.fail("%w", ErrNoRun)
	}

	path := filepath.Join(state.Result.TmpDir, engineLogRelPath)

	data, err := os.ReadFile(path)
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
// runner at least once — the "Cannot run …" case, where the binary the
// spec names does not exist and the only proof it was reached is the spawn
// record written before the fork.
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

// assertTestRunnerArgs pins the argv a spawned framework runner carried.
// The scenario names a regexp; the step passes when at least one
// "Spawning test runner" record's arguments — joined into one
// space-separated string — match it. This is how a scenario proves the
// engine ran the exact command its spec declared (a `-run ^TestName$`
// filter, a machine-readable flag) rather than merely spawning *a*
// runner. The capture group is a regexp so one definition serves every
// scenario that names a different argv fragment.
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
// spawned. The whole-suite discovery pass is one; a per-fix rerun is
// another — so the count is what separates "discovery failed and stopped"
// from "it went on to rerun a fixed test".
func assertTestRunnerCount(state *State, args []string) error {
	want, err := strconv.Atoi(args[0])
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
// runner at all — the startup-refusal case, where the static pre-pass
// rejects the run before the first subprocess fork, so the proof is the
// ABSENCE of the spawn record every fork writes. The engine still wrote a
// log (its "Refusing to start" record went to slog), so readEngineLog
// succeeds and finding zero spawns is the pass, not the missing file.
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

// assertAITurnCount pins how many model turns the engine dispatched.
// Counted at the router's dispatch record, so it is one number regardless
// of which CLI a tier named. Zero is the read-only walk that never reaches
// a fix.
func assertAITurnCount(state *State, args []string) error {
	want, err := aiTurnCount(args[0])
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

// aiTurnCount reads the count a scenario wrote, accepting the word "no"
// for zero so "dispatched no AI turns" reads as English while
// "dispatched 3 AI turns" still parses as a number.
func aiTurnCount(token string) (int, error) {
	if token == "no" {
		return 0, nil
	}

	count, err := strconv.Atoi(token)
	if err != nil {
		return 0, fmt.Errorf("%q is not a count: %w", token, err)
	}

	return count, nil
}

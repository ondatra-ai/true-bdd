package steps

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"sync"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

// ErrNoConcurrentReads is returned when a clause grades a batch of concurrent
// replies and no When step fired one.
var ErrNoConcurrentReads = errors.New("no When step read concurrently")

const (
	// The three kinds of read the concurrent clause fires, and the words its
	// failures call them by.
	statusRead = "session status"
	detailRead = "session detail"
	runRead    = "run"
)

// concurrentRead is one reply a concurrent read came back with, kept beside the
// error that stopped it: a read that never answered is graded, not dropped.
type concurrentRead struct {
	Path     string
	Response *apiResponse
	Err      error
}

// registerConcurrentReadSteps binds the correlation vocabulary: three kinds of
// read in flight at once, and what each reply is then held to carry.
func registerConcurrentReadSteps(suite *bddgo.Suite[State]) {
	suite.Step(
		`^the (Product Owner|System Architect|Quality Engineer) reads the session status, `+
			`the session detail and run "([^"]+)" concurrently (\d+) times$`,
		readConcurrently)
	suite.Step(`^every session status reply names the session it was asked about$`,
		assertStatusRepliesCorrelated)
	suite.Step(`^every session detail reply names that session and carries an inventory$`,
		assertDetailRepliesCorrelated)
	suite.Step(`^every run reply names run "([^"]+)" and that session$`,
		assertRunRepliesCorrelated)
}

// readConcurrently fires all three reads together, count times each: routing by
// correlation rather than by arrival order is only observable while the replies
// overlap. args[0] is the role, discarded as openPath's is.
func readConcurrently(state *State, args []string) error {
	label := args[1]

	times, err := strconv.Atoi(args[2])
	if err != nil {
		return state.fail("the step names %q times, which is not a number: %w", args[2], err)
	}

	session, err := ensureSession(state)
	if err != nil {
		return err
	}

	runReadPath, err := runPath(state, label)
	if err != nil {
		return err
	}

	state.ReadSession = session.SessionID
	state.Reads = fireReads(state, map[string]string{
		statusRead: fmt.Sprintf("%s/%s?view=status", sessionsPath, session.SessionID),
		detailRead: fmt.Sprintf("%s/%s", sessionsPath, session.SessionID),
		runRead:    runReadPath,
	}, times)

	return nil
}

// fireReads sends every read on its own goroutine and waits for all of them.
// Each writes its own pre-sized slot, so the batch needs no lock and nothing
// serialises it behind the slowest reply.
func fireReads(state *State, paths map[string]string, times int) map[string][]concurrentRead {
	reads := make(map[string][]concurrentRead, len(paths))
	for kind := range paths {
		reads[kind] = make([]concurrentRead, times)
	}

	relayURL := state.RelayURL

	var group sync.WaitGroup

	for kind, path := range paths {
		for index := range times {
			group.Add(1)

			go func() {
				defer group.Done()

				response, err := apiGet(relayURL, path)
				reads[kind][index] = concurrentRead{Path: path, Response: response, Err: err}
			}()
		}
	}

	group.Wait()

	return reads
}

// correlatedReply is what every reply in the batch is graded on: which session
// and run it names, and whether a detail read carried its inventory.
type correlatedReply struct {
	SessionID string          `json:"session_id"`
	RunID     string          `json:"run_id"`
	Inventory json.RawMessage `json:"inventory"`
}

// gradeReads holds every reply of one kind to check, naming the first read that
// failed, which route it was, and what it answered instead.
func gradeReads(state *State, kind string, check func(*apiResponse) string) error {
	reads := state.Reads[kind]
	if len(reads) == 0 {
		return state.fail("%w: no %s reply to grade", ErrNoConcurrentReads, kind)
	}

	for index, read := range reads {
		if read.Err != nil {
			return state.fail("%s reply %d of %d (GET %s) never answered: %w",
				kind, index+1, len(reads), read.Path, read.Err)
		}

		if read.Response.Status != http.StatusOK {
			return state.fail("%s reply %d of %d (GET %s) returned %d, want 200: %s",
				kind, index+1, len(reads), read.Path, read.Response.Status,
				read.Response.snippet())
		}

		fault := check(read.Response)
		if fault != "" {
			return state.fail("%s reply %d of %d (GET %s) %s: %s",
				kind, index+1, len(reads), read.Path, fault, read.Response.snippet())
		}
	}

	return nil
}

// decodeReply reads the correlated fields off one reply, rendering a decode
// failure as the fault the grader reports rather than as an error of its own.
func decodeReply(response *apiResponse) (correlatedReply, string) {
	var reply correlatedReply

	err := json.Unmarshal(response.Body, &reply)
	if err != nil {
		return reply, fmt.Sprintf("is not JSON (%s)", err)
	}

	return reply, ""
}

// assertStatusRepliesCorrelated holds every status reply to naming the session
// it was asked about — one routed by arrival order would name another.
func assertStatusRepliesCorrelated(state *State, _ []string) error {
	return gradeReads(state, statusRead, func(response *apiResponse) string {
		reply, fault := decodeReply(response)
		if fault != "" {
			return fault
		}

		if reply.SessionID != state.ReadSession {
			return fmt.Sprintf("names session %q, want the session it was asked about, %q",
				reply.SessionID, state.ReadSession)
		}

		return ""
	})
}

// assertDetailRepliesCorrelated holds every detail reply to the same session AND
// to carrying its inventory: a detail reply served as a status one is correlated
// and still the wrong view.
func assertDetailRepliesCorrelated(state *State, _ []string) error {
	return gradeReads(state, detailRead, func(response *apiResponse) string {
		reply, fault := decodeReply(response)
		if fault != "" {
			return fault
		}

		if reply.SessionID != state.ReadSession {
			return fmt.Sprintf("names session %q, want the session it was asked about, %q",
				reply.SessionID, state.ReadSession)
		}

		if len(reply.Inventory) == 0 || string(reply.Inventory) == "null" {
			return "carries no inventory"
		}

		return ""
	})
}

// assertRunRepliesCorrelated holds every run reply to naming the run it was
// asked for and the session it was read through.
func assertRunRepliesCorrelated(state *State, args []string) error {
	label := args[0]

	runID, err := lookupID(state, label)
	if err != nil {
		return err
	}

	return gradeReads(state, runRead, func(response *apiResponse) string {
		reply, fault := decodeReply(response)
		if fault != "" {
			return fault
		}

		if reply.RunID != runID {
			return fmt.Sprintf("names run %q, want run %q, which %q names",
				reply.RunID, runID, label)
		}

		if reply.SessionID != state.ReadSession {
			return fmt.Sprintf("names session %q, want the session it was read through, %q",
				reply.SessionID, state.ReadSession)
		}

		return ""
	})
}

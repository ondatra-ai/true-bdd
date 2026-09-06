package steps

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/cli/ps"
	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

// ErrNoCommandChild is returned when a clause is about the child a run was
// bound to and no step recorded one while it was alive.
var ErrNoCommandChild = errors.New("no step recorded the run's command child")

// ErrNoDescendantGroup is returned when the survival clause runs and no Given
// step waited for a live descendant to record its group.
var ErrNoDescendantGroup = errors.New("no Given step waited for a live agent descendant")

const (
	// remoteRefPattern is how a clause names a remote: the scenario's own, or
	// the one a Given labelled. Spliced into both descendant clauses, so the two
	// phrasings resolve through one definition.
	remoteRefPattern = `the remote|remote "[^"]+"`
	// namedRemotePrefix is how a labelled reference opens.
	namedRemotePrefix = `remote "`
	// theRunRef is how a clause names its run without labelling it, resolved as
	// the sequence-number clause resolves it: the run the scenario last saw
	// blocked on a prompt.
	theRunRef = "the run"
	// descendantTimeout is how long the command child has to spawn one: the CLI
	// boots, reads the checklist and reaches its first model call inside it.
	descendantTimeout = 120 * time.Second
	// descendantGoneTimeout is how long the group has to empty after the
	// interrupt — the CLI escalates SIGINT to SIGKILL before it exits itself.
	descendantGoneTimeout = 60 * time.Second
	// childrenPidfileDir and the two halves of the name mirror the CLI's own
	// (remote.childrenPidfilePath): one pidfile per owner, so two remotes in one
	// tree never clobber each other's children.
	childrenPidfileDir    = "tmp"
	childrenPidfilePrefix = "true-bdd-remote-children."
	childrenPidfileSuffix = ".jsonl"
)

// commandChild is one line of that pidfile: the group the CLI ran the run's
// command in, the start identity that tells it from a recycled pid, and the run
// it belongs to.
type commandChild struct {
	PGID          int    `json:"pgid"`
	StartIdentity string `json:"start_identity"`
	RunID         string `json:"run_id"`
}

// registerCommandChildSteps binds the vocabulary about the process a run's
// command actually runs in: that it survived a relay restart, that it had a
// live descendant, and that an interrupt left none of them running.
func registerCommandChildSteps(suite *bddgo.Suite[State]) {
	suite.Step(`^(`+theRunRef+`|`+runRefPattern+`)'s command child is the same `+
		`process group it was before$`, assertCommandChildUnchanged)
	suite.Step(`^(`+remoteRefPattern+`) has a live agent descendant$`, awaitAgentDescendant)
	suite.Step(`^no agent descendant of (`+remoteRefPattern+`) survives$`,
		assertNoAgentDescendant)
}

// assertCommandChildUnchanged holds the run to having been answered by the very
// child it was blocked in before the relay went down: an answer consumed by a
// respawned child is one the reader never sent to the run they were watching.
func assertCommandChildUnchanged(state *State, args []string) error {
	label, err := restartedRunLabel(state, args[0])
	if err != nil {
		return err
	}

	before, restarted := state.ChildBeforeRestart[label]
	if !restarted {
		return state.fail("%w: run %q, when the relay restarted (%s)",
			ErrNoCommandChild, label, commandChildrenPath(state, label))
	}

	after, answered := state.ChildAtAnswer[label]
	if !answered {
		return state.fail("%w: run %q, when its prompt was answered (%s)",
			ErrNoCommandChild, label, commandChildrenPath(state, label))
	}

	if after != before {
		return state.fail("run %q's answer went to command child group %d started %q, "+
			"want the group %d started %q it was blocked in before the restart",
			label, after.PGID, after.StartIdentity, before.PGID, before.StartIdentity)
	}

	return nil
}

// restartedRunLabel resolves the run a clause names, or the one the scenario
// last saw blocked on a prompt when it names none.
func restartedRunLabel(state *State, ref string) (string, error) {
	if ref != theRunRef {
		return runLabelOf(state, ref)
	}

	if state.Prompted == "" {
		return "", state.fail("%w", ErrNoPrompt)
	}

	return state.Prompted, nil
}

// awaitAgentDescendant waits until a group the remote's command child runs in
// holds a process besides its own leader. pkg/cli/claude spawns the agent CLI
// INSIDE that group, so a second member IS a live descendant.
func awaitAgentDescendant(state *State, args []string) error {
	sessionID, err := remoteSessionID(state, args[0])
	if err != nil {
		return err
	}

	deadline := time.Now().Add(descendantTimeout)

	for {
		group, reason, readErr := groupWithDescendant(state, sessionID)

		switch {
		case readErr != nil:
			reason = readErr.Error()
		case group != 0:
			state.CommandGroups = append(state.CommandGroups, group)

			return nil
		}

		if !time.Now().Before(deadline) {
			return state.fail("the remote never had a live agent descendant within %s: %s",
				descendantTimeout, reason)
		}

		time.Sleep(runPollInterval)
	}
}

// groupWithDescendant answers which recorded command group holds more than its
// own leader, and what the groups held when none does.
func groupWithDescendant(state *State, sessionID string) (int, string, error) {
	children, err := readCommandChildren(state, sessionID)
	if err != nil {
		return 0, "", err
	}

	if len(children) == 0 {
		return 0, "the remote records no command child at all", nil
	}

	for _, child := range children {
		members, memberErr := ps.GroupMembers(child.PGID)
		if memberErr != nil {
			return 0, "", state.fail("reading process group %d: %w", child.PGID, memberErr)
		}

		if len(members) > 1 {
			return child.PGID, "", nil
		}
	}

	return 0, "every command child is alone in its group", nil
}

// assertNoAgentDescendant holds every group the remote's command child was seen
// running in to being empty: a descendant that outlived the interrupt is a model
// call still spending on a run nobody is watching.
func assertNoAgentDescendant(state *State, args []string) error {
	_, err := remoteSessionID(state, args[0])
	if err != nil {
		return err
	}

	if len(state.CommandGroups) == 0 {
		return state.fail("%w", ErrNoDescendantGroup)
	}

	deadline := time.Now().Add(descendantGoneTimeout)

	for {
		alive, readErr := survivingMembers(state)

		var reason string

		switch {
		case readErr != nil:
			reason = readErr.Error()
		case len(alive) == 0:
			return nil
		default:
			reason = describeSurvivors(alive)
		}

		if !time.Now().Before(deadline) {
			return state.fail("the remote's command groups still hold %s after %s, "+
				"want no descendant of it left running", reason, descendantGoneTimeout)
		}

		time.Sleep(runPollInterval)
	}
}

// survivingMembers is every process still running in a group this scenario saw
// a command child in, its leader included: the leader IS the command child, and
// an interrupt must take it down with what it spawned.
func survivingMembers(state *State) (map[int][]int, error) {
	alive := map[int][]int{}

	for _, group := range state.CommandGroups {
		members, err := ps.GroupMembers(group)
		if err != nil {
			return nil, state.fail("reading process group %d: %w", group, err)
		}

		if len(members) > 0 {
			alive[group] = members
		}
	}

	return alive, nil
}

// describeSurvivors renders what is still running, so the failure names the pids
// a reader can look up rather than only their number.
func describeSurvivors(alive map[int][]int) string {
	groups := make([]string, 0, len(alive))

	for group, members := range alive {
		pids := make([]string, 0, len(members))

		for _, pid := range members {
			pids = append(pids, strconv.Itoa(pid))
		}

		groups = append(groups, fmt.Sprintf("group %d: %s", group, strings.Join(pids, ", ")))
	}

	sort.Strings(groups)

	return strings.Join(groups, "; ")
}

// noteCommandChild files the child a run is bound to RIGHT NOW, called from the
// two steps that run while it is demonstrably alive, since the CLI drops the
// entry as soon as the child exits.
func noteCommandChild(state *State, label string, into map[string]commandChild) {
	if state.Tree == nil {
		return
	}

	sessionID, dispatched := state.RunSessions[label]

	runID, filed := state.Runs[label]
	if !dispatched || !filed {
		return
	}

	children, err := readCommandChildren(state, sessionID)
	if err != nil {
		return
	}

	child, live := children[runID]
	if live {
		into[label] = child
	}
}

// readCommandChildren reads the owner's children pidfile — the JSONL the CLI
// rewrites as it spawns and reaps command children — keyed by the run each child
// is running. An absent file is no children, which is not a failure.
func readCommandChildren(state *State, sessionID string) (map[string]commandChild, error) {
	path := childrenPidfilePath(state.Tree.Dir, sessionID)

	data, err := disk.Read(path)
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]commandChild{}, nil
	}

	if err != nil {
		return nil, state.fail("reading the children pidfile %s: %w", path, err)
	}

	children := map[string]commandChild{}

	for _, line := range strings.Split(string(data), "\n") {
		var entry commandChild

		if json.Unmarshal([]byte(strings.TrimSpace(line)), &entry) != nil {
			continue
		}

		children[entry.RunID] = entry
	}

	return children, nil
}

// commandChildrenPath is the pidfile the run's own owner writes, or why it could
// not be named — a failure pointing at no file sends the reader hunting.
func commandChildrenPath(state *State, label string) string {
	if state.Tree == nil {
		return "the scenario named no project tree"
	}

	sessionID, dispatched := state.RunSessions[label]
	if !dispatched {
		return "the scenario recorded no session for run " + label
	}

	return childrenPidfilePath(state.Tree.Dir, sessionID)
}

// childrenPidfilePath mirrors the CLI's own naming:
// <folder>/tmp/true-bdd-remote-children.<owner>.jsonl.
func childrenPidfilePath(treeDir, sessionID string) string {
	return filepath.Join(treeDir, childrenPidfileDir,
		childrenPidfilePrefix+sessionID+childrenPidfileSuffix)
}

// remoteSessionID resolves the owner id of the remote a clause names — the
// scenario's own, or the one a Given labelled — since the children pidfile is
// keyed on it.
func remoteSessionID(state *State, ref string) (string, error) {
	if state.Tree == nil {
		return "", state.fail("%w", ErrNoProjectTree)
	}

	if !strings.HasPrefix(ref, namedRemotePrefix) {
		session, err := ensureSession(state)
		if err != nil {
			return "", err
		}

		return session.SessionID, nil
	}

	session, err := ensureNamedSession(state,
		strings.TrimSuffix(strings.TrimPrefix(ref, namedRemotePrefix), `"`))
	if err != nil {
		return "", err
	}

	return session.SessionID, nil
}

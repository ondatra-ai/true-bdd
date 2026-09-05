package steps

import (
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/cli"
	"github.com/ondatra-ai/true-bdd/pkg/cli/spec"
	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

// registerTreeChangeSteps binds what the run did to the host project: the
// mutation oracle every fix scenario closes with.
func registerTreeChangeSteps(suite *bddgo.Suite[State]) {
	suite.Step(`^no file in the project tree changed$`, assertTreeUnchanged)
	suite.Step(`^no file was modified$`, assertNothingModified)
	suite.Step(`^the only files? changed match "([^"]+)"$`, assertOnlyChangesMatch)
	suite.Step(`^the only file changed is "([^"]+)"$`, assertOnlyFileChanged)
	suite.Step(`^at least (\d+) files? matching "([^"]+)" is created$`, assertFilesCreated)
	suite.Step(`^the file "([^"]+)" is (created|modified|unchanged)$`, assertFileChange)
}

// treeChange is what the scenario did to the project tree.
type treeChange struct {
	Created  []string
	Modified []string
	Deleted  []string
}

// changed is every path it touched, ordered, which a failure names.
func (change treeChange) changed() []string {
	all := slices.Concat(change.Created, change.Modified, change.Deleted)
	sort.Strings(all)

	return all
}

// treeChanges diffs the tree as it stands against the baseline the materializer
// took before the scenario ran.
func treeChanges(state *State) (treeChange, error) {
	if state.Tree == nil {
		return treeChange{}, state.fail("%w", ErrNoProjectTree)
	}

	now, err := currentTreeHash(state)
	if err != nil {
		return treeChange{}, err
	}

	var change treeChange

	for path, sum := range now {
		before, existed := state.Tree.Baseline[path]

		switch {
		case !existed:
			change.Created = append(change.Created, path)
		case before != sum:
			change.Modified = append(change.Modified, path)
		}
	}

	for path := range state.Tree.Baseline {
		_, still := now[path]
		if !still {
			change.Deleted = append(change.Deleted, path)
		}
	}

	return change, nil
}

// currentTreeHash re-hashes the tree through the SAME materializer that took the
// baseline. Re-implementing its exclusions here — root-level tmp/ among them — is
// the drift the shared binary exists to prevent.
func currentTreeHash(state *State) (map[string]string, error) {
	binary, err := state.Harness.MaterializerBinary()
	if err != nil {
		return nil, state.fail("%w", err)
	}

	finished, err := spec.Run([]string{binary, "-list-baseline", "-target", state.Tree.Dir},
		cli.Options{Timeout: materializeTimeout})
	if err == nil {
		err = finished.Err()
	}

	if err != nil {
		return nil, state.fail("re-hashing %s: %w\n%s", state.Tree.Dir, err, finished.Stderr)
	}

	var result struct {
		Baseline map[string]string `json:"baseline"`
	}

	err = json.Unmarshal([]byte(finished.Stdout), &result)
	if err != nil {
		return nil, state.fail("read the re-hash of %s: %w\n%s",
			state.Tree.Dir, err, finished.Stdout)
	}

	return result.Baseline, nil
}

// assertTreeUnchanged is the read-only clause: a run that answered a prompt with
// exit must have written nothing at all.
func assertTreeUnchanged(state *State, _ []string) error {
	change, err := treeChanges(state)
	if err != nil {
		return err
	}

	touched := change.changed()
	if len(touched) > 0 {
		return state.fail("the run changed %s, want the project tree untouched",
			strings.Join(touched, ", "))
	}

	return nil
}

// assertNothingModified allows creation and forbids rewriting — what separates
// authoring a missing test from editing one that was already there.
func assertNothingModified(state *State, _ []string) error {
	change, err := treeChanges(state)
	if err != nil {
		return err
	}

	if len(change.Modified) > 0 {
		return state.fail("the run modified %s, want it to have modified nothing",
			strings.Join(change.Modified, ", "))
	}

	return nil
}

// assertOnlyChangesMatch holds every path the run touched to the glob the step
// names, and to there being at least one: a run that changed nothing would pass an
// "only" clause on a technicality it was never written for.
func assertOnlyChangesMatch(state *State, args []string) error {
	change, err := treeChanges(state)
	if err != nil {
		return err
	}

	touched := change.changed()
	if len(touched) == 0 {
		return state.fail("the run changed no file, want what it changed to match %q", args[0])
	}

	for _, path := range touched {
		matched, matchErr := matchesGlob(args[0], path)
		if matchErr != nil {
			return state.fail("%w", matchErr)
		}

		if !matched {
			return state.fail("the run changed %q, which does not match %q; it changed %s",
				path, args[0], strings.Join(touched, ", "))
		}
	}

	return nil
}

// assertOnlyFileChanged holds the run to one named path and nothing beside it.
func assertOnlyFileChanged(state *State, args []string) error {
	change, err := treeChanges(state)
	if err != nil {
		return err
	}

	touched := change.changed()
	if len(touched) != 1 || touched[0] != args[0] {
		return state.fail("the run changed %s, want only %q",
			strings.Join(touched, ", "), args[0])
	}

	return nil
}

// assertFilesCreated holds the run to having created at least that many files
// matching the glob — the clause for a file whose name is unknowable in advance.
func assertFilesCreated(state *State, args []string) error {
	want, err := strconv.Atoi(args[0])
	if err != nil {
		return state.fail("the step names %q files, which is not a number: %w", args[0], err)
	}

	change, err := treeChanges(state)
	if err != nil {
		return err
	}

	created := 0

	for _, path := range change.Created {
		matched, matchErr := matchesGlob(args[1], path)
		if matchErr != nil {
			return state.fail("%w", matchErr)
		}

		if matched {
			created++
		}
	}

	if created < want {
		return state.fail("the run created %d file(s) matching %q, want at least %d; it created %s",
			created, args[1], want, strings.Join(change.Created, ", "))
	}

	return nil
}

// assertFileChange holds one named path to the fate the step gives it.
func assertFileChange(state *State, args []string) error {
	change, err := treeChanges(state)
	if err != nil {
		return err
	}

	got := fateOf(change, args[0])
	if got != args[1] {
		return state.fail("%q is %s, want it %s", args[0], got, args[1])
	}

	return nil
}

// fateOf names what became of one path, in the words the step writes.
func fateOf(change treeChange, path string) string {
	switch {
	case slices.Contains(change.Created, path):
		return "created"
	case slices.Contains(change.Modified, path):
		return "modified"
	case slices.Contains(change.Deleted, path):
		return "deleted"
	default:
		return "unchanged"
	}
}

// matchesGlob answers whether a path matches the pattern a clause wrote. `**`
// spans separators, which path.Match cannot express, so the pattern is compiled to
// a regexp rather than handed to it.
func matchesGlob(pattern, target string) (bool, error) {
	var builder strings.Builder

	builder.WriteString("^")

	for index := 0; index < len(pattern); index++ {
		rest := pattern[index:]

		switch {
		case strings.HasPrefix(rest, "**/"):
			builder.WriteString("(?:.*/)?")

			index += 2
		case strings.HasPrefix(rest, "**"):
			builder.WriteString(".*")

			index++
		case pattern[index] == '*':
			builder.WriteString("[^/]*")
		case pattern[index] == '?':
			builder.WriteString("[^/]")
		default:
			builder.WriteString(regexp.QuoteMeta(pattern[index : index+1]))
		}
	}

	builder.WriteString("$")

	matcher, err := regexp.Compile(builder.String())
	if err != nil {
		return false, fmt.Errorf("the glob %q does not compile: %w", pattern, err)
	}

	return matcher.MatchString(target), nil
}

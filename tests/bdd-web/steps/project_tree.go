package steps

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/cli"
	"github.com/ondatra-ai/true-bdd/pkg/cli/spec"
	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

// ErrNoFixture is returned when a scenario names a project tree this
// suite has no fixture for.
var ErrNoFixture = errors.New("no such fixture under tests/bdd-web/fixtures")

// ErrNoProjectTree is returned when a step needs a host project and no
// Given step named one.
var ErrNoProjectTree = errors.New("no Given step named a project tree")

// materializeTimeout caps the materializer, whose prep commands may
// install a fixture's dependencies.
const materializeTimeout = 10 * time.Minute

// ProjectTree is one scenario's materialized host project: the folder a
// remote runs in, and the symlink it may be reached through. Dir has
// every link resolved, so a clause against it is against the real path.
type ProjectTree struct {
	// Name is the fixture this tree was materialized from.
	Name string
	// Dir is the tree's real path.
	Dir string
	// Link is a symlink to Dir, empty until a step makes one.
	Link string
	// Baseline is the tree hash the materializer took after prep and before the
	// scenario ran — every mutation clause's only evidence, since nothing else
	// records what the tree looked like before.
	Baseline map[string]string
}

// materializeTree prepares the named fixture in a fresh directory under
// tmp/, one per scenario. The shared materializer binary owns the layering
// rules; re-implementing them here is the drift it exists to prevent.
func materializeTree(harness *Harness, scenarioID, name string) (*ProjectTree, error) {
	binary, err := harness.MaterializerBinary()
	if err != nil {
		return nil, err
	}

	fixture := filepath.Join(harness.RepoRoot, "tests", "bdd-web", "fixtures", name)

	_, err = os.Stat(filepath.Join(fixture, "fixture.yaml"))
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrNoFixture, fixture)
	}

	scenarioDir := filepath.Join(harness.RepoRoot, "tmp", "bdd-web-run", scenarioID)

	err = disk.RemoveTree(scenarioDir)
	if err != nil {
		return nil, fmt.Errorf("clear %s: %w", scenarioDir, err)
	}

	target, baseline, err := runMaterializer(binary, fixture,
		filepath.Join(scenarioDir, "host"), harness.RepoRoot)
	if err != nil {
		return nil, err
	}

	canonical, err := filepath.EvalSymlinks(target)
	if err != nil {
		return nil, fmt.Errorf("resolve %s to its real path: %w", target, err)
	}

	return &ProjectTree{Name: name, Dir: canonical, Baseline: baseline}, nil
}

// runMaterializer spawns the materializer and returns the target it
// reports. stdout is the JSON result and stderr the diagnostics, so a
// failure carries the materializer's own one-line reason.
func runMaterializer(binary, fixture, target, repoRoot string) (string, map[string]string, error) {
	finished, err := spec.Run(
		[]string{binary, "-fixture", fixture, "-target", target, "-repo", repoRoot},
		cli.Options{Timeout: materializeTimeout})
	if err == nil {
		err = finished.Err()
	}

	if err != nil {
		return "", nil, fmt.Errorf("materialize %s: %w\n%s", fixture, err, finished.Stderr)
	}

	var result struct {
		Target   string            `json:"target"`
		Baseline map[string]string `json:"baseline"`
	}

	err = json.Unmarshal([]byte(finished.Stdout), &result)
	if err != nil {
		return "", nil, fmt.Errorf("read the materializer's result: %w\n%s", err, finished.Stdout)
	}

	return result.Target, result.Baseline, nil
}

// linkThrough puts a symlink to the tree beside it. A remote started
// through the link registers what it resolves rather than what it was
// handed, which is the only way to tell a canonical folder from a lazy one.
func (tree *ProjectTree) linkThrough() (string, error) {
	link := filepath.Join(filepath.Dir(tree.Dir), "host-via-symlink")

	err := os.Symlink(tree.Dir, link)
	if err != nil {
		return "", fmt.Errorf("link %s to %s: %w", link, tree.Dir, err)
	}

	tree.Link = link

	return link, nil
}

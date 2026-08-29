package remote

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// buildTrueBDD builds the real true-bdd binary (which carries the hidden
// remote-supervisor subcommand) so the gated-supervisor mechanics can be
// exercised end-to-end. It skips when the go toolchain is unavailable.
func buildTrueBDD(t *testing.T) string {
	t.Helper()

	_, lookErr := exec.LookPath("go")
	if lookErr != nil {
		t.Skip("go toolchain unavailable")
	}

	bin := filepath.Join(t.TempDir(), "true-bdd")
	ctx := context.Background()

	const mainPkg = "github.com/ondatra-ai/true-bdd/services/bdd-cli"

	//nolint:gosec // argv is literals and a TempDir path.
	build := exec.CommandContext(ctx, "go", "build", "-o", bin, mainPkg)
	build.Env = os.Environ()

	out, buildErr := build.CombinedOutput()
	if buildErr != nil {
		t.Fatalf("build true-bdd: %v\n%s", buildErr, out)
	}

	return bin
}

// TestGatedSupervisorFailsClosedWithoutRelease is the regression for the
// spawn-before-bookkeeping crash window (finding 4): if the parent "crashes"
// (aborts the gate) BEFORE recording the identity, the supervisor EOF-exits WITHOUT ever running the command.
func TestGatedSupervisorFailsClosedWithoutRelease(t *testing.T) {
	bin := buildTrueBDD(t)

	child, err := spawnGatedGroup(spawnConfig{
		binPath: bin, args: []string{commandVersion}, env: os.Environ(), dir: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("spawn gated supervisor: %v", err)
	}

	var out bytes.Buffer

	readDone := make(chan struct{})

	go func() { _, _ = io.Copy(&out, child.stdout); close(readDone) }()

	// Simulate a bookkeeping crash BEFORE release: abort the gate.
	child.abortGate()

	waitErr := child.Wait()

	<-readDone

	if strings.Contains(out.String(), "true-bdd version") {
		t.Fatalf("the command ran despite no release — crash window not closed (finding 4): %q", out.String())
	}

	if waitErr != nil {
		t.Fatalf("the gated supervisor should exit cleanly on gate abort: %v", waitErr)
	}
}

// TestGatedSupervisorRunsAfterRelease proves the happy path: once the parent
// records the identity and releases the gate, the supervisor execs the real
// command inside its group and propagates its exit (finding 4).
func TestGatedSupervisorRunsAfterRelease(t *testing.T) {
	bin := buildTrueBDD(t)

	child, err := spawnGatedGroup(spawnConfig{
		binPath: bin, args: []string{commandVersion}, env: os.Environ(), dir: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("spawn gated supervisor: %v", err)
	}

	var out bytes.Buffer

	readDone := make(chan struct{})

	go func() { _, _ = io.Copy(&out, child.stdout); close(readDone) }()

	relErr := child.Release()
	if relErr != nil {
		t.Fatalf("release: %v", relErr)
	}

	waitErr := child.Wait()

	<-readDone

	if waitErr != nil {
		t.Fatalf("version via the supervisor should exit 0: %v", waitErr)
	}

	if !strings.Contains(out.String(), "true-bdd version") {
		t.Fatalf("released supervisor must run the command: %q", out.String())
	}
}

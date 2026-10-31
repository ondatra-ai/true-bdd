package stepskeleton_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/stepskeleton"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/cli/git"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/stepcoverage"
)

func TestSkeletonCompilesAndFailsWithItsConceptBeforeBodyIsWritten(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	definition, err := stepskeleton.WriteConcept(
		filepath.Join(dir, "steps"), "then", "the command", `\Athe command "([^"]+)" gets ((?:[0-9]+|no))\z`)
	if err != nil {
		t.Fatal(err)
	}

	err = definition.Intact(filepath.Join(dir, "steps"))
	if err != nil {
		t.Fatal(err)
	}

	suite, err := stepcoverage.LoadSuite(filepath.Join(dir, "steps"))
	if err != nil {
		t.Fatal(err)
	}

	if bound := suite.Bind(`the command "double" gets 2`); len(bound) != 1 {
		t.Fatalf("want one binding: %+v", bound)
	}

	for file, content := range map[string]string{
		"go.mod": "module host\n\ngo 1.24\n",
		"steps/skeleton_test.go": `package steps
import ("strings"; "testing")
type registrar struct { pattern string; body func(*State, []string) error }
func (r *registrar) Step(p string, f func(*State, []string) error) { r.pattern, r.body = p, f }
func TestPending(t *testing.T) {
 r := &registrar{}; Register(r)
 err := r.body(&State{}, []string{"double", "2"})
 if err == nil || !strings.Contains(err.Error(), "step not written yet") ||
  !strings.Contains(err.Error(), "the command") { t.Fatal(err) }
}
`,
	} {
		err := disk.Write(filepath.Join(dir, file), []byte(content), disk.Shared)
		if err != nil {
			t.Fatal(err)
		}
	}

	cmd := exec.CommandContext(t.Context(), "go", "test", "./steps")

	cmd.Dir = dir
	cmd.Env = git.ScrubEnvironment(os.Environ())

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("skeleton must compile and expose harness shape: %s: %v", out, err)
	}

	body, err := disk.Read(definition.File)
	if err != nil || !strings.Contains(string(body), "args []string") {
		t.Fatalf("missing captured arguments: %s %v", body, err)
	}
}

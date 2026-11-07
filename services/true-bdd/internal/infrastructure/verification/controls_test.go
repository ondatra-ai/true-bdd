package verification_test

import (
	"encoding/json"
	"fmt"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/cli/git"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

type knownVerdict struct {
	Control  string `json:"control"`
	Expected string `json:"expected_verdict"`
	Category string `json:"category"`
	Pattern  string `json:"pattern"`
}

func TestExactDoubleControlHasAPatternScopeDefect(t *testing.T) {
	control := exactDoubleVerdict(t)
	if control.Expected != "fail" || control.Category != "pattern_scope" {
		t.Fatal(control)
	}

	helper, err := os.ReadFile("testdata/exact_double.go.txt")
	if err != nil {
		t.Fatal(err)
	}

	directory := t.TempDir()
	for path, source := range map[string]string{
		"go.mod":    "module verification/control\n\ngo 1.25.0\n",
		"helper.go": string(helper),
		"witness_test.go": fmt.Sprintf(`package helpers
import("errors";"regexp";"testing")
func TestWitness(t *testing.T) {
 captures:=regexp.MustCompile(%q).FindStringSubmatch("the Owner exactly doubles 6")
 if len(captures)!=4 { t.Fatal(captures) }
 command,exact,value,err:=OperationFromStep(captures[1:])
 if err!=nil || command!="double" || !exact || value!="6" {
 t.Fatalf("captures did not select exact doubling: %%v %%v",captures,err) }
 _,err=RunCalc(command,exact,value)
 if !errors.Is(err,ErrExactNotDeclared) { t.Fatalf("wanted the undeclared exact-double refusal, got %%v",err) }
}
`, control.Pattern),
	} {
		err = os.WriteFile(filepath.Join(directory, path), []byte(source), 0600)
		if err != nil {
			t.Fatal(err)
		}
	}

	command := exec.CommandContext(t.Context(), "go", "test", "./...")
	command.Dir = directory

	command.Env = append(git.ScrubEnvironment(os.Environ()), "GOWORK=off")

	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("source-level control witness: %v\n%s", err, output)
	}
}

func TestInstallationControlNamesTheTestProcessWorkingDirectory(t *testing.T) {
	path := "../../../../../tests/true-bdd/fixtures/build-steps-critique-correct/input/tests/calc/helpers/build.go"

	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	match := regexp.MustCompile(`(?m)^const HostRoot = "([^"]+)"$`).FindStringSubmatch(string(source))
	if len(match) != 2 {
		t.Fatal("fixture has no HostRoot declaration")
	}

	if filepath.Clean(filepath.Join("tests/calc", match[1])) != "." {
		t.Fatalf("HostRoot=%q", match[1])
	}

	if !strings.Contains(string(source), "HostRoot is relative to the test process working directory, tests/calc.") {
		t.Fatal("HostRoot comment must name the process working directory")
	}

	if strings.Contains(string(source), "tests/calc/steps") {
		t.Fatal("HostRoot comment still names the package directory")
	}
}

func exactDoubleVerdict(t *testing.T) knownVerdict {
	t.Helper()

	controls, err := os.ReadFile("testdata/verdicts.json")
	if err != nil {
		t.Fatal(err)
	}

	var verdicts []knownVerdict

	err = json.Unmarshal(controls, &verdicts)
	if err != nil {
		t.Fatal(err)
	}

	control := knownVerdict{}

	for _, verdict := range verdicts {
		if verdict.Control == "concept-bodies-cases-claude-009" {
			control = verdict
		}
	}

	return control
}

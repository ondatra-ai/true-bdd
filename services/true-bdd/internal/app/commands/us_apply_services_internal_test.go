package commands

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/engine"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/template"
)

const applyServicesArchitecture = `architecture:
  services:
  - name: calc
    path: services/calc
    language: go
  - name: neighbour
    path: services/neighbour
    language: go
`

const applyServicesCanonical = "metadata: {title: canonical}\nscenarios: {}\n"

// applyServicesScenario is one Registry entry claiming the Story halve's criterion.
func applyServicesScenario(key, service string) string {
	return "  " + key + ":\n" +
		"    description: Halving\n" +
		"    service: " + service + "\n" +
		"    path: tests/calc/calc_test.go\n" +
		"    user_stories:\n" +
		"      - story: docs/product/stories/halve-a-number.yaml\n" +
		"        scenario_id: " + key + "\n" +
		"    merged_steps:\n" +
		"      when: [the Owner halves 6]\n"
}

// commitApplyHost lays out a canonical Registry, a walked scratch copy and an
// architecture, then runs us apply's commit on a converged walk of the items.
func commitApplyHost(t *testing.T, scratch string, items ...string) (string, error) {
	t.Helper()

	dir := t.TempDir()
	canonical := filepath.Join(dir, "scenarios.yaml")
	scratchPath := filepath.Join(dir, "tmp", "scenarios.yaml")
	architecturePath := filepath.Join(dir, "architecture.yaml")

	for path, content := range map[string]string{
		canonical: applyServicesCanonical, scratchPath: scratch, architecturePath: applyServicesArchitecture,
	} {
		err := os.MkdirAll(filepath.Dir(path), 0o750)
		if err != nil {
			t.Fatal(err)
		}

		err = os.WriteFile(path, []byte(content), 0o600)
		if err != nil {
			t.Fatal(err)
		}
	}

	walked := make([]*template.ScenarioApplyData, 0, len(items))
	for _, lineage := range items {
		walked = append(walked, &template.ScenarioApplyData{LineageScenarioID: lineage})
	}

	architecture := func() (string, error) { return architecturePath, nil }
	err := commitApplyWalk(scratchPath, canonical, architecture)(
		&engine.Result[*template.ScenarioApplyData]{Items: walked, AllPassed: true, Reason: engine.Converged},
	)

	written, readErr := os.ReadFile(canonical)
	if readErr != nil {
		t.Fatal(readErr)
	}

	return string(written), err
}

func TestCommitApplyWalkRefusesAMergedScenarioWithNoDeclaredService(t *testing.T) {
	scratch := "scenarios:\n" +
		applyServicesScenario("halve-001-even", `""`) +
		applyServicesScenario("halve-002-odd", "abacus")

	canonical, err := commitApplyHost(t, scratch, "halve-001-even", "halve-002-odd")
	if !errors.Is(err, errUndeclaredService) {
		t.Fatalf("commit error = %v, want %v", err, errUndeclaredService)
	}

	wants := []string{`halve-001-even has service ""`, `halve-002-odd has service "abacus"`, "calc, neighbour"}
	for _, want := range wants {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("commit error %q does not name %q", err, want)
		}
	}

	if canonical != applyServicesCanonical {
		t.Errorf("canonical Registry written despite the refusal:\n%s", canonical)
	}
}

func TestCommitApplyWalkWritesAMergedScenarioNamingADeclaredService(t *testing.T) {
	scratch := "scenarios:\n" +
		applyServicesScenario("halve-001-even", "calc") +
		applyServicesScenario("other-001-left-alone", `""`)

	canonical, err := commitApplyHost(t, scratch, "halve-001-even")
	if err != nil {
		t.Fatalf("commit error = %v, want none", err)
	}

	if canonical != scratch {
		t.Errorf("canonical Registry = %q, want the walked copy", canonical)
	}
}

func TestCommitApplyWalkRefusesWhenTheArchitectureCannotBeRead(t *testing.T) {
	dir := t.TempDir()
	scratchPath := filepath.Join(dir, "scenarios.yaml")

	err := os.WriteFile(scratchPath, []byte("scenarios:\n"+applyServicesScenario("halve-001-even", "calc")), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	unresolved := errors.New("document file does not exist")
	architecture := func() (string, error) { return "", unresolved }

	err = commitApplyWalk(scratchPath, filepath.Join(dir, "canonical.yaml"), architecture)(
		&engine.Result[*template.ScenarioApplyData]{
			Items:  []*template.ScenarioApplyData{{LineageScenarioID: "halve-001-even"}},
			Reason: engine.Converged,
		},
	)
	if !errors.Is(err, unresolved) {
		t.Fatalf("commit error = %v, want %v", err, unresolved)
	}

	_, statErr := os.Stat(filepath.Join(dir, "canonical.yaml"))
	if !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("canonical Registry written although no service could be checked: %v", statErr)
	}
}

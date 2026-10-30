package stepskeleton_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/stepskeleton"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/stepcoverage"
)

const negativePattern = `^the Owner halves (-?\d+)$`

func TestConceptSkeletonIsIdentifiedByConceptSuiteAndPattern(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "steps")
	first := stepskeleton.ConceptLocation(dir, "when", "halve", negativePattern)

	if again := stepskeleton.ConceptLocation(dir, "when", "halve", negativePattern); again != first {
		t.Fatalf("identity is not stable: %+v %+v", first, again)
	}

	for _, other := range []stepskeleton.Definition{
		stepskeleton.ConceptLocation(dir, "when", "double", negativePattern),
		stepskeleton.ConceptLocation(dir, "then", "halve", negativePattern),
		stepskeleton.ConceptLocation(filepath.Join(t.TempDir(), "steps"), "when", "halve", negativePattern),
		stepskeleton.ConceptLocation(dir, "when", "halve", `^the Owner halves (\d+)$`),
	} {
		if other.Function == first.Function {
			t.Fatalf("two identities share %s", first.Function)
		}
	}
}

func TestConceptSkeletonRecordsItsConceptAndResumesWithoutDuplicates(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "steps")
	for range 2 {
		_, err := stepskeleton.WriteConcept(dir, "when", "halve", negativePattern)
		if err != nil {
			t.Fatal(err)
		}
	}

	suite, err := stepcoverage.LoadSuite(dir)
	if err != nil {
		t.Fatal(err)
	}

	if got := len(suite.Bind("the Owner halves -7")); got != 1 {
		t.Fatalf("resume created %d bindings", got)
	}

	found, err := stepskeleton.Concepts(dir)
	if err != nil {
		t.Fatal(err)
	}

	want := stepskeleton.ConceptDefinition{Block: "when", Concept: "halve",
		Definition: stepskeleton.ConceptLocation(dir, "when", "halve", negativePattern)}
	if len(found) != 1 || found[0] != want || !found[0].Pending() {
		t.Fatalf("recorded concept skeletons: %+v", found)
	}
}

func TestConceptSkeletonTakesTheCapturesAndFailsUntilWritten(t *testing.T) {
	t.Parallel()

	definition, err := stepskeleton.WriteConcept(filepath.Join(t.TempDir(), "steps"), "when", "halve", negativePattern)
	if err != nil {
		t.Fatal(err)
	}

	source, err := disk.Read(definition.File)
	if err != nil || !strings.Contains(string(source), `step not written yet: %s`) ||
		!strings.Contains(string(source), "args []string") ||
		!strings.Contains(string(source), `//truebdd:concept when "halve"`) {
		t.Fatalf("pending skeleton: %s %v", source, err)
	}
}

func TestConceptSkeletonWithAWrittenBodyIsNoLongerPending(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "steps")

	definition, err := stepskeleton.WriteConcept(dir, "when", "halve", negativePattern)
	if err != nil {
		t.Fatal(err)
	}

	source, err := disk.Read(definition.File)
	if err != nil {
		t.Fatal(err)
	}

	written := strings.Replace(string(source), `return fmt.Errorf("step not written yet: %s",`,
		`return fmt.Errorf("halving %s",`, 1)

	err = disk.Write(definition.File, []byte(written), disk.Shared)
	if err != nil {
		t.Fatal(err)
	}

	found, err := stepskeleton.Concepts(dir)
	if err != nil {
		t.Fatal(err)
	}

	if len(found) != 1 || found[0].Pending() || found[0].Function != definition.Function {
		t.Fatalf("written concept definition: %+v", found)
	}
}

// A pattern the critique sent back is replaced: its skeleton file and its
// registration go, every other definition of the suite stays as it was.
func TestRemovedConceptSkeletonLeavesNoRegistrationAndKeepsOthers(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "steps")

	wrong, err := stepskeleton.WriteConcept(dir, "when", "halve", `^the Owner halves (-?\d)\d*$`)
	if err != nil {
		t.Fatal(err)
	}

	kept, err := stepskeleton.WriteConcept(dir, "then", "result", `^the result is (-?\d+)$`)
	if err != nil {
		t.Fatal(err)
	}

	err = stepskeleton.RemoveConcept(dir, wrong)
	if err != nil {
		t.Fatal(err)
	}

	requireBindings(t, dir, map[string]int{"the Owner halves 64": 0, "the result is 32": 1})

	found, err := stepskeleton.Concepts(dir)
	if err != nil || len(found) != 1 || found[0].Function != kept.Function || kept.Intact(dir) != nil {
		t.Fatalf("concept records after removal: %+v, %v", found, err)
	}

	_, err = disk.Read(wrong.File)
	if err == nil {
		t.Fatal("the removed skeleton's file remains")
	}
}

func requireBindings(t *testing.T, dir string, want map[string]int) {
	t.Helper()

	suite, err := stepcoverage.LoadSuite(dir)
	if err != nil {
		t.Fatal(err)
	}

	for text, count := range want {
		if bound := suite.Bind(text); len(bound) != count {
			t.Errorf("%q binds %v, want %d", text, bound, count)
		}
	}
}

// A hand-written suite may register its definitions from helpers Register
// calls; their records count as the resolver counts their registrations.
func TestConceptRecordsRegisteredOutsideRegisterAreRead(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "steps")
	source := "package steps\n\n" +
		"func Register(suite *Suite) {\n\tregisterGuard(suite)\n}\n\n" +
		"func registerGuard(suite *Suite) {\n\tsuite.Step(`^the guard allows the tool call$`, allows)\n}\n\n" +
		"//truebdd:concept then \"guard answer\"\nfunc allows(*State, []string) error { return nil }\n"

	err := disk.Write(filepath.Join(dir, "steps.go"), []byte(source), disk.Shared)
	if err != nil {
		t.Fatal(err)
	}

	found, err := stepskeleton.Concepts(dir)
	if err != nil {
		t.Fatal(err)
	}

	if len(found) != 1 || found[0].Concept != "guard answer" || found[0].Pattern != `^the guard allows the tool call$` {
		t.Fatalf("concept records registered by a helper: %+v", found)
	}
}

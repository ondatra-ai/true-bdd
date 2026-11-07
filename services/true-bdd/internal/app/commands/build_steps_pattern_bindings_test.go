package commands_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/commands"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/stepskeleton"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/provider"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/input"
)

const (
	patternPass            = "pattern-bindings:pass"
	patternFail            = "pattern-bindings:fail"
	coveragePass           = "step-coverage:pass"
	coverageFail           = "step-coverage:fail"
	patternCaptureCategory = "pattern_captures"
	verificationPass       = "verification:pass"
)

type patternWalkReplies struct{ verificationReplies }

func (reply *patternWalkReplies) ExecutePromptWithSystem(ctx context.Context, role provider.Role,
	system, user string, model provider.ModelRef, mode provider.ExecutionMode, schema string,
	options ...provider.TurnOptions,
) (string, error) {
	if strings.Contains(schema, `"patterns"`) {
		slog.Info("Pattern F proposed")
	}

	return reply.verificationReplies.ExecutePromptWithSystem(ctx, role, system, user, model, mode, schema, options...)
}

func TestPatternChecklistRechecksEachProposalAndRestartsBeforeCoverage(t *testing.T) {
	reply := &patternWalkReplies{verificationReplies: verificationReplies{conceptReplies: conceptReplies{
		proposals: []string{patternAnswer(`^Press Halve once for one\.$`, false),
			patternAnswer(`^Press Halve once for (\w+)\.$`, true)},
	}}}
	deps := conceptHost(t, reply)
	prepareVerificationWalk(t, &deps, &reply.verificationReplies)
	output := capturePatternWalk(t)

	err := commands.RunBuildSteps(t.Context(), deps, "docs/scenarios.yaml", "docs/architecture/architecture.yaml", true)
	if err != nil {
		t.Fatal(err)
	}

	want := []string{patternFail, "F", patternFail, "F", patternPass,
		patternPass, coverageFail, coveragePass, patternPass,
		coveragePass, verificationPass, patternPass, coveragePass, verificationPass}
	if got := patternWalkTrace(t, output); !slices.Equal(got, want) {
		t.Fatalf("trace=%v, want %v", got, want)
	}

	if !strings.Contains(reply.proposed[1], `no pattern binds when step \"Press Halve once for two.\"`) {
		t.Fatal("pattern F lost its Q's findings")
	}
}

func patternAnswer(pattern string, capture bool) string {
	captures := "[]"
	if capture {
		captures = `[{"name":"number","means":"The complete supplied number."}]`
	}

	return fmt.Sprintf(`{"patterns":[{"pattern":%q,"captures":%s,"serves":"Press Halve once."}]}`, pattern, captures)
}

func capturePatternWalk(t *testing.T) *bytes.Buffer {
	t.Helper()

	var output bytes.Buffer

	previous := slog.Default()

	slog.SetDefault(slog.New(slog.NewJSONHandler(&output, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	return &output
}

func patternWalkTrace(t *testing.T, output *bytes.Buffer) []string {
	t.Helper()

	trace := make([]string, 0)

	for _, line := range strings.Split(strings.TrimSpace(output.String()), "\n") {
		var event map[string]any

		err := json.Unmarshal([]byte(line), &event)
		if err != nil {
			t.Fatal(err)
		}

		switch event["msg"] {
		case "Check answered":
			trace = append(trace, fmt.Sprint(event["check"])+":"+fmt.Sprint(event["answer"]))
		case "Step definition verified":
			verdict := verificationPass
			if findings, ok := event["findings"].([]any); ok && len(findings) > 0 {
				verdict = "verification:fail"
			}

			trace = append(trace, verdict)
		case "Pattern F proposed":
			trace = append(trace, "F")
		}
	}

	return trace
}

func TestBareFreshConceptReportsPatternFindingsWithoutAProposal(t *testing.T) {
	reply := &conceptReplies{}
	deps := conceptHost(t, reply)
	output := capturePatternWalk(t)

	err := commands.RunBuildSteps(t.Context(), deps, "docs/scenarios.yaml", "docs/architecture/architecture.yaml", false)
	if !errors.Is(err, commands.ErrMissingStepDefinitions) {
		t.Fatal(err)
	}

	if got := patternWalkTrace(t, output); !slices.Equal(got, []string{patternFail, coverageFail}) {
		t.Fatalf("bare trace=%v", got)
	}

	if len(reply.proposed) != 0 || !strings.Contains(output.String(), "no pattern binds when step") {
		t.Fatalf("bare run lost findings or proposed a fix: %s", output.String())
	}

	_, err = os.Stat("tests/calc/steps")
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("bare run created skeletons: %v", err)
	}
}

func TestBareConceptAnswersCoverageBeforePatternBindingsWhenWrittenFirst(t *testing.T) {
	reply := &conceptReplies{}
	deps := conceptHost(t, reply)

	content, err := disk.Read("true-bdd/checklists/build-steps.yaml")
	if err != nil {
		t.Fatal(err)
	}

	sections := strings.Split(string(content), "  - id: ")
	sections[1], sections[2] = sections[2], sections[1]

	err = disk.Write("true-bdd/checklists/build-steps.yaml", []byte(strings.Join(sections, "  - id: ")), disk.Shared)
	if err != nil {
		t.Fatal(err)
	}

	output := capturePatternWalk(t)

	err = commands.RunBuildSteps(t.Context(), deps, "docs/scenarios.yaml", "docs/architecture/architecture.yaml", false)
	if !errors.Is(err, commands.ErrMissingStepDefinitions) {
		t.Fatal(err)
	}

	if got := patternWalkTrace(t, output); !slices.Equal(got, []string{coverageFail, patternFail}) {
		t.Fatalf("bare trace=%v", got)
	}

	if len(reply.proposed) != 0 {
		t.Fatal("bare run proposed a fix")
	}
}

func TestResumedCapturingPatternReachesBodiesWithoutAProposal(t *testing.T) {
	reply := &verificationReplies{}
	deps := conceptHost(t, reply)
	prepareVerificationWalk(t, &deps, reply)

	_, err := stepskeleton.WriteConcept("tests/calc/steps", "when", "halve", `^Press Halve once for (\w+)\.$`)
	if err != nil {
		t.Fatal(err)
	}

	output := capturePatternWalk(t)

	err = commands.RunBuildSteps(t.Context(), deps, "docs/scenarios.yaml", "docs/architecture/architecture.yaml", true)
	if err != nil {
		t.Fatal(err)
	}

	got := patternWalkTrace(t, output)
	if len(reply.proposed) != 0 || len(got) == 0 || got[0] != patternPass {
		t.Fatalf("resumed trace=%v, proposals=%v", got, reply.proposed)
	}
}

func TestVerificationPatternFixIsJudgedByItsOwnQThenTheRestartedCodeQ(t *testing.T) {
	reply := &patternWalkReplies{verificationReplies: verificationReplies{category: patternCaptureCategory,
		conceptReplies: conceptReplies{proposals: []string{
			patternAnswer(`^Press Halve once for (\w+)\.$`, true),
			patternAnswer(`^Press Halve once for one\.$`, false),
			patternAnswer(`^Press Halve once for (\w+)\.$`, true),
		}},
	}}
	deps := conceptHost(t, reply)
	prepareVerificationWalk(t, &deps, &reply.verificationReplies)
	output := capturePatternWalk(t)

	err := commands.RunBuildSteps(t.Context(), deps, "docs/scenarios.yaml", "docs/architecture/architecture.yaml", true)
	if err != nil {
		t.Fatal(err)
	}

	want := []string{patternFail, "F", patternPass, patternPass, coverageFail, coveragePass,
		patternPass, coveragePass, "verification:fail", "F", verificationPass,
		patternFail, "F", patternPass, patternPass, coverageFail, coveragePass,
		patternPass, coveragePass, verificationPass, patternPass, coveragePass, verificationPass}
	if got := patternWalkTrace(t, output); !slices.Equal(got, want) {
		t.Fatalf("trace=%v, want %v", got, want)
	}

	if len(reply.proposed) != 3 || !strings.Contains(reply.proposed[1], "Repair this discrepancy only.") ||
		!strings.Contains(reply.proposed[2], `no pattern binds when step \"Press Halve once for two.\"`) {
		t.Fatalf("Q-to-F findings were lost: %v", reply.proposed)
	}
}

func TestPatternChecklistHoldsTheOriginalProposalPromptInItsF(t *testing.T) {
	deps := conceptHost(t, &conceptReplies{})

	document, err := deps.ChecklistLoader.LoadFull("build-steps")
	if err != nil {
		t.Fatal(err)
	}

	prompt := document.Sections[0].ValidationPrompts[0]

	const originalSHA = "43ee55b5a624949c376d12ff783f39ecbfb25ab443b27be8ec1fe26eead38436"
	if fmt.Sprintf("%x", sha256.Sum256([]byte(prompt.FixTemplate))) != originalSHA {
		t.Fatal("the original proposal prompt changed while moving from Q.prompt to F")
	}

	if prompt.Check != "pattern-bindings" || document.Sections[1].ID != "step-coverage" {
		t.Fatal("pattern bindings must precede step coverage")
	}

	for _, section := range document.Sections {
		if section.ID == "concept-patterns" {
			t.Fatal("proposal still has a separate question")
		}
	}
}

func TestPatternFixRefinementKeepsTheProposalAndThePersonsFeedback(t *testing.T) {
	reply := &patternWalkReplies{verificationReplies: verificationReplies{conceptReplies: conceptReplies{
		proposals: []string{patternAnswer(`^Press Halve once for (\w+)\.$`, true)},
	}}}
	deps := conceptHost(t, reply)
	prepareVerificationWalk(t, &deps, &reply.verificationReplies)

	const feedback = "Keep the complete word capture and name it word."

	deps.UserInputCollector = input.NewUserInputCollectorFrom(strings.NewReader("2\n" + feedback + "\n\n1\n1\n"))

	err := commands.RunBuildSteps(t.Context(), deps, "docs/scenarios.yaml", "docs/architecture/architecture.yaml", true)
	if err != nil {
		t.Fatal(err)
	}

	if len(reply.proposed) != 2 || !strings.Contains(reply.proposed[1], feedback) ||
		!strings.Contains(reply.proposed[1], `^Press Halve once for (\\w+)\\.$`) {
		t.Fatalf("refinement lost its candidate or feedback: %v", reply.proposed)
	}
}

func TestExitingThePatternFixWritesNoSkeleton(t *testing.T) {
	reply := &conceptReplies{proposals: []string{patternAnswer(`^Press Halve once for (\w+)\.$`, true)}}
	deps := conceptHost(t, reply)
	deps.UserInputCollector = input.NewUserInputCollectorFrom(strings.NewReader("3\n"))

	err := commands.RunBuildSteps(t.Context(), deps, "docs/scenarios.yaml", "docs/architecture/architecture.yaml", true)
	if !errors.Is(err, commands.ErrBuildStepsNotConverged) {
		t.Fatal(err)
	}

	_, err = os.Stat("tests/calc/steps")
	if !errors.Is(err, fs.ErrNotExist) || len(reply.proposed) != 1 {
		t.Fatalf("exited proposal wrote skeletons: %v; proposals=%d", err, len(reply.proposed))
	}
}

func TestCoveredHostNeedsNoDocumentsForAConceptItemItDoesNotHave(t *testing.T) {
	repository, err := filepath.Abs("../../../../..")
	if err != nil {
		t.Fatal(err)
	}

	reply := &conceptReplies{}
	deps := conceptHost(t, reply)

	err = disk.CopyTree(".", filepath.Join(repository, "tests/true-bdd/fixtures/build-tests-coverage-in-process/input"),
		disk.Shared)
	if err != nil {
		t.Fatal(err)
	}

	err = disk.Remove("docs/product/product.yaml")
	if err != nil {
		t.Fatal(err)
	}

	err = commands.RunBuildSteps(t.Context(), deps, "docs/scenarios.yaml", "docs/architecture/architecture.yaml", false)
	if err != nil || len(reply.requests) != 0 {
		t.Fatalf("covered host: %v; turns=%v", err, reply.requests)
	}
}

func TestResumedItemChecksItsUnusedPendingRegistration(t *testing.T) {
	reply := &conceptReplies{}
	deps := conceptHost(t, reply)

	for _, pattern := range []string{`^Press Halve once for (\w+)\.$`, `^a step absent from the suite$`} {
		_, err := stepskeleton.WriteConcept("tests/calc/steps", "when", "halve", pattern)
		if err != nil {
			t.Fatal(err)
		}
	}

	output := capturePatternWalk(t)

	err := commands.RunBuildSteps(t.Context(), deps, "docs/scenarios.yaml", "docs/architecture/architecture.yaml", false)
	if !errors.Is(err, commands.ErrMissingStepDefinitions) {
		t.Fatal(err)
	}

	got := patternWalkTrace(t, output)
	if len(got) == 0 || got[0] != patternFail || !strings.Contains(output.String(), "binds no step of the suite") {
		t.Fatalf("resumed Item lost its unused registration: %s", output.String())
	}
}

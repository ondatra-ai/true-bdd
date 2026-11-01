package catalogue_test

import (
	"slices"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/catalogue"
)

const (
	halveItem  = "when:halve"
	resultItem = "then:result"
)

func suiteSteps() []catalogue.SuiteStep {
	return []catalogue.SuiteStep{
		{Block: catalogue.BlockWhen, Text: "the Owner halves -7", Item: halveItem},
		{Block: catalogue.BlockWhen, Text: "the Owner halves 6", Item: halveItem},
		{Block: catalogue.BlockWhen, Text: "the Owner doubles 6", Item: "when:double"},
		{Block: catalogue.BlockThen, Text: "the result is -3", Item: resultItem},
		{Block: catalogue.BlockThen, Text: "the result is 6", Bound: `^the result is (\d+)$`},
	}
}

func proposal(patterns ...string) []catalogue.ProposedPattern {
	proposed := make([]catalogue.ProposedPattern, 0, len(patterns))
	for _, pattern := range patterns {
		proposed = append(proposed, catalogue.ProposedPattern{Pattern: pattern,
			Captures: []catalogue.PatternCapture{{Name: "value", Means: "the operand"}}})
	}

	return proposed
}

func TestPatternCheckAcceptsEveryStepBindingExactlyOneOwnPattern(t *testing.T) {
	t.Parallel()

	findings := catalogue.CheckPatterns(halveItem, proposal(`^the Owner halves (-?\d+)$`), suiteSteps())
	if len(findings) != 0 {
		t.Fatalf("clean proposal refused: %q", findings)
	}
}

func TestPatternCheckRefusesUnboundAndDoublyBoundSteps(t *testing.T) {
	t.Parallel()

	uncovered := catalogue.CheckPatterns(halveItem, proposal(`^the Owner halves (\d+)$`), suiteSteps())
	want := []string{`no pattern binds when step "the Owner halves -7"`}

	if !slices.Equal(uncovered, want) {
		t.Fatalf("uncovered step: %q", uncovered)
	}

	twice := catalogue.CheckPatterns(halveItem, proposal(`^the Owner halves (-?\d+)$`, `^the Owner halves (-\d+)$`),
		suiteSteps())
	want = []string{`when step "the Owner halves -7" binds 2 patterns: ` +
		`^the Owner halves (-?\d+)$ | ^the Owner halves (-\d+)$`}

	if !slices.Equal(twice, want) {
		t.Fatalf("doubly bound step: %q", twice)
	}
}

func TestPatternCheckRefusesACollisionWithAnExistingDefinition(t *testing.T) {
	t.Parallel()

	findings := catalogue.CheckPatterns(resultItem, proposal(`^the result is (-?\d+)$`), suiteSteps())
	want := []string{`pattern ^the result is (-?\d+)$ also binds then step "the result is 6", ` +
		`which the definition ^the result is (\d+)$ binds`}

	if !slices.Equal(findings, want) {
		t.Fatalf("collision with a definition: %q", findings)
	}
}

func TestPatternCheckRefusesAStepOfAnotherItemInEveryBlock(t *testing.T) {
	t.Parallel()

	sameBlock := catalogue.CheckPatterns(halveItem, proposal(`^the Owner (?:halves|doubles) (-?\d+)$`), suiteSteps())
	want := []string{`pattern ^the Owner (?:halves|doubles) (-?\d+)$ also binds when step "the Owner doubles 6", ` +
		`which is grouped into when:double`}

	if !slices.Equal(sameBlock, want) {
		t.Fatalf("cross-Item collision: %q", sameBlock)
	}

	steps := append(suiteSteps(), catalogue.SuiteStep{Block: catalogue.BlockThen, Text: "the Owner halves 6",
		Item: resultItem})
	otherBlock := catalogue.CheckPatterns(halveItem, proposal(`^the Owner halves (-?\d+)$`), steps)
	want = []string{`pattern ^the Owner halves (-?\d+)$ also binds then step "the Owner halves 6", ` +
		`which is grouped into then:result`}

	if !slices.Equal(otherBlock, want) {
		t.Fatalf("cross-block collision: %q", otherBlock)
	}
}

func TestPatternCheckRefusesAPatternThatBindsNoStepOfTheSuite(t *testing.T) {
	t.Parallel()

	findings := catalogue.CheckPatterns(halveItem,
		proposal(`^the Owner halves (-?\d+)$`, `^the Owner halves nothing (\d+)$`), suiteSteps())
	want := []string{`pattern ^the Owner halves nothing (\d+)$ binds no step of the suite`}

	if !slices.Equal(findings, want) {
		t.Fatalf("unused pattern: %q", findings)
	}
}

func TestPatternCheckRefusesMalformedPatterns(t *testing.T) {
	t.Parallel()

	broken := catalogue.CheckPatterns(halveItem, proposal(`^the Owner halves (-?\d+$`), suiteSteps())
	if len(broken) != 1 || broken[0][:len(`pattern ^the Owner halves (-?\d+$ does not compile`)] !=
		`pattern ^the Owner halves (-?\d+$ does not compile` {
		t.Fatalf("compile failure: %q", broken)
	}

	unnamed := []catalogue.ProposedPattern{{Pattern: `^the Owner (halves) (-?\d+)$`,
		Captures: []catalogue.PatternCapture{{Name: "value", Means: "the operand"}}}}
	counted := catalogue.CheckPatterns(halveItem, unnamed, suiteSteps())
	want := []string{`pattern ^the Owner (halves) (-?\d+)$ has 2 capture groups but names 1 capture(s)`}

	if !slices.Equal(counted, want) {
		t.Fatalf("capture count: %q", counted)
	}
}

func TestPatternCapturesListTheStepArgumentsInOrder(t *testing.T) {
	t.Parallel()

	captures := catalogue.PatternCaptures(`^the Owner (halves|doubles) (-?\d+)$`, "the Owner halves -7")
	if !slices.Equal(captures, []string{"halves", "-7"}) {
		t.Fatalf("captures: %q", captures)
	}
}

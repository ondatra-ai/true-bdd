package catalogue_test

import (
	"reflect"
	"regexp"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/catalogue"
)

func TestCapturingPatternPreservesSlotOrderAndGrammar(t *testing.T) {
	t.Parallel()

	const template = `the {actor:role} sends "{message}" [with|using] {count:number} item[s] matching {tail:pattern}`

	compiled, malformed := catalogue.Compile(
 []catalogue.Entry{{Template: template, Block: "when"}}, []string{"User", "Architect"})
	if len(malformed) != 0 {
		t.Fatal(malformed)
	}

	pattern := regexp.MustCompile(compiled.CapturingPattern(template))
	want := []string{"User", "hello world", "no", `a "quoted" tail`}

	match := pattern.FindStringSubmatch(`the User sends "hello world" using no items matching a "quoted" tail`)
	if len(match) == 0 || !reflect.DeepEqual(match[1:], want) {
		t.Fatalf("captures: %q, want %q", match, want)
	}

	if regexp.MustCompile(compiled.Pattern(template)).NumSubexp() != 0 {
		t.Fatal("ordinary pattern acquired captures")
	}

	if pattern.MatchString(`the User sends "hello world" using -1 items matching tail`) {
		t.Fatal("capturing variant changed the grammar")
	}
}

package commands

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/story"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/docs"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/fs"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/template"
)

const refineDoubling = `concepts:
  when:
    - concept: doubling
      means: The Owner doubles a whole number with the calculator.
      terms: [number]
`

const refineGlossary = `glossary:
  - term: number
    means: An integer operand.
`

const refineProduct = refineDoubling + refineGlossary

const refineProductFixed = refineDoubling + `    - concept: halving
      means: The Owner halves a whole number with the calculator.
      terms: [number]
` + refineGlossary

// The walked Story's id and Feature, which a rewrite's body cannot change.
const (
	refineStoryID = "halve"
	refineFeature = "arithmetic"
)

// refineLiveConfirmation is the block #245's second live run returned: the
// unquoted ": " in `(terms: [number])` makes it invalid YAML.
const refineLiveConfirmation = "applied: true\n" +
	"target: product\n" +
	"summary: Added new `when` concept `halving` (terms: [number]) after `doubling`, " +
	"and widened the `then` concept `result`'s means to cover both doubling and halving."

// refineHost is a Story's first version and a product document whose
// working copy holds an applied concept fix.
type refineHost struct {
	versions *fs.StoryVersionManager
	item     *template.StoryRefineData
	product  string
}

func newRefineHost(t *testing.T) refineHost {
	t.Helper()

	dir := t.TempDir()
	productPath := filepath.Join(dir, "product.yaml")
	scratchPath := filepath.Join(dir, "tmp", "product.yaml")

	for path, content := range map[string]string{productPath: refineProduct, scratchPath: refineProductFixed} {
		err := os.MkdirAll(filepath.Dir(path), 0o750)
		if err != nil {
			t.Fatal(err)
		}

		err = os.WriteFile(path, []byte(content), 0o600)
		if err != nil {
			t.Fatal(err)
		}
	}

	runDir, err := fs.NewRunDirectory(filepath.Join(dir, "tmp"))
	if err != nil {
		t.Fatal(err)
	}

	loaded := &story.Story{
		ID: refineStoryID, Feature: refineFeature, Title: "Halve a number", AsA: "Owner", Status: "DRAFT",
		AcceptanceCriteria: []story.AcceptanceCriterion{{ID: "001-even", Description: "It must print 3."}},
	}

	versions := fs.NewStoryVersionManager(runDir, loaded.ID)

	err = versions.SaveInitialVersion(loaded)
	if err != nil {
		t.Fatal(err)
	}

	documents := []template.FixDocument{{Key: docs.KeyProduct, Path: productPath, Scratch: scratchPath}}

	return refineHost{versions: versions, item: refineItem(loaded, documents), product: productPath}
}

func (h refineHost) productContent(t *testing.T) string {
	t.Helper()

	content, err := os.ReadFile(h.product)
	if err != nil {
		t.Fatal(err)
	}

	return string(content)
}

// A confirmation an unquoted ": " keeps from decoding is still the concept
// fix's confirmation: us refine commits the product working copy and keeps
// the Story, never reading the block as a Story body.
func TestRefinePostFixCommitsAConceptFixWhoseConfirmationIsNotValidYAML(t *testing.T) {
	host := newRefineHost(t)

	next, err := refinePostFix(host.versions)(context.Background(), host.item, refineLiveConfirmation)
	if err != nil {
		t.Fatalf("refinePostFix() error = %v, want the concept fix committed", err)
	}

	if next != host.item {
		t.Errorf("refinePostFix() returned another Item, want the walked Story kept")
	}

	if got := host.productContent(t); got != refineProductFixed {
		t.Errorf("product document = %q, want its working copy committed", got)
	}

	if version := host.versions.GetCurrentVersion(); version != 1 {
		t.Errorf("Story version = %d, want 1: a confirmation is no Story body", version)
	}
}

// A step rewrite's Story body stays a Story body: it becomes the Story's
// next version and no document is committed.
func TestRefinePostFixReadsAStoryBodyAsTheStorysNextVersion(t *testing.T) {
	host := newRefineHost(t)
	body := "title: Halve a number\nas_a: Owner\nstatus: DRAFT\nacceptance_criteria:\n" +
		"  - id: 001-even\n    description: \"It must print 3: half of 6.\"\n"

	next, err := refinePostFix(host.versions)(context.Background(), host.item, body)
	if err != nil {
		t.Fatalf("refinePostFix() error = %v, want the Story body read", err)
	}

	if got := next.AcceptanceCriteria[0].Description; got != "It must print 3: half of 6." {
		t.Errorf("rewritten criterion = %q, want the Story body's", got)
	}

	if next.ID != refineStoryID || next.Feature != refineFeature {
		t.Errorf("rewritten Story id/feature = %q/%q, want the walked Story's", next.ID, next.Feature)
	}

	if version := host.versions.GetCurrentVersion(); version != 2 {
		t.Errorf("Story version = %d, want 2", version)
	}

	if got := host.productContent(t); got != refineProduct {
		t.Errorf("product document = %q, want it untouched by a step rewrite", got)
	}
}

// An answer that is neither a readable confirmation nor a parseable Story
// body stops with the error naming what came back, committing nothing.
func TestRefinePostFixRefusesAnAnswerThatIsNeither(t *testing.T) {
	for name, content := range map[string]string{
		"block without applied": "target: product\nsummary: Added `halving` (terms: [number]).",
		"prose":                 "I added the concept halving: it halves: a number.",
	} {
		t.Run(name, func(t *testing.T) {
			host := newRefineHost(t)

			_, err := refinePostFix(host.versions)(context.Background(), host.item, content)
			if err == nil || !strings.Contains(err.Error(), "failed to parse updated story body") {
				t.Fatalf("refinePostFix() error = %v, want the Story body parse failure", err)
			}

			if got := host.productContent(t); got != refineProduct {
				t.Errorf("product document = %q, want nothing committed", got)
			}
		})
	}
}

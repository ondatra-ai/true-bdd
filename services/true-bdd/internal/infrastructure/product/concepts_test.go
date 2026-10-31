package product_test

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/product"
)

func TestProductConcepts(t *testing.T) {
	const retired = "declare concepts: in its place"

	t.Parallel()

	cases := []struct{ name, body, finding string }{
		{"concept form", "concepts:\n  given: [{concept: number, means: A number is stored., " +
			"terms: [number]}]\n  when: [{concept: number, means: Call the number " +
			"command., terms: []}]\n", ""},
		{"retired catalogue", "catalogue: []\n", retired},
		{"retired catalogue beside concepts", "catalogue: []\nconcepts: {}\n", retired},
		{"null catalogue still declared", "catalogue: null\n", retired},
		{"duplicate in block", "concepts:\n  given: [{concept: number, means: Stored., terms: []}, " +
			"{concept: number, means: Present., terms: []}]\n", "given concept 'number'"},
		{"duplicate block", "concepts:\n  given: []\n  given: []\n", "block given"},
		{"unknown block", "concepts: {whenever: []}\n", "whenever"},
		{"missing terms", "concepts: {given: [{concept: number, means: Stored.}]}\n", "number"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			path := filepath.Join(t.TempDir(), "product.yaml")

			err := disk.Write(path, []byte(testCase.body), disk.Shared)
			if err != nil {
				t.Fatal(err)
			}

			loaded, err := product.Load(path)
			if testCase.finding != "" {
				if err == nil || !strings.Contains(err.Error(), testCase.finding) {
					t.Fatalf("expected %q, got %v", testCase.finding, err)
				}

				return
			}

			if err != nil {
				t.Fatal(err)
			}

			assertConceptsLoaded(t, loaded)
		})
	}
}

func assertConceptsLoaded(t *testing.T, loaded product.Document) {
	t.Helper()

	if !loaded.ConceptsDeclared || len(loaded.Concepts) != 2 {
		t.Fatalf("lost concepts: %+v", loaded)
	}

	if loaded.Concepts[0].Name != "number" || loaded.Concepts[1].Block != "when" {
		t.Fatalf("lost identity: %+v", loaded)
	}
}

func TestValidateFormRefusesTheRetiredCatalogue(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "product.yaml")

	err := disk.Write(path, []byte("catalogue:\n  - template: the Owner halves {number}\n"), disk.Shared)
	if err != nil {
		t.Fatal(err)
	}

	err = product.ValidateForm(path)
	if !errors.Is(err, product.ErrCatalogueRetired) || !strings.Contains(err.Error(), "concepts:") {
		t.Fatalf("startup must refuse the catalogue naming concepts, got %v", err)
	}
}

func TestValidateFormLeavesLegacyReadErrorsToTheCommand(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "product.yaml")

	err := disk.Write(path, []byte("concepts: [\n"), disk.Shared)
	if err != nil {
		t.Fatal(err)
	}

	err = product.ValidateForm(path)
	if err != nil {
		t.Fatalf("legacy startup changed: %v", err)
	}

	_, err = product.Load(path)
	if err == nil {
		t.Fatal("command must still report the unreadable product")
	}
}

package product_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/product"
)

const fourSentences = "4 sentences"

func TestGlossaryRefusesFourSentencesAtEveryReader(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "product.yaml")

	err := disk.Write(path, []byte("glossary:\n- term: greeting\n  means: One. Two! Three? Four.\n"), disk.Shared)
	if err != nil {
		t.Fatal(err)
	}

	readers := map[string]func() error{
		"startup": func() error { return product.ValidateForm(path) },
		"concepts": func() error {
			_, err := product.Load(path)

			return err
		},
		"glossary": func() error {
			_, err := product.LoadGlossary(path)

			return err
		},
	}
	for name, read := range readers {
		err := read()
		if err == nil {
			t.Errorf("%s accepted four sentences", name)

			continue
		}

		for _, want := range []string{"greeting", fourSentences, "1 words", "at most 3 sentences", "25 words"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("%s: missing %q in %v", name, want, err)
			}
		}
	}
}

func TestGlossarySentenceWordAndParagraphBoundaries(t *testing.T) {
	t.Parallel()

	const twentyFiveWords = "One two three four five six seven eight nine ten eleven twelve thirteen fourteen " +
		"fifteen sixteen seventeen eighteen nineteen twenty twenty-one twenty-two twenty-three twenty-four twenty-five"

	cases := []struct{ name, means, finding string }{
		{"25 words", twentyFiveWords + ".", ""},
		{"26 words", twentyFiveWords + " twenty-six.", "longest sentence 26 words"},
		{"three sentences", "One. Two! Three?", ""},
		{"unterminated fourth", "One. Two! Three? Four", fourSentences},
		{"punctuation run", "One... Two!? Three!!!", ""},
		{"paths and decimals", "Read docs/product.yaml at 1.25. Then compare. Done.", ""},
		{"unicode whitespace", "One.\u00a0Two!\u2003Three?\tFour", fourSentences},
		{"wrapped paragraph", "One line\ncontinues here.", ""},
		{"two paragraphs", "One.\n \t\nTwo.", "2 paragraphs"},
		{"empty meaning", "", "0 paragraphs"},
		{"CRLF paragraphs", "One.\r\n\r\nTwo.", "2 paragraphs"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), "product.yaml")

			body := "glossary:\n- term: boundary\n  means: |\n    " + strings.ReplaceAll(testCase.means, "\n", "\n    ") + "\n"

			err := disk.Write(path, []byte(body), disk.Shared)
			if err != nil {
				t.Fatal(err)
			}

			err = product.ValidateForm(path)
			if testCase.finding == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.Contains(err.Error(), testCase.finding) {
				t.Fatalf("want %q, got %v", testCase.finding, err)
			}
		})
	}
}

func TestLongGlossaryMeaningCannotHideBehindMalformedSibling(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "product.yaml")
	body := "glossary:\n- term: greeting\n  means: One. Two. Three. Four.\n- term: broken\n  means: []\n"

	err := disk.Write(path, []byte(body), disk.Shared)
	if err != nil {
		t.Fatal(err)
	}

	err = product.ValidateForm(path)
	if err == nil || !strings.Contains(err.Error(), fourSentences) {
		t.Fatalf("long meaning hidden by sibling: %v", err)
	}
}

package commands

import (
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
)

const conceptRegistry = "docs/scenarios.yaml"

// A concept host's malformed term list stops build steps --fix before its
// grouping and binding turns.
func TestConceptBuildStepsReadsTermListsBeforeAnyTurn(t *testing.T) {
	for name, example := range map[string]struct{ file, from, to, want string }{
		"glossary": {"docs/product/product.yaml", "glossary:\n  - term: number",
			"glossary:\n  not: a list\nunused:\n  - term: number", "glossary: must hold a list of terms"},
		"word mapping": {repairArchitecture, "  terms:\n  - term: number",
			"  terms:\n    not: a list\n  unused:\n  - term: number", "terms: must hold a list"},
	} {
		t.Run(name, func(t *testing.T) {
			reply := &proposalReplies{}
			execution := repairExecution(t, reply)
			execution.registryFile = conceptRegistry

			body, err := disk.Read(example.file)
			if err != nil {
				t.Fatal(err)
			}

			if !strings.Contains(string(body), example.from) {
				t.Fatalf("%s holds no %q", example.file, example.from)
			}

			err = disk.Write(example.file, []byte(strings.Replace(string(body), example.from, example.to, 1)),
				disk.Shared)
			if err != nil {
				t.Fatal(err)
			}

			_, err = execution.prepareConcepts(t.Context())
			if err == nil || !strings.Contains(err.Error(), example.want) {
				t.Fatalf("want %q, got %v", example.want, err)
			}

			if len(reply.requests) != 0 {
				t.Fatalf("%d turns before the refusal", len(reply.requests))
			}
		})
	}
}

package stepskeleton_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/stepskeleton"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
)

func TestShapeProtectsRegistrationsSignaturesAndOtherBodies(t *testing.T) {
	t.Parallel()

	const original = `package steps
 type State struct{}
 type Other struct{}
 func selected(state *State, args []string) error { return nil }
 func kept(state *State, args []string) error { return nil }
 func (o *Other) kept() { println("kept") }
 func Register(suite Registrar) { suite.Step("selected", selected); suite.Step("kept", kept) }
 `

	for _, change := range []struct{ name, old, replacement string }{
		{"registration", `"kept", kept`, `"changed", kept`},
		{"selected signature", "selected(state *State", "selected(state *Other"},
		{"other body", "func kept(state *State, args []string) error { return nil }",
			"func kept(state *State, args []string) error { panic(1) }"},
		{"other method", `println("kept")`, `println("changed")`},
	} {
		t.Run(change.name, func(t *testing.T) {
			t.Parallel()
			dir := filepath.Join(t.TempDir(), "steps")

			file := filepath.Join(dir, "steps.go")

			err := disk.Write(file, []byte(original), disk.Shared)
			if err != nil {
				t.Fatal(err)
			}

			before, err := stepskeleton.Shape(dir, "selected")
			if err != nil {
				t.Fatal(err)
			}

			modified := strings.Replace(original, change.old, change.replacement, 1)

			err = disk.Write(file, []byte(modified), disk.Shared)
			if err != nil {
				t.Fatal(err)
			}

			after, err := stepskeleton.Shape(dir, "selected")
			if err != nil {
				t.Fatal(err)
			}

			if before == after {
				t.Fatalf("shape permits changing %s", change.name)
			}
		})
	}
}

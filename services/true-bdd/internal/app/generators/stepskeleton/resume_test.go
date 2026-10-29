package stepskeleton_test

import (
	"path/filepath"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/stepskeleton"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/stepcoverage"
)

func TestPendingSkeletonCanResumeWithoutDuplicateRegistration(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "steps")
	for range 2 {
		_, err := stepskeleton.Write(dir, "the command runs", `\Athe command runs\z`)
		if err != nil {
			t.Fatal(err)
		}
	}

	suite, err := stepcoverage.LoadSuite(dir)
	if err != nil {
		t.Fatal(err)
	}

	if got := len(suite.Bind("the command runs")); got != 1 {
		t.Fatalf("resume created %d bindings", got)
	}
}

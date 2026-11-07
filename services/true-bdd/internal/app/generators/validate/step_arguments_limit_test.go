package validate_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/adapters/ai"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/validate"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/config"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
)

func TestStepArgumentEvidenceRefusesOversizedTurnsBeforeProviderStart(t *testing.T) {
	conceptRequestEvaluator(t)
	stub := t.TempDir()

	executable := filepath.Join(stub, "claude")

	err := disk.Write(executable, []byte("#!/bin/sh\nexit 97\n"), disk.Private)
	if err != nil {
		t.Fatal(err)
	}

	err = os.Chmod(executable, 0o700)
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("PATH", stub+string(os.PathListSeparator)+os.Getenv("PATH"))

	body, err := disk.Read(argumentArchitecturePath)
	if err != nil {
		t.Fatal(err)
	}

	const limit = 150000

	section := "step_arguments:\n- name: literal\n  read: " + strings.Repeat("é", limit) +
		"\n  applies: every step\n  escaping: none\n"

	err = disk.Write(argumentArchitecturePath, append(body, []byte(section)...), disk.Shared)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := config.NewViperConfig()
	if err != nil {
		t.Fatal(err)
	}

	router := ai.NewRouter(".", t.TempDir(), limit)
	evaluator := validate.NewChecklistEvaluatorWithPaths(router, cfg, argumentModels(t), "unused.tpl", "unused.tpl")
	subject, prompt := argumentProtocol(t, evaluator, linkJudgement)
	_, err = evaluator.EvaluateOne(t.Context(), subject, "argument-item", prompt, t.TempDir())

	var refused *ai.PromptLimitError
	if !errors.As(err, &refused) || refused.Size < len(section) || refused.Limit != limit {
		t.Fatalf("complete oversized evidence must refuse distinctly before spawning: %v", err)
	}
}

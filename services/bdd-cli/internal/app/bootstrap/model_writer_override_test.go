package bootstrap_test

import (
	"errors"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/cmd"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/app/bootstrap"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/app/commands"
	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/app/generators/validate"
	pkgerrors "github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/pkg/errors"
)

func TestBuildCodeModelOverrideReachesClaudeWriter(t *testing.T) {
	container, directory := prepareWriterOverride(t, "coder=claude:claude-sonnet-5-5")

	_, err := container.BuildCodeFixApplier.Apply(t.Context(), validate.ApplyParams{
		Subject: &commands.BuildCodeItem{}, SubjectID: "failing-set",
		TmpDir: container.RunDir.GetTmpOutPath(), FixPrompt: "repair",
		ModelTier: "coder", ClaudeWriter: true,
	})
	if !errors.Is(err, pkgerrors.ErrFixApplierNoContent) {
		t.Fatalf("writer returned %v", err)
	}

	assertOverriddenClaudeWriter(t, directory, container.RunDir.GetTmpOutPath())
}

func assertOverriddenClaudeWriter(t *testing.T, directory, runDirectory string) {
	t.Helper()

	arguments, err := disk.Read(filepath.Join(directory, "arguments"))
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{"--model\nclaude-sonnet-5-5\n", "--restricted\n", "--settings\n"} {
		if !strings.Contains(string(arguments), want) {
			t.Fatalf("missing %q in writer arguments: %s", want, arguments)
		}
	}

	if strings.Contains(string(arguments), "--safe-mode") {
		t.Fatal("writer hooks disabled by safe mode")
	}

	log, err := disk.Read(filepath.Join(directory, "tmp", "true-bdd.log.json"))
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(log), `"role":"apply","cli":"claude","model":"claude-sonnet-5-5"`) {
		t.Fatalf("override did not dispatch the Claude apply turn: %s", log)
	}

	environment, err := disk.Read(filepath.Join(directory, "environment"))
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(string(environment), "coder=claude:") {
		t.Fatal("override leaked into environment")
	}

	_, err = disk.Read(filepath.Join(runDirectory, "claude-turn001-claude-config", "policy.json"))
	if err != nil {
		t.Fatalf("writer guard configuration missing: %v", err)
	}
}

func prepareWriterOverride(t *testing.T, override string) (*bootstrap.Container, string) {
	t.Helper()

	repository, err := filepath.Abs(repoRoot)
	if err != nil {
		t.Fatal(err)
	}

	directory := modelEnvironmentProbe(t)
	for _, name := range []string{"true-bdd", "templates"} {
		err = disk.CopyTree(filepath.Join(directory, name), filepath.Join(repository, name), disk.Shared)
		if err != nil {
			t.Fatal(err)
		}
	}

	t.Chdir(directory)
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(directory, "state"))

	previous := slog.Default()

	t.Cleanup(func() { slog.SetDefault(previous) })

	build := cmd.NewBuildCommand(nil)

	code, _, err := build.Find([]string{"code"})
	if err != nil {
		t.Fatal(err)
	}

	err = code.ParseFlags([]string{"--fix", "--model", override})
	if err != nil {
		t.Fatal(err)
	}

	overrides, err := code.Flags().GetStringArray("model")
	if err != nil {
		t.Fatal(err)
	}

	container, err := bootstrap.NewContainer(overrides...)
	if err != nil {
		t.Fatal(err)
	}

	return container, directory
}

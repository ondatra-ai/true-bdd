package bootstrap_test

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/adapters/ai"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/bootstrap"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/provider"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/config"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
)

func TestModelOverridesResolveOnlyTheirDeclaredTier(t *testing.T) {
	t.Chdir(repoRoot)

	cfg, err := config.NewViperConfig()
	if err != nil {
		t.Fatal(err)
	}

	var records bytes.Buffer

	previous := slog.Default()

	slog.SetDefault(slog.New(slog.NewJSONHandler(&records, nil)))
	defer slog.SetDefault(previous)

	models, err := bootstrap.NewModelRegistry(cfg, "coder=claude:other:model", "quick=codex:another")
	if err != nil {
		t.Fatal(err)
	}

	if strings.Count(records.String(), `"msg":"Model tier overridden"`) != 2 {
		t.Fatalf("expected one record per applied override: %s", records.String())
	}

	for _, want := range []string{
		`"level":"INFO","msg":"Model tier overridden","tier":"coder",` +
			`"from":"codex:gpt-6.1-sol","to":"claude:other:model"`,
		`"level":"INFO","msg":"Model tier overridden","tier":"quick","from":"claude:claude-sonnet-5","to":"codex:another"`,
	} {
		if !strings.Contains(records.String(), want) {
			t.Fatalf("missing override record %s: %s", want, records.String())
		}
	}

	for tier, want := range map[provider.Tier]string{
		provider.TierCoder: "claude:other:model", provider.TierQuick: "codex:another",
		provider.TierHigh: "claude:claude-opus-5", provider.TierXHigh: "claude:claude-fable-5",
	} {
		ref, err := models.Resolve(tier)
		if err != nil || ref.String() != want {
			t.Fatalf("%s = %v, %v; want %s", tier, ref, err, want)
		}
	}

	if got := cfg.GetStringMapString("engine.models")["coder"]; got != "codex:gpt-6.1-sol" {
		t.Fatalf("config mutated: %s", got)
	}
}

func TestModelOverridesRefuseFaultsAgainstLoadedTiers(t *testing.T) {
	directory := t.TempDir()

	err := disk.Write(filepath.Join(directory, "true-bdd/true-bdd.yaml"), []byte(`engine:
  models:
    high: claude:configured
  default_prompt_model: high
  default_fix_model: high
  default_apply_model: high
`), disk.Private)
	if err != nil {
		t.Fatal(err)
	}

	t.Chdir(directory)

	cfg, err := config.NewViperConfig()
	if err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		values []string
		fault  string
	}{
		{[]string{"coder=claude:new"}, "not configured"},
		{[]string{"other=claude:new"}, "unknown model tier"},
		{[]string{"high=claude"}, "<cli>:<model>"},
		{[]string{"high=claude:"}, "empty model"},
		{[]string{"high=:new"}, "empty cli"},
		{[]string{"high=unknown:new"}, "unknown cli"},
		{[]string{"high"}, "<tier>=<cli>:<model>"},
		{[]string{"high=claude:one", " high =codex:two"}, "twice"},
	} {
		t.Run(strings.Join(test.values, ","), func(t *testing.T) {
			var records bytes.Buffer

			previous := slog.Default()

			slog.SetDefault(slog.New(slog.NewJSONHandler(&records, nil)))
			defer slog.SetDefault(previous)

			_, err := bootstrap.NewModelRegistry(cfg, test.values...)
			if err == nil || !strings.Contains(err.Error(), test.fault) {
				t.Fatalf("got %v, want %s", err, test.fault)
			}

			if records.Len() != 0 {
				t.Fatalf("refused overrides logged as applied: %s", records.String())
			}
		})
	}
}

func TestOverriddenTurnDoesNotExportOverridesToProcessEnvironment(t *testing.T) {
	t.Chdir(repoRoot)

	cfg, err := config.NewViperConfig()
	if err != nil {
		t.Fatal(err)
	}

	const override = "RUN_ONLY_OVERRIDE_200"

	models, err := bootstrap.NewModelRegistry(cfg, "high=claude:"+override)
	if err != nil {
		t.Fatal(err)
	}

	directory := modelEnvironmentProbe(t)

	ref, err := models.ResolveRole(provider.RolePrompt, "")
	if err != nil {
		t.Fatal(err)
	}

	router := ai.NewRouter(directory, directory)

	_, err = router.ExecutePromptWithSystem(t.Context(), provider.RolePrompt,
		"system", "task", ref, ai.ExecutionMode{}, "")
	if err != nil {
		t.Fatal(err)
	}

	environment, err := disk.Read(filepath.Join(directory, "environment"))
	if err != nil {
		t.Fatal(err)
	}

	if bytes.Contains(environment, []byte(override)) || bytes.Contains(environment, []byte("--model")) {
		t.Fatal("override leaked into process environment")
	}

	arguments, err := disk.Read(filepath.Join(directory, "arguments"))
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Contains(arguments, []byte("--model\n"+override+"\n")) {
		t.Fatalf("resolved model did not reach CLI: %s", arguments)
	}
}

func modelEnvironmentProbe(t *testing.T) string {
	t.Helper()
	directory := t.TempDir()

	script := `#!/bin/sh
/usr/bin/env > environment
printf '%s\n' "$@" > arguments
printf '%s\n' '{"type":"result","subtype":"success","is_error":false,"result":"ok"}'
`

	err := disk.Write(filepath.Join(directory, "claude"), []byte(script), disk.Shared)
	if err != nil {
		t.Fatal(err)
	}

	err = disk.Executable(filepath.Join(directory, "claude"))
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("PATH", directory+string(os.PathListSeparator)+os.Getenv("PATH"))

	return directory
}

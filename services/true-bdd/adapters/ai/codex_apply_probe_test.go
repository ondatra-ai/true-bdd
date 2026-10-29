package ai_test

import (
	"errors"
	"fmt"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/bootstrap"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/validate"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/adapters/ai"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/cli/gotool"
)

const probeBuildCode = "build code"

func TestCodexEveryCommandApplyConfinement(t *testing.T) {
	paths, err := gotool.Environment(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	for _, command := range []string{"us refine", "us apply", "scen check", probeBuildCode} {
		t.Run(command, func(t *testing.T) { probeCodexApply(t, command, paths) })
	}
}

func probeCodexApply(t *testing.T, command string, paths map[string]string) {
	t.Helper()
	root := t.TempDir()
	outside := t.TempDir()
	tools := t.TempDir()
	putCodexFile(t, filepath.Join(tools, "auth.json"), "{}")
	putCodexFile(t, filepath.Join(tools, "codex"), codexApplyProbeScript)

	cache := filepath.Join(outside, "module-cache")
	for _, path := range []string{"marker", "module-cache/github.com/ondatra-ai/marker",
		"module-cache/cache/download/github.com/ondatra-ai/marker", "module-cache/cache/vcs/marker"} {
		putCodexFile(t, filepath.Join(outside, path), "harmless probe")
	}

	goEnv := `{"GOROOT":` + strconv.Quote(paths["GOROOT"]) + `,"GOMODCACHE":` + strconv.Quote(cache) + `}`
	putCodexFile(t, filepath.Join(tools, "go"), fmt.Sprintf(dummyGoEnvironment, goEnv, cache))
	putCodexFile(t, filepath.Join(root, "outside.txt"), outside)

	userHome, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}

	putCodexFile(t, filepath.Join(root, "user-home.txt"), userHome)
	putCodexFile(t, filepath.Join(root, "go.mod"), "module probe\n\ngo 1.25\n")
	putCodexFile(t, filepath.Join(root, "package.json"), `{"scripts":{"test":"node tests/runtime.cjs"}}`)
	putCodexFile(t, filepath.Join(root, "node_modules/.bin/probe-runtime"), "#!/bin/sh\nnode tests/runtime.cjs\n")
	putCodexFile(t, filepath.Join(root, "tests/runtime.cjs"),
		`require('node:assert/strict').equal(2+2,4);console.log("node suite passed");`)
	putCodexFile(t, filepath.Join(root, "tests/suite/probe_test.go"),
		"package probe\nimport \"testing\"\nfunc TestA(t *testing.T){}\n")
	putCodexFile(t, filepath.Join(root, "docs/story.yaml"), "original")
	putCodexFile(t, filepath.Join(root, "tests/alpha/steps/existing.go"), "package steps")
	putCodexFile(t, filepath.Join(root, "readonly.txt"), "original")
	t.Setenv("CODEX_HOME", tools)
	t.Setenv("PATH", tools+":"+os.Getenv("PATH"))

	t.Setenv("TRUE_BDD_TEST_COMMAND", command)
	container := codexProbeContainer(t, root)

	applier := codexCommandApplier(container, command)
	if command == probeBuildCode {
		applier.UseWriteRoots([]string{"services/new"})
	}

	_, err = applier.Apply(t.Context(), validate.ApplyParams{SubjectID: "probe", FixPrompt: "probe",
		TmpDir: container.RunDir.GetTmpOutPath(), CodexWriter: command == probeBuildCode})
	if os.Getenv("TRUE_BDD_CODEX_SANDBOX_PROBE") != "" {
		t.Log(command + "\n" + readCodexFile(t, filepath.Join(root, "tmp/probe.log")))
	}

	if err != nil {
		t.Fatal(err)
	}
}

const codexApplyProbeScript = `#!/bin/sh
set -eu
if [ -n "${TRUE_BDD_CODEX_SANDBOX_PROBE:-}" ]; then
 probe="$TRUE_BDD_CODEX_SANDBOX_PROBE"
 outside=$(cat outside.txt)
 user_home=$(cat user-home.txt)
 cache="$outside/module-cache"
 : > tmp/probe.log
 deny() {
  printf 'DENY: %s\n' "$*" >> tmp/probe.log
  if "$probe" sandbox -C "$PWD" -P writer "$@" >> tmp/probe.log 2>&1; then exit 61; fi
  tail -n 2 tmp/probe.log | grep -Ei 'Operation not permitted|Permission denied' >/dev/null || exit 62
 }
 deny /bin/ls "$user_home/work"
 deny /bin/ls "$user_home/.codex/sessions"
 deny /bin/ls "$user_home/.claude/projects"
 deny /bin/cat "$CODEX_HOME/auth.json"
 deny /bin/cat "$outside/marker"
 deny /bin/cat "$cache/github.com/ondatra-ai/marker"
 deny /bin/cat "$cache/cache/download/github.com/ondatra-ai/marker"
 deny /bin/cat "$cache/cache/vcs/marker"
 deny /bin/sh -c 'printf forbidden > readonly.txt'
 deny /bin/sh -c 'printf forbidden > docs/sibling.yaml'
 deny /bin/sh -c 'printf forbidden > tests/alpha/sibling.go'
 deny node -e '
 const net=require("net");const s=net.connect(80,"192.0.2.1");
 s.on("error",e=>{console.error(e.code === "EPERM" ? "Operation not permitted" : e.code);process.exit(1)});
 s.on("connect",()=>process.exit(0))'
 "$probe" sandbox -C "$PWD" -P writer /bin/sh -ec '
  case "$1" in
   "build code") printf written > services/new/created.txt ;;
  esac
  printf written > tmp/scratch.txt
  go test -count=1 ./tests/suite/... -run "^TestA$"
  node --version
  npm --version
  npm test
  npx --no-install probe-runtime
  docker --version
 ' sh "$TRUE_BDD_TEST_COMMAND" >> tmp/probe.log 2>&1
 printf 'GRANTED: command write roots, scratch, selected Go replay, node, npm, docker client\n' >> tmp/probe.log
fi
request=$(cat)
result=$(printf '%s\n' "$request" | sed -n 's/^RESULT=//p')
while [ "$1" != '-o' ]; do shift; done
printf '=== FILE_START: %s ===\napplied: true\n=== FILE_END: %s ===\n' "$result" "$result" > "$2"
`

func TestCodexApplyRefusesUnsafeFileAndGlobRoots(t *testing.T) {
	for _, pattern := range []string{"escape/story.yaml", "escape/*/steps/**", "../outside/**", "dangling.yaml",
		"tmp/codex-homes/injected/**", "tmp/CODEX-HOMES/injected/**"} {
		t.Run(strings.ReplaceAll(pattern, "/", "_"), func(t *testing.T) {
			root := t.TempDir()

			outside := t.TempDir()
			tools := t.TempDir()
			putCodexFile(t, filepath.Join(tools, "auth.json"), "{}")
			putCodexFile(t, filepath.Join(tools, "codex"), unexpectedCodexSpawnScript)
			t.Setenv("CODEX_HOME", tools)
			t.Setenv("PATH", tools+":"+os.Getenv("PATH"))

			err := os.Symlink(outside, filepath.Join(root, "escape"))
			if err != nil {
				t.Fatal(err)
			}

			err = os.Symlink(filepath.Join(outside, "missing.yaml"), filepath.Join(root, "dangling.yaml"))
			if err != nil {
				t.Fatal(err)
			}

			_, err = ai.NewCodexProvider().Execute(t.Context(), ai.Request{WorkDir: root, TmpDir: filepath.Join(root, "tmp/run"),
				CodexApply: true,
				Mode:       ai.ExecutionMode{AllowedTools: []string{"Write(" + pattern + ")"}}})
			if !errors.Is(err, os.ErrPermission) {
				t.Fatalf("unsafe root was not refused: %v", err)
			}
		})
	}
}

func codexProbeContainer(t *testing.T, root string) *bootstrap.Container {
	t.Helper()
	putCodexFile(t, filepath.Join(root, "tests/single.txt"), "original")
	putCodexFile(t, filepath.Join(root, "system.tpl"), "Apply.")
	putCodexFile(t, filepath.Join(root, "user.tpl"), "RESULT={{.ResultPath}}")

	config := `engine:
  models:
    coder: codex:probe
  default_prompt_model: coder
  default_fix_model: coder
  default_apply_model: coder
paths:
  tmp_dir: ./tmp
  tmp_glob: ./tmp/**
templates:
  prompts:
`

	var applierTemplates strings.Builder
	for _, prefix := range []string{"", "apply_", "scen_check_", "build_code_"} {
		applierTemplates.WriteString("    " + prefix + "fix_applier: user.tpl\n" +
			"    " + prefix + "fix_applier_system: system.tpl\n")
	}

	config += applierTemplates.String()

	config = strings.Replace(config, "  tmp_dir: ./tmp", "  tmp_dir: "+strconv.Quote(filepath.Join(root, "tmp")), 1)
	putCodexFile(t, filepath.Join(root, "true-bdd/true-bdd.yaml"), config)
	t.Chdir(root)

	previous := slog.Default()

	t.Cleanup(func() { slog.SetDefault(previous) })

	container, err := bootstrap.NewContainer()
	if err != nil {
		t.Fatal(err)
	}

	return container
}

func codexCommandApplier(container *bootstrap.Container, command string) *validate.FixApplier {
	switch command {
	case "us refine":
		return container.RefineFixApplier
	case "us apply":
		return container.ApplyFixApplier
	case "scen check":
		return container.ScenCheckFixApplier
	default:
		return container.BuildCodeFixApplier
	}
}

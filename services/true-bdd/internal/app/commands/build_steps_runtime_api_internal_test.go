package commands

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/adapters/ai"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/stepskeleton"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/app/generators/validate"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/catalogue"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/checklist"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/domain/models/provider"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/architecture"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/config"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
)

const bodyModuleFile = "go.mod"
const bodySubjectID = "body"
const bodyCoderTier = "coder"

type bodyEvidenceTurn struct {
	system string
	user   string
	mode   provider.ExecutionMode
}

func (turn *bodyEvidenceTurn) ExecutePromptWithSystem(_ context.Context, _ provider.Role, system string, user string,
	_ provider.ModelRef, mode provider.ExecutionMode, _ string, _ ...provider.TurnOptions,
) (string, error) {
	turn.system, turn.user, turn.mode = system, user, mode

	return "", errors.New("turn captured")
}

func TestBodyTurnsReadRuntimeDocumentationWithoutLibrarySource(t *testing.T) {
	for _, transitive := range []bool{false, true} {
		t.Run(fmt.Sprintf("transitive=%t", transitive), func(t *testing.T) {
			testBodyRuntimeDocumentation(t, transitive)
		})
	}
}

func testBodyRuntimeDocumentation(t *testing.T, transitive bool) {
	t.Helper()

	turn := &bodyEvidenceTurn{}
	execution := testkitExecution(t, turn)
	arch := execution.concepts.arch
	arch.Testing.Testkit = &architecture.Testkit{}
	excluded := runtimeLibraryGraph(t, arch, transitive)

	err := execution.configure(t.Context(), arch, execution.concepts.discovery.scenarios)
	if err != nil {
		t.Fatal(err)
	}

	item := &BuildStepsItem{Suite: repairSuite, Service: repairService,
		Concept: &BuildStepsConcept{Block: catalogue.BlockWhen, Name: repairConcept}}

	err = describeConceptItem(item, arch, execution.concepts.specification,
		execution.concepts.discovery, execution.runtimeAPI)
	if err != nil {
		t.Fatal(err)
	}

	host, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	library, err := filepath.EvalSymlinks(filepath.Join(host, "local-library/testkit"))
	if err != nil {
		t.Fatal(err)
	}

	_, _ = execution.deps.FixGenerator.Generate(t.Context(), validate.GenerateParams{
		Subject: item, SubjectID: bodySubjectID, TmpDir: execution.deps.TmpDir,
		FailedCheck: checklist.ValidationResult{PromptIndex: 1},
	})
	requireRuntimeAPITurn(t, turn, execution, host, library, excluded)

	for _, tier := range []string{repairTier, bodyCoderTier} {
		_, _ = execution.deps.FixApplier.Apply(t.Context(), validate.ApplyParams{
			Subject: item, SubjectID: bodySubjectID, TmpDir: execution.deps.TmpDir, ModelTier: tier,
		})
		requireRuntimeAPITurn(t, turn, execution, host, library, excluded)
	}

	requireEmbeddedRuntimeAPI(t, execution, turn, item)
}

func requireEmbeddedRuntimeAPI(t *testing.T, execution *buildStepsExecution, turn *bodyEvidenceTurn,
	item *BuildStepsItem,
) {
	t.Helper()
	execution.deps.Critic = validate.NewStepCritic(turn, testkitModels(t),
		"templates/build-steps.critique.system.prompt.tpl", "templates/build-steps.critique.prompt.tpl")

	const selected = "selectedRuntimeUse"

	path := filepath.Join(repairSuite, "selected_runtime_use.go")
	writeHelperEvidence(t, path, "package steps\nfunc "+selected+
		"(state *State,args []string) error { return state.PrepareTree(args[0]) }\n")
	item.Concept.Definitions = []stepskeleton.Definition{{Function: selected, File: path}}

	_, _ = execution.critique(t.Context(), item, verificationTestPrompt())

	for _, instruction := range []string{"selected Public interface entries",
		"runtime API declarations with their go doc meanings",
		"selected source declarations and imports", "relying on them is not by itself a finding",
		"Fixture file contents are not supplied."} {
		if !strings.Contains(turn.system, instruction) {
			t.Errorf("critique lacks %q", instruction)
		}
	}

	normalized := strings.Join(strings.Fields(turn.user), " ")
	if !strings.Contains(normalized, "PrepareTree selects a fixture.") ||
		strings.Contains(turn.user, "PRIVATE_RUNTIME_IMPLEMENTATION") {
		t.Fatalf("critique lacks API meaning or embeds source: %s", turn.user)
	}

	if len(turn.mode.AllowedTools) != 0 || len(turn.mode.DisallowedTools) == 0 {
		t.Fatalf("critique has tools: %+v", turn.mode)
	}
}

func requireRuntimeAPITurn(t *testing.T, turn *bodyEvidenceTurn, execution *buildStepsExecution,
	host, library string, excluded []string,
) {
	t.Helper()

	requireRuntimeBodyInstructions(t, turn)

	policy := ai.ClaudeGuardPolicy{Replay: ai.NewReplayPolicy(turn.mode, host),
		EvidenceRoots: turn.mode.ReadRoots, ReadRoots: turn.mode.LibraryReadRoots}
	for _, path := range execution.runtimeAPI {
		if !strings.Contains(turn.user, path) || !slices.Contains(turn.mode.ReadRoots, path) {
			t.Fatalf("missing documentation path or grant: %s, %+v", turn.user, turn.mode)
		}

		documentation, err := disk.Read(path)
		if err != nil || !strings.Contains(string(documentation), "PrepareTree selects a fixture.") {
			t.Fatalf("missing method meaning: %s, %v", documentation, err)
		}

		requireDocumentationGrant(t, policy, host, path, library)
	}

	for _, path := range excluded {
		if slices.Contains(turn.mode.LibraryReadRoots, path) {
			t.Errorf("runtime source or archive granted: %s", path)
		}

		requireDocumentationGrant(t, policy, host, execution.runtimeAPI["github.com/ondatra-ai/true-bdd/pkg/testkit"], path)
	}

	expected, err := execution.concepts.arch.DownloadLibraries(t.Context(), host)
	if err != nil {
		t.Fatal(err)
	}

	expected = slices.DeleteFunc(expected, func(path string) bool { return slices.Contains(excluded, path) })
	if !slices.Equal(turn.mode.LibraryReadRoots, expected) {
		t.Fatalf("other library grants changed: %v, want %v", turn.mode.LibraryReadRoots, expected)
	}
}

func requireDocumentationGrant(t *testing.T, policy ai.ClaudeGuardPolicy, host, path, library string) {
	t.Helper()

	targets := []string{path, filepath.Join(filepath.Dir(path), "other.txt"), library, filepath.Join(library, "run.go")}
	for _, target := range targets {
		payload := fmt.Sprintf(`{"tool_name":"Read","cwd":%q,"tool_input":{"file_path":%q}}`, host, target)
		if allowed, reason := policy.DecidePayload([]byte(payload)); allowed != (target == path) {
			t.Errorf("read %s: %v (%s)", target, allowed, reason)
		}
	}
}

func testkitExecution(t *testing.T, turn *bodyEvidenceTurn) *buildStepsExecution {
	t.Helper()
	execution := repairExecution(t, &proposalReplies{})
	execution.registryFile = conceptRegistry
	execution.deps.TmpDir = "tmp/run"
	execution.concepts.arch.Testing.Libraries = nil

	for path, content := range map[string]string{
		bodyModuleFile: "module host\n\ngo 1.25.0\nrequire github.com/ondatra-ai/true-bdd/pkg v0.0.0\n" +
			"replace github.com/ondatra-ai/true-bdd/pkg => ./local-library\n",
		"local-library/go.mod": "module github.com/ondatra-ai/true-bdd/pkg\n\ngo 1.25.0\n",
		"local-library/testkit/run.go": "package testkit\ntype ScenarioRun struct{}\n" +
			"// PrepareTree selects a fixture.\n" +
			"func (*ScenarioRun) PrepareTree(string) error { /* PRIVATE_RUNTIME_IMPLEMENTATION */ return nil }\n",
		"local-library/proxy/cli/api.go": "package cli\n// PrepareTree selects a fixture.\nfunc PrepareTree() {}\n",
	} {
		err := disk.Write(path, []byte(content), disk.Private)
		if err != nil {
			t.Fatal(err)
		}
	}

	cfg, err := config.NewViperConfig()
	if err != nil {
		t.Fatal(err)
	}

	models := testkitModels(t)

	execution.deps.FixGenerator = validate.NewFixPromptGeneratorWithPaths(turn, cfg, models,
		"templates/build-steps.fix-generator.system.prompt.tpl", "templates/build-steps.fix-generator.prompt.tpl")
	execution.deps.FixApplier = validate.NewFixApplierWithPaths(turn, cfg, models,
		"templates/build-steps.fix-applier.system.prompt.tpl", "templates/build-steps.fix-applier.prompt.tpl")

	return execution
}

func testkitModels(t *testing.T) *provider.Registry {
	t.Helper()

	models, err := provider.NewRegistry(map[string]string{repairTier: "claude:runtime-api", bodyCoderTier: "codex:test"},
		map[provider.Role]string{provider.RolePrompt: repairTier,
			provider.RoleFix: repairTier, provider.RoleApply: repairTier})
	if err != nil {
		t.Fatal(err)
	}

	return models
}

func TestBodyTurnsWithoutTestkitKeepTheirPromptsAndPolicyRoots(t *testing.T) {
	turn := &bodyEvidenceTurn{}
	execution := testkitExecution(t, turn)

	host, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	arch, discovery := execution.concepts.arch, execution.concepts.discovery

	err = disk.Remove(bodyModuleFile)
	if err != nil {
		t.Fatal(err)
	}

	err = execution.configureRoots(arch, discovery.scenarios, host)
	if err != nil {
		t.Fatal(err)
	}

	legacyRoots := slices.Clone(execution.readRoots)

	err = execution.configure(t.Context(), arch, discovery.scenarios)
	if err != nil {
		t.Fatalf("a host without testkit must not resolve a Go package: %v", err)
	}

	item := &BuildStepsItem{Suite: repairSuite, Service: repairService,
		Concept: &BuildStepsConcept{Block: catalogue.BlockWhen, Name: repairConcept}}

	err = describeConceptItem(item, arch, execution.concepts.specification, discovery, execution.runtimeAPI)
	if err != nil {
		t.Fatal(err)
	}

	_, _ = execution.deps.FixGenerator.Generate(t.Context(), validate.GenerateParams{
		Subject: item, SubjectID: bodySubjectID, TmpDir: execution.deps.TmpDir,
		FailedCheck: checklist.ValidationResult{PromptIndex: 1},
	})
	proposal := turn.user
	requireLegacyBodyRoots(t, turn, legacyRoots)

	for _, tier := range []string{repairTier, bodyCoderTier} {
		_, _ = execution.deps.FixApplier.Apply(t.Context(), validate.ApplyParams{
			Subject: item, SubjectID: bodySubjectID, TmpDir: execution.deps.TmpDir, ModelTier: tier,
		})
		requireLegacyBodyRoots(t, turn, legacyRoots)
	}

	context, err := critiqueContext(item, "package steps")
	if err != nil || strings.Contains(context, "testkit") || strings.Contains(proposal, "testkit:") {
		t.Fatalf("undeclared library changed body evidence: %s, %v", context, err)
	}
}

func requireLegacyBodyRoots(t *testing.T, turn *bodyEvidenceTurn, roots []string) {
	t.Helper()

	if turn.user == "" || !slices.Equal(turn.mode.ReadRoots, roots) || len(turn.mode.LibraryReadRoots) != 0 {
		t.Fatalf("undeclared testkit changed the body roots: %+v", turn.mode)
	}
}

func TestRuntimeDocumentationFailurePrecedesAnyAITurn(t *testing.T) {
	turn := &bodyEvidenceTurn{}
	execution := testkitExecution(t, turn)
	execution.concepts.arch.Testing.Testkit = &architecture.Testkit{}

	err := disk.Remove("local-library/testkit/run.go")
	if err != nil {
		t.Fatal(err)
	}

	err = execution.prepareRuntimeAPI(t.Context(), execution.concepts.arch)
	if err == nil || !strings.Contains(err.Error(), "github.com/ondatra-ai/true-bdd/pkg/testkit") || turn.user != "" {
		t.Fatalf("documentation startup refusal: %v, turn=%s", err, turn.user)
	}
}

func TestOversizedRuntimeAPICritiqueRefusesDistinctly(t *testing.T) {
	turn := &bodyEvidenceTurn{}

	execution := testkitExecution(t, turn)
	content := []byte("func Oversized()\n    " + strings.Repeat("Complete API meaning. ", ai.DefaultPromptLimit))
	stub := t.TempDir()
	writeHelperEvidence(t, filepath.Join(stub, "claude"), "#!/bin/sh\nexit 97\n")

	err := os.Chmod(filepath.Join(stub, "claude"), 0750)
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("PATH", stub+string(os.PathListSeparator)+os.Getenv("PATH"))
	writeHelperEvidence(t, filepath.Join(repairSuite, "oversized.go"),
		"package steps\nfunc useOversized(state *State,args []string) error { testkit.Oversized(); return nil }\n")

	err = disk.Write("tmp/oversized-api.txt", content, disk.Private)
	if err != nil {
		t.Fatal(err)
	}

	item := &BuildStepsItem{Suite: repairSuite, Concept: &BuildStepsConcept{
		Definitions: []stepskeleton.Definition{{Function: "useOversized"}}},
		Context: "runtime_api:\n  testkit: tmp/oversized-api.txt\n"}

	host, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	execution.deps.Critic = validate.NewStepCritic(ai.NewRouter(host, execution.deps.TmpDir), testkitModels(t),
		"templates/build-steps.critique.system.prompt.tpl", "templates/build-steps.critique.prompt.tpl")
	_, err = execution.critique(t.Context(), item, verificationTestPrompt())

	var refusal *ai.PromptLimitError
	if !errors.As(err, &refusal) {
		t.Fatalf("want distinct prompt limit refusal before a provider starts: %v", err)
	}
}

func TestStepEvidenceRootsAreUnique(t *testing.T) {
	execution := testkitExecution(t, &bodyEvidenceTurn{})

	host, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	execution.configureEvidenceRoots(host, []string{repairSuite, repairSuite, "tests/calc/./steps"},
		[]string{conceptRegistry, conceptRegistry})

	want := []string{filepath.Join(host, repairSuite), filepath.Join(host, conceptRegistry)}
	if !slices.Equal(execution.readRoots, want) {
		t.Fatalf("duplicate evidence roots: %v, want %v", execution.readRoots, want)
	}
}

func runtimeLibraryGraph(t *testing.T, arch *architecture.Architecture, transitive bool) []string {
	t.Helper()
	proxy, cache := t.TempDir(), t.TempDir()
	t.Cleanup(func() {
		err := filepath.WalkDir(cache, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			if entry.IsDir() {
				return os.Chmod(path, 0o700)
			}

			return nil
		})
		if err != nil {
			t.Error(err)
		}
	})

	const (
		runtimeModule = "github.com/ondatra-ai/true-bdd/pkg"
		supportModule = "example.com/support"
		version       = "v1.0.0"
	)

	writeBodyModule(t, proxy, runtimeModule, "module "+runtimeModule+"\n\ngo 1.25.0\n")
	writeBodyModule(t, proxy, supportModule, "module "+supportModule+"\n\ngo 1.25.0\n"+
		"require "+runtimeModule+" "+version+"\n")
	t.Setenv("GOPROXY", "file://"+filepath.ToSlash(proxy))
	t.Setenv("GOSUMDB", "off")
	t.Setenv("GOMODCACHE", cache)

	arch.Testing.Libraries = []architecture.Library{{Module: supportModule, Version: version}}
	if !transitive {
		arch.Testing.Libraries = append(arch.Testing.Libraries, architecture.Library{Module: runtimeModule, Version: version})
	}

	return []string{filepath.Join(cache, runtimeModule+"@"+version),
		filepath.Join(cache, "cache/download", runtimeModule, "@v", version+".zip")}
}

func writeBodyModule(t *testing.T, proxy, module, manifest string) {
	t.Helper()

	var archive bytes.Buffer

	writer := zip.NewWriter(&archive)
	for name, content := range map[string]string{bodyModuleFile: manifest, "source.go": "package library\n"} {
		file, err := writer.Create(module + "@v1.0.0/" + name)
		if err != nil {
			t.Fatal(err)
		}

		_, err = file.Write([]byte(content))
		if err != nil {
			t.Fatal(err)
		}
	}

	err := writer.Close()
	if err != nil {
		t.Fatal(err)
	}

	for suffix, content := range map[string][]byte{
		".mod": []byte(manifest), ".info": []byte(`{"Version":"v1.0.0","Time":"2026-01-01T00:00:00Z"}`),
		".zip": archive.Bytes(),
	} {
		err := disk.Write(filepath.Join(proxy, module, "@v", "v1.0.0"+suffix), content, disk.Private)
		if err != nil {
			t.Fatal(err)
		}
	}
}

func requireRuntimeBodyInstructions(t *testing.T, turn *bodyEvidenceTurn) {
	t.Helper()

	for _, instruction := range []string{"semantic core and Public interface first", "saved complete go doc -all files",
		"runtime_api", "then read suite source and fixture inputs"} {
		if !strings.Contains(turn.user, instruction) {
			t.Errorf("body turn lacks %q", instruction)
		}
	}
}

package inventory

import (
	"os"
	"path/filepath"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/architecture"
	"gopkg.in/yaml.v3"
)

// checklistStems are the five per-command checklists resolved by
// convention (plan §3.4 / README-testids inventory-doc-checklist-<stem>).
//
//nolint:gochecknoglobals // fixed checklist vocabulary
var checklistStems = []string{
	"us-create", "us-refine", "us-apply", "build-tests", "build-code",
}

// docResult is one chip's status plus an optional parse error.
type docResult struct {
	status string
	err    string
}

// scanDocuments builds the document/directory/checklist chip maps (plan
// §3.4): key→status, plus key→error for the chips that failed to parse.
func scanDocuments(folder string, cfg resolvedConfig) (map[string]string, map[string]string) {
	statuses := make(map[string]string)
	errs := make(map[string]string)

	record := func(key string, result docResult) {
		statuses[key] = result.status
		if result.err != "" {
			errs[key] = result.err
		}
	}

	record("config", docResult{status: cfg.configStatus, err: cfg.configError})
	record("product", yamlDoc(filepath.Join(folder, cfg.productRel)))
	record("architecture", architectureDoc(filepath.Join(folder, cfg.architectureRel)))
	record("registry", registryDoc(filepath.Join(folder, cfg.registryRel)))
	record("stories-dir", dirDoc(filepath.Join(folder, cfg.storiesRel)))
	record("epics-dir", dirDoc(filepath.Join(folder, cfg.epicsRel)))

	for _, stem := range checklistStems {
		path := filepath.Join(folder, cfg.checklistsRel, stem+".yaml")
		record("checklist-"+stem, yamlDoc(path))
	}

	return statuses, errs
}

// yamlDoc classifies a document expected to be a parseable YAML file:
// missing when absent, invalid when it fails to parse, present otherwise.
func yamlDoc(path string) docResult {
	data, err := os.ReadFile(path)
	if err != nil {
		return docResult{status: StatusMissing}
	}

	var probe any

	err = yaml.Unmarshal(data, &probe)
	if err != nil {
		return docResult{status: StatusInvalid, err: err.Error()}
	}

	return docResult{status: StatusPresent}
}

// rawArchitectureShape mirrors architecture.Loader's decode shape: the
// `architecture.services:` list whose entries name the source roots
// build code may edit. Only presence of the list matters for the chip.
type rawArchitectureShape struct {
	Architecture struct {
		Services []yaml.Node `yaml:"services"`
	} `yaml:"architecture"`
}

// rawWorkspaceArchitectureShape mirrors the workspace file-as-source schema
// (services/bdd-web/src/app/lib/workspace/derive.ts's deriveArchitecture): a
// top-level `services:` MAP (no `architecture:` wrapper, no list) — the
// schema the workspace UI's architecture.yaml pages/editor speak. Only
// presence of at least one service matters for the chip.
type rawWorkspaceArchitectureShape struct {
	Services map[string]yaml.Node `yaml:"services"`
}

// architectureDoc classifies docs/architecture/architecture.yaml: missing
// when absent, invalid on a parse error, and — mirroring ErrNoServices —
// invalid when the file parses but declares no services under EITHER
// supported schema. Two schemas coexist in this codebase: the legacy engine
// shape the build-code loader walks (`architecture.services:` list,
// architecture.Loader.Load) and the newer workspace file-as-source shape
// (top-level `services:` map). A generic yamlDoc would mark any
// syntactically valid YAML `present`, advertising an architecture no reader
// can actually walk.
func architectureDoc(path string) docResult {
	data, err := os.ReadFile(path)
	if err != nil {
		return docResult{status: StatusMissing}
	}

	var raw rawArchitectureShape

	err = yaml.Unmarshal(data, &raw)
	if err != nil {
		return docResult{status: StatusInvalid, err: err.Error()}
	}

	if len(raw.Architecture.Services) > 0 {
		return docResult{status: StatusPresent}
	}

	var flat rawWorkspaceArchitectureShape

	err = yaml.Unmarshal(data, &flat)
	if err == nil && len(flat.Services) > 0 {
		return docResult{status: StatusPresent}
	}

	return docResult{status: StatusInvalid, err: architecture.ErrNoServices.Error()}
}

// dirDoc classifies a path expected to be a directory: missing when
// absent, not_a_dir when a regular file occupies it, present otherwise.
func dirDoc(path string) docResult {
	info, err := os.Stat(path)
	if err != nil {
		return docResult{status: StatusMissing}
	}

	if !info.IsDir() {
		return docResult{status: StatusNotADir}
	}

	return docResult{status: StatusPresent}
}

// registryDoc classifies the configured scenario registry
// (documents.scenarios_yaml, conventionally docs/scenarios.yaml): missing,
// invalid on a parse error, present_empty when its scenarios map is empty,
// else present.
func registryDoc(path string) docResult {
	data, err := os.ReadFile(path)
	if err != nil {
		return docResult{status: StatusMissing}
	}

	var raw rawRegistry

	err = yaml.Unmarshal(data, &raw)
	if err != nil {
		return docResult{status: StatusInvalid, err: err.Error()}
	}

	if len(raw.Scenarios) == 0 {
		return docResult{status: StatusPresentEmpty}
	}

	return docResult{status: StatusPresent}
}

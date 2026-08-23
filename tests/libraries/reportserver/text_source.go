package reportserver

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ondatra-ai/true-bdd/tests/libraries/reporter"
	"github.com/ondatra-ai/true-bdd/tests/libraries/runner"
)

// Pseudo-refs for bodies that are not turn artifacts.
const (
	refJudgeSystem   = "judge:system"
	refJudgeUser     = "judge:user"
	refJudgeResponse = "judge:response"
	refCLIStdout     = "cli:stdout"
	refCLIStderr     = "cli:stderr"
	refJudgeSpec     = "manifest:judge_spec"
	// The two sides of every check the run performed, plus the file its
	// fixes mutate. Named by the run's own log, so they stay lookups
	// like every other ref rather than a path the client supplies.
	refItemsFile = "run:items"
	refChecklist = "run:checklist"
	refTarget    = "run:target"
	refCommitted = "run:committed"
)

// resolveText returns the body a ref names, and whether it exists. Refs are
// resolved by LOOKUP against the fixture's own loaded artifacts, never by
// joining a client string onto a path — no traversal surface to get wrong.
func resolveText(fixture *reporter.Fixture, ref string) (string, bool) {
	body, present, handled := fixedRef(fixture, ref)
	if handled {
		return body, present
	}

	parts := strings.Split(ref, ":")

	switch {
	case len(parts) == 4 && parts[0] == "turn":
		return turnArtifact(fixture, parts[1], parts[2], parts[3])
	case len(parts) == 3 && parts[0] == "testrun":
		return testRunStream(fixture, parts[1], parts[2])
	default:
		return "", false
	}
}

// fixedRef resolves the pseudo-refs that name exactly one body. Three
// results, not two: `present` (body found) and `handled` (ref recognized)
// are separate so a readable-but-empty file doesn't collapse into "absent".
func fixedRef(fixture *reporter.Fixture, ref string) (string, bool, bool) {
	switch ref {
	case refJudgeSystem:
		return presenceOf(sidecar(fixture, runner.JudgeSystemFile))
	case refJudgeUser:
		return presenceOf(sidecar(fixture, runner.JudgeUserFile))
	case refJudgeResponse:
		return presenceOf(sidecar(fixture, runner.JudgeResponseFile))
	case refCLIStdout:
		return presenceOf(recordFile(fixture, stdoutOf))
	case refCLIStderr:
		return presenceOf(recordFile(fixture, stderrOf))
	case refJudgeSpec:
		if fixture.Manifest == nil {
			return "", false, true
		}

		return presenceOf(fixture.Manifest.JudgeSpec)
	default:
		return provenanceRef(fixture, ref)
	}
}

// presenceOf keeps the old contract for the bodies that have no way to
// distinguish empty from missing: they are read through helpers that
// return "" for both.
func presenceOf(body string) (string, bool, bool) {
	return body, body != "", true
}

// provenanceRef resolves the refs that name a file the run's own log
// pointed at, rather than one the harness wrote. Here the read itself
// reports presence, so a zero-byte file is present.
func provenanceRef(fixture *reporter.Fixture, ref string) (string, bool, bool) {
	var logged string

	switch ref {
	case refItemsFile:
		logged = fixture.Meta.ItemsFile
	case refChecklist:
		logged = fixture.Meta.ChecklistPath
	case refTarget:
		logged = fixture.Meta.TargetFile
	case refCommitted:
		logged = fixture.Meta.CommittedFile
	default:
		return "", false, false
	}

	body, ok := reporter.ReadContained(fixture.Dir, logged)

	return body, ok, true
}

// provenanceRefs lists the files behind every check the run performed.
// Run-level, so they are resolved once and every turn points at the same bodies.
func provenanceRefs(fixture *reporter.Fixture) []ArtifactRef {
	candidates := []struct{ ref, kind, path string }{
		{refItemsFile, "subject from", fixture.Meta.ItemsFile},
		{refChecklist, "checklist", fixture.Meta.ChecklistPath},
		// Both, though usually only one resolves (see RunMetadata.CommittedFile).
		{refTarget, "fixes written to", fixture.Meta.TargetFile},
		{refCommitted, "fixes committed to", fixture.Meta.CommittedFile},
	}

	refs := []ArtifactRef{}

	for _, candidate := range candidates {
		body, present := resolveText(fixture, candidate.ref)
		if !present {
			continue
		}

		refs = append(refs, ArtifactRef{
			Ref:      candidate.ref,
			Kind:     candidate.kind,
			Name:     candidate.path,
			RepoPath: containedPath(fixture.RelDir, candidate.path),
			Bytes:    len(body),
		})
	}

	return refs
}

// stdoutOf and stderrOf name the record's captured CLI streams.
func stdoutOf(record *runner.HarnessRecord) string { return record.StdoutFile }
func stderrOf(record *runner.HarnessRecord) string { return record.StderrFile }

// sidecar reads one file the harness recorded beside the run.
func sidecar(fixture *reporter.Fixture, name string) string {
	if fixture.Record == nil || !hasArtifact(fixture.Record, name) {
		return ""
	}

	blob, err := os.ReadFile(filepath.Join(fixture.Dir, runner.SpawnLogDir, name))
	if err != nil {
		return ""
	}

	return string(blob)
}

// recordFile reads a stream the harness record points at.
func recordFile(fixture *reporter.Fixture, pick func(*runner.HarnessRecord) string) string {
	if fixture.Record == nil {
		return ""
	}

	path := pick(fixture.Record)
	if path == "" {
		return ""
	}

	blob, err := os.ReadFile(path)
	if err != nil {
		return ""
	}

	return string(blob)
}

// turnArtifact resolves "turn:<index>:<in|out>:<position>".
func turnArtifact(fixture *reporter.Fixture, turnIndex, side, position string) (string, bool) {
	turn, found := indexInto(fixture.Turns, turnIndex)
	if !found {
		return "", false
	}

	artifacts := turn.Inputs
	if side == "out" {
		artifacts = turn.Outputs
	}

	artifact, present := indexIntoValues(artifacts, position)
	if !present || artifact.Missing {
		return "", false
	}

	return artifact.Content, true
}

// testRunStream resolves "testrun:<index>:<stdout|stderr>".
func testRunStream(fixture *reporter.Fixture, index, stream string) (string, bool) {
	run, ok := indexIntoValues(fixture.TestRuns, index)
	if !ok {
		return "", false
	}

	artifact := run.Stdout
	if stream == "stderr" {
		artifact = run.Stderr
	}

	if artifact.Missing {
		return "", false
	}

	return artifact.Content, true
}

// indexInto bounds-checks a pointer slice against a decimal string.
func indexInto[T any](items []*T, raw string) (*T, bool) {
	position, err := strconv.Atoi(raw)
	if err != nil || position < 0 || position >= len(items) {
		var zero *T

		return zero, false
	}

	return items[position], true
}

// indexIntoValues bounds-checks a value slice against a decimal string.
func indexIntoValues[T any](items []T, raw string) (T, bool) {
	var zero T

	position, err := strconv.Atoi(raw)
	if err != nil || position < 0 || position >= len(items) {
		return zero, false
	}

	return items[position], true
}

// fixtureRefs lists the non-turn bodies a fixture can serve, so the UI
// can offer exactly what exists rather than probing.
func fixtureRefs(fixture *reporter.Fixture) []ArtifactRef {
	refs := []ArtifactRef{}

	// file is the sidecar's name inside bdd-cli-logs, empty for a body
	// that is not a file on disk (the rubric comes from the manifest).
	candidates := []struct {
		ref  string
		kind string
		file string
	}{
		{refCLIStdout, "cli stdout", spawnStdoutFile},
		{refCLIStderr, "cli stderr", spawnStderrFile},
		{refJudgeSystem, "judge system prompt", runner.JudgeSystemFile},
		{refJudgeUser, "judge user prompt", runner.JudgeUserFile},
		{refJudgeResponse, "judge response", runner.JudgeResponseFile},
		{refJudgeSpec, "judge rubric", ""},
	}

	for _, candidate := range candidates {
		body, present := resolveText(fixture, candidate.ref)
		if !present {
			continue
		}

		refs = append(refs, ArtifactRef{
			Ref:      candidate.ref,
			Kind:     candidate.kind,
			Name:     candidate.ref,
			RepoPath: sidecarRepoPath(fixture, candidate.file),
			Bytes:    len(body),
		})
	}

	return refs
}

// The CLI stream names the spawn log writes (runner/spawn_log.go).
const (
	spawnStdoutFile = "cli-stdout.txt"
	spawnStderrFile = "cli-stderr.txt"
)

// sidecarRepoPath locates one bdd-cli-logs file from the repo root.
func sidecarRepoPath(fixture *reporter.Fixture, name string) string {
	if name == "" || fixture.RelDir == "" {
		return ""
	}

	return containedPath(fixture.RelDir, filepath.Join(runner.SpawnLogDir, name))
}

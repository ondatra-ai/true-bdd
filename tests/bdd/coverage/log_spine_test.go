package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestAttributeApplies pins the consumed-state attribution machine.
func TestAttributeApplies(t *testing.T) {
	t.Parallel()

	seg := logSegment{Events: []logEvent{
		{Kind: evFixGenerated, Subject: "a", PromptIndex: 2},
		{Kind: evFixGenerated, Subject: "a", PromptIndex: 2}, // regeneration replaces
		{Kind: evFixApplied, Subject: "a"},
		{Kind: evFixApplied, Subject: "a"}, // second apply has no generation left
		{Kind: evFixGenerated, Subject: "b", PromptIndex: 1},
		{Kind: evFixApplied, Subject: "b"},
	}}

	got := attributeApplies(seg)
	if len(got) != 3 {
		t.Fatalf("attributions: got %d, want 3", len(got))
	}

	if got[0].PromptIndex != 2 || got[1].PromptIndex != 0 || got[2].PromptIndex != 1 {
		t.Errorf("wrong attribution: %+v", got)
	}
}

// TestCleanWalkProven pins the convergence heuristic: strictly more
// post-apply saves than one full items×prompts pass, and no generation
// after the last apply.
func TestCleanWalkProven(t *testing.T) {
	t.Parallel()

	base := []logEvent{
		{Kind: evLoadedPrompts, Items: 1, Prompts: 2},
		{Kind: evResultSaved}, {Kind: evResultSaved},
		{Kind: evFixGenerated, Subject: "a", PromptIndex: 2},
		{Kind: evFixApplied, Subject: "a"},
	}

	saves := func(n int) []logEvent {
		events := make([]logEvent, n)
		for i := range events {
			events[i] = logEvent{Kind: evResultSaved}
		}

		return events
	}

	converged := logSegment{Events: append(append([]logEvent{}, base...), saves(4)...)}
	if !cleanWalkProven(converged) {
		t.Error("restart + full re-walk (4 saves > 2) must prove a clean walk")
	}

	maxAttempts := logSegment{Events: append(append([]logEvent{}, base...), saves(2)...)}
	if cleanWalkProven(maxAttempts) {
		t.Error("bare restart (2 saves == items×prompts) must NOT prove a clean walk")
	}

	genAfter := logSegment{Events: append(append([]logEvent{}, base...),
		logEvent{Kind: evResultSaved}, logEvent{Kind: evResultSaved},
		logEvent{Kind: evFixGenerated, Subject: "a", PromptIndex: 1},
		logEvent{Kind: evResultSaved}, logEvent{Kind: evResultSaved}, logEvent{Kind: evResultSaved})}
	if cleanWalkProven(genAfter) {
		t.Error("a generation after the last apply must NOT prove a clean walk")
	}

	if cleanWalkProven(logSegment{Events: base[:2]}) {
		t.Error("a walk without any apply proves nothing")
	}

	warnAfter := logSegment{Events: append(append([]logEvent{}, base...),
		logEvent{Kind: evResultSaved}, logEvent{Kind: evResultSaved},
		logEvent{Kind: evResultSaved}, logEvent{Kind: evWarnProtocol})}
	if cleanWalkProven(warnAfter) {
		t.Error("a protocol WARN after the last apply must invalidate the clean walk")
	}
}

// TestParseLogSpine pins segmentation at Loaded prompts boundaries and
// partition binding via file path fields.
func TestParseLogSpine(t *testing.T) {
	t.Parallel()

	lines := `{"msg":"Loaded prompts","command":"us mini","items":1,"prompts":2}
{"msg":"Result file saved","file":"tmp/2026-01-01-00-00/01-9.9-checklist-mini-s-result.yaml"}
{"msg":"Generating fix prompt","subjectID":"9.9","promptIndex":2,"section":"mini/s","iteration":1}
{"msg":"Fix applied successfully","subjectID":"9.9"}
{"msg":"irrelevant record"}
{"msg":"Loaded prompts","command":"us mini","items":1,"prompts":2}
{"msg":"Result file saved","file":"tmp/2026-01-01-00-05/01-9.9-checklist-mini-s-result.yaml"}
`

	path := filepath.Join(t.TempDir(), "log.json")

	err := os.WriteFile(path, []byte(lines), 0o644)
	if err != nil {
		t.Fatalf("writing log: %v", err)
	}

	segments, malformed, err := parseLogSpine(path)
	if err != nil {
		t.Fatalf("parsing: %v", err)
	}

	if malformed != 0 {
		t.Errorf("malformed lines: got %d, want 0", malformed)
	}

	if len(segments) != 2 {
		t.Fatalf("segments: got %d, want 2", len(segments))
	}

	if segments[0].Partition != "2026-01-01-00-00" || segments[1].Partition != "2026-01-01-00-05" {
		t.Errorf("partition binding wrong: %q / %q", segments[0].Partition, segments[1].Partition)
	}

	if len(segments[0].Events) != 4 {
		t.Errorf("segment 0 events: got %d, want 4 (irrelevant line dropped)", len(segments[0].Events))
	}
}

// TestParseLogSpineEdgeCases covers empty and garbled logs.
func TestParseLogSpineEdgeCases(t *testing.T) {
	t.Parallel()

	var (
		segments  []logSegment
		malformed int
		err       error
	)

	empty := filepath.Join(t.TempDir(), "empty.json")

	writeErr := os.WriteFile(empty, nil, 0o644)
	if writeErr != nil {
		t.Fatalf("writing empty log: %v", writeErr)
	}

	segments, malformed, err = parseLogSpine(empty)
	if err != nil || len(segments) != 0 || malformed != 0 {
		t.Errorf("zero-byte log must yield nothing: %v %d %v", segments, malformed, err)
	}

	garbled := filepath.Join(t.TempDir(), "garbled.json")

	garbledErr := os.WriteFile(garbled, []byte(
		"this is not JSON\n{\"msg\":\"Loaded prompts\",\"command\":\"us mini\"}\n"), 0o644)
	if garbledErr != nil {
		t.Fatalf("writing garbled log: %v", garbledErr)
	}

	segments, malformed, err = parseLogSpine(garbled)
	if err != nil || len(segments) != 1 || malformed != 1 {
		t.Errorf("garbled log must count 1 malformed line: %v %d %v", segments, malformed, err)
	}
}

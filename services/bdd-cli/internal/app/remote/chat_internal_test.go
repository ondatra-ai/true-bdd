package remote

import (
	"context"
	"strings"
	"testing"
	"time"
)

func strPtr(s string) *string { return &s }

// TestChatSystemPromptDemandsExactFormatting guards against a real regression
// (a10): a real Claude turn reproduced the WHOLE file with a subtly different
// indentation, which the client-side YAML gate correctly rejected as invalid
// (P10b) — so the edit was silently discarded and never reached disk. The
// system prompt must explicitly demand byte-for-byte fidelity outside the
// requested change.
func TestChatSystemPromptDemandsExactFormatting(t *testing.T) {
	path := archYAMLPath
	prompt := chatSystemPrompt(chatPayload{CurrentPath: &path, CurrentContent: strPtr("version: 1\n")})

	for _, phrase := range []string{"byte-for-byte", "indentation", "valid YAML"} {
		if !strings.Contains(prompt, phrase) {
			t.Fatalf("system prompt must demand exact formatting (missing %q): %s", phrase, prompt)
		}
	}
}

// TestStripAccidentalDocumentMarkers reproduces a LIVE a10 failure: asked for
// "no code fences", real Claude (opus-4) instead wrapped new_content in a
// leading/trailing YAML `---` document separator. Go's yaml.Unmarshal
// silently decodes only the first document in the resulting two-document
// stream (so the CLI's own gate accepts it), but the browser's strict
// single-document `YAML.parse()` throws "multiple documents" — so the
// client-side P10b gate discarded an otherwise-correct edit FOREVER (the
// disk write never even reached the CLI). This must be stripped server-side
// before the edit is ever returned to the browser.
func TestStripAccidentalDocumentMarkers(t *testing.T) {
	wrapped := "---\nversion: 1\nterms:\n  - name: A\n\n---\n"

	stripped := stripAccidentalDocumentMarkers(wrapped)

	if strings.Contains(stripped, "---") {
		t.Fatalf("expected every document marker stripped, got %q", stripped)
	}

	if !isValidYAML([]byte(stripped)) {
		t.Fatalf("stripped content must be valid single-document YAML: %q", stripped)
	}
}

func TestStripAccidentalDocumentMarkersLeavesUnwrappedContentUntouched(t *testing.T) {
	plain := "version: 1\nterms:\n  - name: A\n"

	if got := stripAccidentalDocumentMarkers(plain); got != plain {
		t.Fatalf("unwrapped content must pass through byte-for-byte, got %q", got)
	}
}

// TestParseChatResultStripsDocumentMarkersFromEdit is the end-to-end
// regression test using the EXACT structured-result shape captured from the
// live a10 failure (leading "---\n" + trailing "\n\n---\n").
func TestParseChatResultStripsDocumentMarkersFromEdit(t *testing.T) {
	raw := `{"reply_text": "done", "edit": {"path": "docs/architecture/architecture.yaml", ` +
		`"new_content": "---\nversion: 1\nterms:\n  - name: term-x\n\n---\n"}}`

	result, ok := parseChatResult(raw)
	if !ok {
		t.Fatalf("expected a well-formed parse")
	}

	if result.Edit == nil {
		t.Fatalf("expected an edit")
	}

	if strings.Contains(result.Edit.NewContent, "---") {
		t.Fatalf("edit.new_content must have its document markers stripped: %q", result.Edit.NewContent)
	}

	if !isValidYAML([]byte(result.Edit.NewContent)) {
		t.Fatalf("stripped new_content must be valid single-document YAML: %q", result.Edit.NewContent)
	}
}

func TestDeterministicChatTurnAddTerm(t *testing.T) {
	content := "version: 1\nterms:\n  - name: Existing\n"
	result := deterministicChatTurn(chatPayload{
		Conversation:   []chatMessage{{Role: chatRoleUser, Content: "@probe add-term NewTerm"}},
		CurrentPath:    strPtr(archYAMLPath),
		CurrentContent: strPtr(content),
	})

	if result.Edit == nil {
		t.Fatalf("expected an edit, got %+v", result)
	}

	if result.Edit.Path != archYAMLPath {
		t.Fatalf("edit must target the current path: %+v", result.Edit)
	}

	if !strings.Contains(result.Edit.NewContent, "NewTerm") || !isValidYAML([]byte(result.Edit.NewContent)) {
		t.Fatalf("new_content must be a valid, schema-aware insertion: %q", result.Edit.NewContent)
	}
}

func TestDeterministicChatTurnAddScenario(t *testing.T) {
	content := "metadata:\n  title: t\n\nscenarios:\n  INT-901:\n    description: existing\n"
	result := deterministicChatTurn(chatPayload{
		Conversation:   []chatMessage{{Role: chatRoleUser, Content: "@probe add-scenario E2E-9999"}},
		CurrentPath:    strPtr(scenariosYAMLPath),
		CurrentContent: strPtr(content),
	})

	if result.Edit == nil || !strings.Contains(result.Edit.NewContent, "E2E-9999") {
		t.Fatalf("expected a scenario insertion edit, got %+v", result)
	}

	if !isValidYAML([]byte(result.Edit.NewContent)) {
		t.Fatalf("scenario insertion must stay valid YAML: %q", result.Edit.NewContent)
	}
}

func TestDeterministicChatTurnNonFilePageNeverEdits(t *testing.T) {
	result := deterministicChatTurn(chatPayload{
		Conversation: []chatMessage{{Role: chatRoleUser, Content: "@probe add-term nowrite-x"}},
		CurrentPath:  nil,
	})

	if result.Edit != nil {
		t.Fatalf("a non-file page (no current_path) must NEVER produce an edit, got %+v", result)
	}
}

func TestDeterministicChatTurnUnrecognizedMessageRepliesWithoutEdit(t *testing.T) {
	result := deterministicChatTurn(chatPayload{
		Conversation:   []chatMessage{{Role: chatRoleUser, Content: "just chatting, no directive here"}},
		CurrentPath:    strPtr(archYAMLPath),
		CurrentContent: strPtr("version: 1\nterms: []\n"),
	})

	if result.Edit != nil {
		t.Fatalf("an unrecognized message must reply without an edit, got %+v", result)
	}

	if result.ReplyText == "" {
		t.Fatalf("expected a non-empty reply")
	}
}

func TestParseChatResultMalformed(t *testing.T) {
	if _, ok := parseChatResult("not json at all"); ok {
		t.Fatalf("a non-JSON reply must be reported as malformed")
	}
}

func TestParseChatResultExtractsFromProseWrapper(t *testing.T) {
	raw := "Sure! Here you go:\n```json\n" +
		`{"reply_text": "done", "edit": {"path": "docs/architecture/architecture.yaml", "new_content": "version: 1\n"}}` +
		"\n```"

	result, ok := parseChatResult(raw)
	if !ok {
		t.Fatalf("expected the JSON object to be extracted from the prose wrapper")
	}

	if result.ReplyText != "done" || result.Edit == nil || result.Edit.Path != archYAMLPath {
		t.Fatalf("unexpected parsed result: %+v", result)
	}
}

func TestClaudeChatTurnHonoursCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	handler := &chatHandler{deterministic: false}

	result := handler.turn(ctx, chatPayload{CurrentPath: strPtr(archYAMLPath)})

	if result.Error != chatErrTimeout || result.Edit != nil {
		t.Fatalf("a cancelled context must surface a timeout error with no edit, got %+v", result)
	}
}

func TestClaudeChatTurnHonoursExpiredDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()

	time.Sleep(time.Millisecond)

	handler := &chatHandler{deterministic: false}
	result := handler.turn(ctx, chatPayload{})

	if result.Error != chatErrTimeout {
		t.Fatalf("an expired deadline must surface a timeout error, got %+v", result)
	}
}

func TestNewChatHandlerReadsDriverEnv(t *testing.T) {
	t.Setenv(chatDriverEnv, chatDriverDeterministic)

	h := newChatHandler("/tmp/whatever")
	if !h.deterministic {
		t.Fatalf("expected the deterministic driver to be enabled from the env var")
	}
}

func TestEnforceTargetBindingRejectsWrongTarget(t *testing.T) {
	result, ok := parseChatResult(
		`{"reply_text": "ok", "edit": {"path": "docs/prd/prd.yaml", "new_content": "title: x\n"}}`,
	)
	if !ok {
		t.Fatalf("expected a well-formed parse")
	}

	currentPath := archYAMLPath

	bound := enforceTargetBinding(result, &currentPath)
	if bound.Edit != nil || bound.Error != chatErrWrongTarget {
		t.Fatalf("an edit targeting a different path must be rejected: %+v", bound)
	}
}

func TestEnforceTargetBindingAcceptsMatchingTarget(t *testing.T) {
	result, ok := parseChatResult(
		`{"reply_text": "ok", "edit": {"path": "docs/architecture/architecture.yaml", "new_content": "version: 1\n"}}`,
	)
	if !ok {
		t.Fatalf("expected a well-formed parse")
	}

	currentPath := archYAMLPath

	bound := enforceTargetBinding(result, &currentPath)
	if bound.Edit == nil || bound.Error != "" {
		t.Fatalf("a matching-target edit must pass through unchanged: %+v", bound)
	}
}

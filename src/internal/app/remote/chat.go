package remote

// chat.go implements the workspace chat work item (plan Slice 5): a
// narrowly-scoped Claude turn — NOT the run/build machinery — that returns a
// structured result `{reply_text, edit: {path, new_content} | null}`. The
// edit is deliberately full-content replacement so it flows through the SAME
// doc_write YAML gate and revision mechanism the direct-edit path uses (S1,
// one persistence path for both). A deterministic driver (env
// TRUE_BDD_CHAT_DRIVER=deterministic) short-circuits the Claude turn with a
// scripted structured result so protocol tests exercise the identical
// buffer→doc_write path with no model call.

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/ondatra-ai/true-bdd/src/adapters/ai"
)

// chatMessage is one turn of the workspace-wide conversation (browser-owned
// history; the CLI is stateless across turns).
type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// chatPayload is the body of a `chat` work item (plan Slice 5): the full
// conversation plus the CURRENT page's backing file, when it has one.
type chatPayload struct {
	Conversation   []chatMessage `json:"conversation"`
	CurrentPath    *string       `json:"current_path"`
	CurrentContent *string       `json:"current_content"`
}

// chatEdit is a full-content replacement targeting exactly CurrentPath.
type chatEdit struct {
	Path       string `json:"path"`
	NewContent string `json:"new_content"`
}

// chatResult is the reply to a `chat` work item. Error is populated (never a
// blind write) when the structured result was malformed, targeted the wrong
// file, or the turn could not complete — Edit is nil in every such case.
type chatResult struct {
	ReplyText string    `json:"reply_text"`
	Edit      *chatEdit `json:"edit"`
	Error     string    `json:"error,omitempty"`
}

// Chat result error reasons (never surfaced as a distinct HTTP status — the
// browser always gets a 200 chatResult; a populated Error means "no edit").
const (
	chatErrMalformed   = "malformed_result"
	chatErrWrongTarget = "wrong_target"
	chatErrTimeout     = "timeout"
	chatErrUnavailable = "claude_unavailable"
)

const chatDriverEnv = "TRUE_BDD_CHAT_DRIVER"
const chatDriverDeterministic = "deterministic"

// chatRoleUser is the conversation role the browser tags the human's own
// messages with (vs. "assistant" for replies).
const chatRoleUser = "user"

// probeDirectiveParts is the exact `directive arg` split width for a `@probe
// <directive> <arg>` message.
const probeDirectiveParts = 2

// chatHandler runs one chat turn: the deterministic probe driver (protocol
// tests) or a real single-turn Claude call (a10), gated by TRUE_BDD_CHAT_DRIVER.
type chatHandler struct {
	deterministic bool
}

func newChatHandler(_ string) *chatHandler {
	return &chatHandler{deterministic: os.Getenv(chatDriverEnv) == chatDriverDeterministic}
}

// turn runs one chat turn and honours ctx cancellation/timeout (plan Slice 5).
func (h *chatHandler) turn(ctx context.Context, payload chatPayload) chatResult {
	if ctx.Err() != nil {
		return chatResult{Error: chatErrTimeout}
	}

	if h.deterministic {
		return deterministicChatTurn(payload)
	}

	return claudeChatTurn(ctx, payload)
}

// ── Deterministic driver (mirrors prompt-probe; NEVER active in production —
// gated behind TRUE_BDD_CHAT_DRIVER) ──

// deterministicChatTurn short-circuits the Claude turn with a scripted
// structured result for `@probe <directive> <arg>` messages (README-testids:
// add-term / add-scenario / append). On a non-file page (CurrentPath nil)
// EVERY directive replies with edit=null — never a write with no backing
// file.
func deterministicChatTurn(payload chatPayload) chatResult {
	last := lastUserMessage(payload.Conversation)

	if payload.CurrentPath == nil || payload.CurrentContent == nil {
		return chatResult{ReplyText: "Noted: " + last}
	}

	directive, arg, ok := parseProbeDirective(last)
	if !ok {
		return chatResult{ReplyText: "Noted: " + last}
	}

	newContent, applyErr := applyProbeDirective(directive, arg, *payload.CurrentContent)
	if applyErr != nil {
		return chatResult{ReplyText: fmt.Sprintf("could not apply %s: %v", directive, applyErr)}
	}

	return chatResult{
		ReplyText: fmt.Sprintf("applied %s %s", directive, arg),
		Edit:      &chatEdit{Path: *payload.CurrentPath, NewContent: newContent},
	}
}

func lastUserMessage(conversation []chatMessage) string {
	for i := len(conversation) - 1; i >= 0; i-- {
		if conversation[i].Role == chatRoleUser {
			return conversation[i].Content
		}
	}

	return ""
}

// parseProbeDirective parses `@probe <directive> <arg>`.
func parseProbeDirective(msg string) (string, string, bool) {
	const prefix = "@probe "

	trimmed := strings.TrimSpace(msg)
	if !strings.HasPrefix(trimmed, prefix) {
		return "", "", false
	}

	rest := strings.TrimSpace(trimmed[len(prefix):])

	parts := strings.SplitN(rest, " ", probeDirectiveParts)
	if len(parts) != probeDirectiveParts {
		return "", "", false
	}

	return parts[0], strings.TrimSpace(parts[1]), true
}

// applyProbeDirective performs a SCHEMA-AWARE edit (README-testids: "not just
// a comment append") so the live outline re-derives.
func applyProbeDirective(directive, arg, content string) (string, error) {
	switch directive {
	case "add-term":
		const anchor = "\nterms:\n"
		if !strings.Contains(content, anchor) {
			return "", errChatNoAnchor
		}

		insert := fmt.Sprintf("\nterms:\n  - name: %s\n    description: chat-added term\n", arg)

		return strings.Replace(content, anchor, insert, 1), nil
	case "add-scenario":
		const anchor = "\nscenarios:\n"
		if !strings.Contains(content, anchor) {
			return "", errChatNoAnchor
		}

		insert := fmt.Sprintf(
			"\nscenarios:\n  %s:\n    description: \"chat-added\"\n    service: \"mcp-service\"\n    user_stories: []\n",
			arg,
		)

		return strings.Replace(content, anchor, insert, 1), nil
	case "append":
		return content + "\n# " + arg + "\n", nil
	default:
		return "", errChatUnknownDirective
	}
}

// ── Real Claude turn (a10) ──

// chatSystemPrompt instructs Claude to reply with ONLY the structured JSON
// result the browser/CLI contract expects (plan Slice 5).
func chatSystemPrompt(payload chatPayload) string {
	var builder strings.Builder

	builder.WriteString("You are the TrueBDD workspace chat assistant, embedded in a web-based ")
	builder.WriteString("GitHub-style file view. Respond with ONLY a single JSON object — no ")
	builder.WriteString("markdown code fences, no prose outside the JSON — matching exactly:\n")
	builder.WriteString(`{"reply_text": string, "edit": {"path": string, "new_content": string} | null}` + "\n\n")
	builder.WriteString("Rules:\n")
	builder.WriteString("- reply_text is your natural-language reply to the user.\n")
	builder.WriteString("- If (and only if) the user asked for a change to the CURRENT file below, set edit ")
	builder.WriteString("to the COMPLETE new file content (never a diff/patch) via new_content, and edit.path ")
	builder.WriteString("MUST equal the current file path exactly.\n")
	builder.WriteString("- new_content MUST be byte-for-byte IDENTICAL to the current file content below, ")
	builder.WriteString("except for the MINIMAL insertion/change the user asked for. Copy every other line ")
	builder.WriteString("VERBATIM — same indentation width, same key order, same blank lines, same quoting ")
	builder.WriteString("style. Do NOT reformat, re-indent, reflow, or reorder anything else. When inserting a ")
	builder.WriteString("new list item or mapping entry, match the indentation of its SIBLING entries EXACTLY ")
	builder.WriteString("(count the leading spaces of a sibling line and reuse that exact count).\n")
	builder.WriteString("- new_content must parse as valid YAML — this is verified mechanically before it is ")
	builder.WriteString("committed, so an invalid result is silently discarded and the user's edit is lost. ")
	builder.WriteString("Re-check your indentation before answering.\n")
	builder.WriteString("- If there is no current file, or no file edit is needed, set edit to null.\n")
	builder.WriteString("- Output nothing but the JSON object — no other text.\n")

	if payload.CurrentPath != nil {
		builder.WriteString("\nCurrent file path: " + *payload.CurrentPath + "\n")
		builder.WriteString("Current file content:\n---\n")

		if payload.CurrentContent != nil {
			builder.WriteString(*payload.CurrentContent)
		}

		builder.WriteString("\n---\n")
	} else {
		builder.WriteString("\nThere is no current file open on this page — never emit an edit.\n")
	}

	return builder.String()
}

// claudeChatTurn runs ONE real Claude turn (a10), disallowing every
// file-mutating tool: the CLI's own doc_write is the ONLY persistence path
// (S1) — Claude must never touch the filesystem directly.
func claudeChatTurn(ctx context.Context, payload chatPayload) chatResult {
	client, err := ai.NewClaudeClient()
	if err != nil {
		return chatResult{Error: chatErrUnavailable}
	}

	mode := ai.ExecutionMode{
		DisallowedTools: []string{"Bash", "Write", "Edit", "MultiEdit", "NotebookEdit", "Agent", "Task"},
	}

	raw, execErr := client.ExecutePromptWithSystem(
		ctx, chatSystemPrompt(payload), lastUserMessage(payload.Conversation), "", mode,
	)
	if execErr != nil {
		if ctx.Err() != nil {
			return chatResult{Error: chatErrTimeout}
		}

		return chatResult{Error: chatErrUnavailable}
	}

	result, ok := parseChatResult(raw)
	if !ok {
		return chatResult{ReplyText: raw, Error: chatErrMalformed}
	}

	return enforceTargetBinding(result, payload.CurrentPath)
}

// enforceTargetBinding rejects an edit whose path does not equal the exact
// current_path (plan Slice 5: "the edit.path MUST equal current_path") —
// never a blind write to an unintended file.
func enforceTargetBinding(result chatResult, currentPath *string) chatResult {
	if result.Edit == nil {
		return result
	}

	if currentPath == nil || result.Edit.Path != *currentPath {
		return chatResult{ReplyText: result.ReplyText, Error: chatErrWrongTarget}
	}

	return result
}

// parseChatResult extracts and decodes the structured JSON object from a raw
// Claude reply (models sometimes wrap JSON in prose despite instructions).
func parseChatResult(raw string) (chatResult, bool) {
	jsonText := extractJSONObject(raw)
	if jsonText == "" {
		return chatResult{}, false
	}

	var parsed struct {
		ReplyText string `json:"reply_text"`
		Edit      *struct {
			Path       string `json:"path"`
			NewContent string `json:"new_content"`
		} `json:"edit"`
	}

	unmarshalErr := json.Unmarshal([]byte(jsonText), &parsed)
	if unmarshalErr != nil {
		return chatResult{}, false
	}

	result := chatResult{ReplyText: parsed.ReplyText}
	if parsed.Edit != nil {
		result.Edit = &chatEdit{Path: parsed.Edit.Path, NewContent: stripAccidentalDocumentMarkers(parsed.Edit.NewContent)}
	}

	return result, true
}

// leadingDocMarker / trailingDocMarker match a stray YAML document-separator
// (`---`) a model sometimes wraps its answer in despite being told "no code
// fences, no prose outside the JSON" — `---` is not a MARKDOWN fence, so a
// model instructed to avoid those sometimes reaches for YAML's OWN document
// marker instead (observed live in a10). A leading/trailing `---` turns
// single-document new_content into a two-document STREAM: Go's
// yaml.Unmarshal silently decodes only the first document (so the CLI's own
// gate would accept it), but the browser's strict single-document
// `YAML.parse()` throws "multiple documents" — so the client-side gate
// (P10b) would silently discard an otherwise-correct edit forever. Stripped
// unconditionally: the original file content never carries a `---`, so this
// can only ever remove the model's accidental fencing, never legitimate content.
var (
	leadingDocMarker  = regexp.MustCompile(`^---[ \t]*\r?\n`)
	trailingDocMarker = regexp.MustCompile(`\r?\n---[ \t]*\r?\n?$`)
)

func stripAccidentalDocumentMarkers(content string) string {
	content = leadingDocMarker.ReplaceAllString(content, "")
	content = trailingDocMarker.ReplaceAllString(content, "\n")

	return content
}

func extractJSONObject(text string) string {
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")

	if start == -1 || end == -1 || end < start {
		return ""
	}

	return text[start : end+1]
}

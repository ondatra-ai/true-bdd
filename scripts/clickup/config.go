package clickup

import (
	"os"
	"strconv"
	"time"
)

// Where the queue's artifacts land. Relative on purpose: every caller runs
// from the repository root, and an absolute path would follow a queue file
// copied to another checkout.
const (
	// TicketsMarkdown is the rendered queue — the artifact a person reads
	// before anything is uploaded.
	TicketsMarkdown = "tmp/merge/tickets.md"
	// FiledRecord is what came back from the filing turn, ticket by ticket.
	FiledRecord = "tmp/merge/filed.json"
	// CorpusDir holds one markdown file per existing ticket — what the
	// similarity turn reads, and the only ClickUp it is allowed to see.
	CorpusDir = "tmp/dupes/corpus"
	// DupesReport is the cluster report `clickup dupes` writes.
	DupesReport = "tmp/dupes/report.md"
)

// The MCP allowlists. Narrow by intent: `list` cannot create a task even if
// the model decides one is missing.
const (
	createTools = "mcp__claude_ai_ClickUP__createTask,mcp__claude_ai_ClickUP__addTagToTask," +
		"mcp__claude_ai_ClickUP__setCustomFieldValue"
	listTools = "mcp__claude_ai_ClickUP__listTasks,mcp__claude_ai_ClickUP__getTask"
	// No updateTask beyond the status field is possible here: the tool can
	// write a description, so the prompt forbids it and this line cannot.
	statusTools = "mcp__claude_ai_ClickUP__updateTask,mcp__claude_ai_ClickUP__addTaskComment"
	// The sweep rewrites a body and records why, so it is the one path needing
	// updateTask, setCustomFieldValue and addTaskComment together. No create,
	// no tags. getTask confirms its own write (observed 2026-08-29).
	triageTools = "mcp__claude_ai_ClickUP__updateTask," +
		"mcp__claude_ai_ClickUP__setCustomFieldValue," +
		"mcp__claude_ai_ClickUP__getTask," +
		"mcp__claude_ai_ClickUP__addTaskComment"
	// The similarity turn reads the dumped corpus and no ClickUp at all.
	// Plan mode refuses writes at the permission layer, so a judge handed
	// Read cannot become one that edits (scripts/triage/score.go:18).
	rankTools = "Read,Glob,Grep"
	rankMode  = "plan"
)

// attempts is the one retry a rejected ANSWER gets — never a failed turn,
// which would spend a second timeout to learn what the first one said.
const attempts = 2

const (
	defaultListID  = "901523097822"
	defaultTimeout = 900 * time.Second
	// roleClickUp labels this package's headless turns in the history.
	roleClickUp = "clickup"
)

// listID is the ClickUp list tickets are filed into.
func listID() string {
	if id := os.Getenv("CLICKUP_LIST_ID"); id != "" {
		return id
	}

	return defaultListID
}

// claudeTimeout bounds the filing turn.
func claudeTimeout() time.Duration {
	seconds, err := strconv.Atoi(os.Getenv("CLICKUP_CLAUDE_TIMEOUT"))
	if err != nil || seconds <= 0 {
		return defaultTimeout
	}

	return time.Duration(seconds) * time.Second
}

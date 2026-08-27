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
)

const (
	defaultListID  = "901523097822"
	defaultTimeout = 900 * time.Second
	// roleClickUp labels this package's headless turns in the history.
	roleClickUp = "clickup"
)

// Permissions for what this package writes: artifacts a person reads, in a
// directory a person browses.
const (
	dirMode  = 0o755
	fileMode = 0o600
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

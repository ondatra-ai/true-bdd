package codex

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/ondatra-ai/true-bdd/services/bdd-cli/internal/infrastructure/enginelog"
)

// WarnDenials reads explicit refusals from codex JSONL without treating ordinary failures as denials.
func WarnDenials(events, label, role string) {
	var denied []enginelog.ToolDenial

	for line := range strings.SplitSeq(events, "\n") {
		var event struct {
			Type string     `json:"type"`
			Item denialItem `json:"item"`
		}
		if json.Unmarshal([]byte(line), &event) != nil || event.Type != "item.completed" {
			continue
		}

		item := event.Item
		if item.Status != "declined" && (item.Status != "failed" || !permissionError(item.Output+" "+item.Error.Message)) {
			continue
		}

		denied = append(denied, item.denials()...)
	}

	enginelog.WarnDenials(label, role, denied)
}

type denialItem struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Command string `json:"command"`
	Output  string `json:"aggregated_output"`
	Changes []struct {
		Path string `json:"path"`
	} `json:"changes"`
	Tool      string `json:"tool"`
	Arguments struct {
		FilePath string `json:"file_path"`
		Path     string `json:"path"`
	} `json:"arguments"`
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (item denialItem) denials() []enginelog.ToolDenial {
	switch item.Type {
	case "command_execution":
		return []enginelog.ToolDenial{{Tool: item.Type, Target: commandTarget(item.Command)}}
	case "file_change":
		calls := make([]enginelog.ToolDenial, 0, len(item.Changes))
		for _, change := range item.Changes {
			calls = append(calls, enginelog.ToolDenial{Tool: item.Type, Target: change.Path})
		}

		if len(calls) == 0 {
			calls = append(calls, enginelog.ToolDenial{Tool: item.Type})
		}

		return calls
	case "mcp_tool_call":
		target := item.Arguments.FilePath
		if target == "" {
			target = item.Arguments.Path
		}

		return []enginelog.ToolDenial{{Tool: item.Tool, Target: target}}
	default:
		return nil
	}
}

func permissionError(message string) bool {
	message = strings.ToLower(message)
	for _, marker := range []string{
		"permission denied", "operation not permitted", "rejected by user", "rejected by policy",
		"exec command rejected", "outside of the project", "outside the allowed",
	} {
		if strings.Contains(message, marker) {
			return true
		}
	}

	return false
}

var commandTargetPattern = regexp.MustCompile(`["'](?:/|\.{1,2}/|~/)[^"']+["']|` +
	`(?:/|\.{1,2}/|~/|[[:alnum:]_*?.-]+/)[^\s"'<>;|&]+`)

// commandTarget keeps path-like command arguments, excluding the shell launcher.
func commandTarget(command string) string {
	matches := commandTargetPattern.FindAllString(command, -1)
	targets := make([]string, 0, len(matches))

	for _, match := range matches {
		target := strings.Trim(match, `"'`)
		switch target {
		case "/bin/sh", "/bin/bash", "/bin/zsh", "/usr/bin/bash", "/usr/bin/zsh":
			if strings.HasPrefix(strings.TrimSpace(command), match) {
				continue
			}
		}

		targets = append(targets, target)
	}

	return strings.Join(targets, " ")
}

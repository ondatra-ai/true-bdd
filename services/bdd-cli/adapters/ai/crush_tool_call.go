package ai

import (
	"path/filepath"
	"strings"
)

// CrushToolCall is the tool input the guard judges.
type CrushToolCall struct {
	Name       string
	TargetPath string
	Command    string
	WorkingDir string
}

func (p CrushGuardPolicy) allowsTestCommand(call CrushToolCall) (bool, string) {
	reason := "shell is denied in this mode; allowed forms: " + p.TestCommand + " OR " +
		p.TestCommand + " " + p.TestSelector + " '<selection>' in " + p.TestDir
	if p.TestCommand == "" {
		return false, "shell is denied in this mode; allowed forms: canonical test command OR " +
			"canonical test command selector '<selection>' (no test command granted)"
	}

	dir := call.WorkingDir
	if !filepath.IsAbs(dir) {
		dir = p.ReadRoot + string(filepath.Separator) + dir
	}

	if !filepath.IsAbs(p.TestDir) || resolveThroughSymlinks(dir) != resolveThroughSymlinks(p.TestDir) {
		return false, reason
	}

	if call.Command == p.TestCommand {
		return true, ""
	}

	prefix := p.TestCommand + " " + p.TestSelector + " '"
	if !strings.HasPrefix(call.Command, prefix) || !strings.HasSuffix(call.Command, "'") {
		return false, reason
	}

	selection := strings.TrimSuffix(strings.TrimPrefix(call.Command, prefix), "'")
	if selection == "" || strings.ContainsAny(selection, "'\n\x00") {
		return false, reason
	}

	return true, ""
}

package ai

import (
	"path/filepath"
	"slices"
	"strings"
)

// ReplayPolicy holds file roots and the exact command forms used by the writer guard.
type ReplayPolicy struct {
	ReadRoot     string   `json:"read_root"`
	WriteRoots   []string `json:"write_roots"`
	TestCommand  string   `json:"test_command,omitempty"`
	TestSelector string   `json:"test_selector,omitempty"`
	TestDir      string   `json:"test_dir,omitempty"`
}

// NewReplayPolicy resolves an execution mode's write roots and replay grant.
func NewReplayPolicy(mode ExecutionMode, workDir string) ReplayPolicy {
	globs := mode.WriteGlobs()
	roots := make([]string, 0, len(globs))

	for _, glob := range globs {
		root := absoluteGlobRoot(glob, workDir)
		if root != "" && !slices.Contains(roots, root) {
			roots = append(roots, root)
		}
	}

	policy := ReplayPolicy{ReadRoot: workDir, WriteRoots: roots}
	if mode.TestCommand != nil {
		policy.TestCommand = mode.TestCommand.Command
		policy.TestSelector = mode.TestCommand.Selector
		policy.TestDir = mode.TestCommand.Dir
	}

	return policy
}

func (p ReplayPolicy) allowsTestCommand(command, workingDir string) (bool, string) {
	reason := "shell is denied in this mode; allowed forms: " + p.TestCommand + " OR " +
		p.TestCommand + " " + p.TestSelector + " '<selection>' in " + p.TestDir
	if p.TestCommand == "" {
		return false, "shell is denied in this mode; allowed forms: canonical test command OR " +
			"canonical test command selector '<selection>' (no test command granted)"
	}

	dir := workingDir
	if !filepath.IsAbs(dir) {
		dir = p.ReadRoot + string(filepath.Separator) + dir
	}

	if !filepath.IsAbs(p.TestDir) || resolveThroughSymlinks(dir) != resolveThroughSymlinks(p.TestDir) {
		return false, reason
	}

	if command == p.TestCommand {
		return true, ""
	}

	prefix := p.TestCommand + " " + p.TestSelector + " '"
	if !strings.HasPrefix(command, prefix) || !strings.HasSuffix(command, "'") {
		return false, reason
	}

	selection := strings.TrimSuffix(strings.TrimPrefix(command, prefix), "'")
	if selection == "" || strings.ContainsAny(selection, "'\n\x00") {
		return false, reason
	}

	return true, ""
}

package state

import "path/filepath"

// LogKey names the Task's log file, relative to TaskLogDir.
const LogKey = "log"

// NoTaskLog collects what a scripts/ program writes with no Task open. Named
// rather than random because interleaving is already safe, so this is one file
// instead of one stranded per fresh Task.
const NoTaskLog = "no-task.json"

// TaskLogDir holds one log per Task. Gitignored with the rest of
// docs/history, so none of it ever reaches a commit.
func TaskLogDir(repo string) string {
	return filepath.Join(HistoryDir(repo), "task_logs")
}

// TaskLog is the Task's one log file, which every scripts/ program appends to.
// The name is RECORDED on first use rather than derived: the history hook
// installs its logger before state.Task has minted the stem.
func TaskLog(repo string) string {
	name := Get(repo, LogKey)
	if name != "" {
		return filepath.Join(TaskLogDir(repo), name)
	}

	task := Get(repo, TaskKey)
	if task == "" {
		// Deliberately NOT recorded: the hook mints the stem moments later,
		// and a persisted fallback would outlive that.
		return filepath.Join(TaskLogDir(repo), NoTaskLog)
	}

	name = task + ".json"

	// Ignored: this runs before logging is installed, so there is nowhere to
	// report to, and a path that could not be recorded is still a usable path.
	_ = Set(repo, LogKey, name)

	return filepath.Join(TaskLogDir(repo), name)
}

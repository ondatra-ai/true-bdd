// Package state is the Task's one state file: docs/history/state.jsonl.
//
// One record per line — {"k":…,"v":…} — appended, never rewritten, and folded
// on read so the last write to a key wins. That is what lets the five
// processes a commit run spawns (the session plus its nested `claude -p`
// workers) share it with no lock: a single write under PIPE_BUF lands whole,
// so no reader ever sees half a record and no writer loses another's.
//
// RECORDS MUST STAY UNDER 4 KB. Every value here is an id or a filename,
// around 100 bytes; the atomicity the whole design rests on is only promised
// below that ceiling, so never store a payload in this file.
//
// The keys:
//
//	task            the Task's stem — docs/history/<task>.md derives from it,
//	                so no path is stored
//	log             the Task's log file, under docs/history/task_logs/. RECORDED
//	                rather than derived: the history hook installs its logger
//	                before `task` exists, so the two are not always the same
//	                answer
//	ticket          the ClickUp Ticket this Task is working on
//	mandate         set while task-handle drives the run unattended
//	cursor:<8>      one session's progress through the current turn
//
// Delete is Set(key, "") — an empty value reads as absent. Init removes the
// file, which is how /task-start rolls a Task: everything stale goes at once,
// including the cursors nothing else prunes. It removes no log — dropping the
// `log` key rolls the log, and the old Task's records stay readable.
package state

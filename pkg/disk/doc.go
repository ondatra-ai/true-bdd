// Package disk is every file access this repository makes, and the only place
// that opens one.
//
// The discipline is a hold: a short advisory flock taken on the target's
// PARENT DIRECTORY, spanning the whole open-do-close, released before the
// function returns. The parent rather than the file itself because Write ends
// in a rename, and an flock on the replaced inode is invisible to the next
// writer, who opens the new one. A directory's inode is stable, nothing here
// renames one, and pkg/testkit/fstree records regular files only — so a
// hold can never appear in a BDD fixture's judged diff, which a lock sidecar
// beside the target would.
//
// Whole-file writes additionally commit through a same-directory temp and a
// rename, because the readers that matter most are not ours and never take the
// lock: the report server scanning a run directory, cat, jq, a CI artifact
// upload.
//
// Every function wraps its error and returns it. It never exits, never logs
// and never retries: the package that knows how to write a file does not know
// whether the program should die.
package disk

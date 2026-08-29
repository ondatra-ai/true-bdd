// Package report folds a Task's log back into the tree the run walked, and
// renders it: what every operation and sub-operation resulted in, how long
// each leaf took, and how long the whole command took end to end.
//
// STRUCTURE IS WRITTEN, NAMING IS DERIVED. A writer stamps two things and
// nothing more — `tree=start` / `tree=end` around an operation, and
// `duration_ms` on a leaf. The nesting, the 1.2a-style numbering, the
// indentation and the status column are all this package's job, computed from
// the order the records arrive in. That is why log/slog stays the logger with
// no wrapper: the only new call site is Open, which emits the two markers.
//
// Nothing is inferred from prose. The merge loop's lines are written for a
// human and change whenever the wording improves; deriving from them would
// make every rewording a silent data regression
// (docs/for_further/observability.md).
//
// THE KEYS ARE A CONTRACT, held by constants rather than by a linter.
// scripts/commit, scripts/merge, scripts/gates and scripts/internal/claudecli
// refer to the same names Fold reads, so a rename breaks the build instead of
// quietly emptying the report — the discipline pkg/enginelog uses for the
// engine's own log.
package report

// Package promptprobe backs the hidden `true-bdd prompt-probe` command
// (plan §4): a deterministic, non-Claude prompt driver that emits a
// choice → clarify → freetext sequence through the REAL answer collector and
// reads answers from stdin. It gives the protocol tests (P5/P10/P13/P14/P15/
// P16) a reliable interactive run to exercise dialogs, dispatch gating, the
// folder flock, and the answer path without spawning `claude`.
//
// The implementation lands here in the prompt-dialog phase. The tests-first
// suite pins the target contract via the in-test seam (nil hook ⇒ RED until
// wired), so `go build ./...` / `go vet ./...` pass today.
package promptprobe

// Package gates holds the quality pipeline as data: which checks exist, what
// each costs, and which changed paths make one necessary.
//
// Two consumers, and they want different things from it.
//
//	.claude/skills/pr-commit/gates.sh — runs them. Bare, it runs every gate;
//	    with --changed <base> it runs only those the diff needs, which is what
//	    handle-loop uses to spend ~2s on a documentation ticket instead of
//	    ~140s. Selection is LOCAL ONLY.
//	.github/workflows/ci.yml — runs every gate, always. Narrowing CI would
//	    buy nothing the loop cares about and would delete the backstop that
//	    makes local narrowing safe: whatever the selector skips, CI still runs.
//
// So the invariant the conformance test enforces is not "the same selector in
// both places" — it is "the same LIST in both places". A gate that exists on
// one machine only is not deterministic enforcement, and this pair has drifted
// before: gates.sh ran lint-comments.sh and CI did not.
//
// Fail-safe is not optional. A changed path matching no rule runs everything,
// so a directory nobody thought about cannot slip through unchecked.
package gates

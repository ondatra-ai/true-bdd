# `scen check` is advisory, and judges one scenario at a time

`scen check` walks every entry in the scenario registry against the `scen-check` checklist. Two properties of it are deliberate, both easy to mistake for oversights, and both cost real capability.

## It exits 0 even when checks fail

A failed walk prints `Validation failed.` and returns success, exactly like `us create`, `us refine` and `us apply`. Only `build tests` and `build code` exit non-zero on nonconvergence, by wrapping `runner.ErrExpectedNonconvergence`.

The consequence is that `scen check` cannot be a CI gate. That is intended: a full run over this repository's registry is 290 scenarios by 8 prompts — 2320 AI turns at the `high` tier — so gating a commit on it was never affordable. The command is a manual instrument for cleaning the registry, and the id filter (`scen check E2E-001 E2E-005`) is its practical entry point.

Fixtures pin `exits with code 0`. "Fixing" the exit code breaks them, which is the intended tripwire.

## A prompt sees one scenario, never the registry

Each cell embeds a single scenario's own fields — id, description, `service`, `path`, `user_stories`, `merged_steps` — and hands the model no path to the registry file. `us apply` does the opposite: its prompts read the scratch registry, because merging an acceptance criterion is inherently a question about the whole file.

The consequence is that no checklist prompt can ever ask a cross-registry question: duplicate behaviour, id collisions, "does the registry contain at least one happy path". This is why 4 of the 12 `us-refine` prompts the checklist was derived from were dropped rather than adapted — they threshold over a set (AC count 3–7, at least 1 happy path, at least 2 error scenarios, at least 80% convertible) and have no meaning applied to one scenario.

Cross-registry properties are cheap to decide in Go and expensive to decide in a model. When they are wanted, they belong in a deterministic pre-pass, not in a paid AI turn.

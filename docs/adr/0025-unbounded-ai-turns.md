# Let every AI turn finish without a router deadline

## Decision

The router adds no deadline to any AI turn, for every role and CLI. It passes the caller's context through the provider and wrapper to the subprocess. Every turn has a process group; cancelling the command stops that group and its descendants. Cancellation stays fatal, including for build code's writers, rather than becoming a recoverable started-writer outcome.

No request, prompt, checklist or template changes. No time, turn, token or cost budget is introduced. Recordings remain byte-identical. The existing fix-loop allowance, writer recovery and outcomes from ADR-0019, ADR-0021 and ADR-0022 stay unchanged. CLI request and tool timeouts, the drill's three-hour command limit and the separate Codex runner's limits are outside this decision.

This amends ADR-0019, ADR-0020 and ADR-0022: the former 20-minute router deadline and its Claude-writer-only exemption are removed. Both ordinary and writer turns share the same unbounded, cancellable lifetime.

## Evidence and alternatives

Issue #181 requires every AI turn to work unbounded. During generation 1 of the two-generation cut A test on 25 October 2026, from `9d701e6a`, the first repair left 180 of 252 tests passing after an 8.4-minute plan and 22.9-minute writer. The next attempt's writer hit model capacity; recovery reran the set, leaving 13 failures. The third plan, on claude-opus-5 with a 115,734-byte request, was killed at 15:31:07 by the router's 20-minute deadline. The fatal provider error stopped the walk after 79 minutes (#207).

Exempting only writers leaves the hardest planning work subject to the same failure. Raising the deadline substitutes another arbitrary cutoff for #181's rule. Passing the caller's context through every provider preserves deliberate command cancellation without imposing a turn duration.

## Verification

Router seam tests cover every role and CLI with ordinary and writer options, asserting no added deadline. Tests through the router and real wrappers cancel running stub CLIs with descendants holding output pipes; Claude, codex and the explicit writers stop promptly, preserve the cancellation cause and avoid recoverable writer outcomes. Full offline replay verifies request compatibility, and recording blobs are compared with the starting commit.

The Owner decided this in #207 under #181; no separate grilling entry records this lifetime decision. The parent drill's accepted replay and isolation limits are recorded in `docs/qna/20261022-201524-rebuild-from-spec-drill.yaml`, Q5.

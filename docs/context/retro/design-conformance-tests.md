# Retro — design-conformance-tests

First run under the redesigned workflow (task-blind test-fixer, fixed 3 fixer
rounds, tests-only plan, harness code consolidated under gitignored
`harness/src/`). Easy lane, escalated easy→hard after the reviewer's round 1.
Outcome: green — 5 design-gate specs pass (w8.1/8.2/8.3/8.4 + w9.1), live smoke
PASS, orchestrator-verified. Reconstructed from `active.json`, `phase-state.log`,
the plan Workflow log, and the Codex ledger.

## Run summary

| Phase | spawns/completions | tokens | duration |
|---|---|---|---|
| planner | 0 / 0 (no agent — correct for easy lane) | — | — |
| test_author | 1 / 1 | not captured¹ | 22m51s (09:34:55→09:57:46) |
| test_fixer | 1 / 1 | not captured¹ | 8m43s recorded (09:58:24→10:07:07) **+ ~25m orchestrator babysitting/verify** before the reviewer could spawn (→10:32:12) ≈ **34m wall** |
| reviewer | 1 / 1 | not captured¹ | 29m30s (10:32:12→11:01:42) |
| escalate easy→hard | — | — | logged 11:02:49 (post-reviewer) |

Run total ≈ **1h27m** (09:34:29→11:01:42, + retro). `turn.stop_blocks` 0,
`order_denies` 0 (this run), escalations 1. Codex rounds: author 1 (7 keep / 3
skip), fixer 3 (R1 1 keep · R2 3 keep incl. 1 reversed skip · R3 0 keep / dry
verify), reviewer 3 (R1 4 keep / 2 skip → escalate · R2 2 keep / 1 skip · R3 1
keep). Total keeps: 7 / 4 / 7.

¹ `skill-metrics.jsonl` schema carries `total_tokens` + `total_duration_ms` per
phase but both are hardwired `null` (verified on the prior `workspace-file-as-
source-ui` line). The retro cannot judge token cost because nothing captures it —
a standing metrics gap, not this run's fault. Durations are wall-clock from the
phase-state timestamps.

## What cost the most

1. **Test-fixer babysitting (~25m of dead orchestrator time).** The single
   largest avoidable cost. The fixer's recorded completion is 10:07:07, but its
   Bash activity in `block_test_edits.log` (session f7adaf7d) continues past it
   (10:07:47, 10:08:xx, 10:09:26 writing `codex-…-fixer-r1.md`), and the reviewer
   did not spawn until 10:32:12 — a 25-minute gap consumed by the orchestrator
   watching the process externally and SendMessage-resuming the fixer four times
   after it ended its turn mid-round. See Deviations.
2. **Reviewer escalation tripled its own cost** (1 floor round → 3). Warranted:
   every round kept real findings (R1 4, R2 2, R3 1), none dry until forced —
   escalation was signal, not waste (see Codex loop efficiency).
3. **Two full 39-spec suite re-runs inside the fixer phase** (post-R1 regression
   sweep + post-R2 confirm) plus the reviewer's final gate run. Each is a
   one-container-per-test Playwright pass; necessary for the isolation rule, but
   the dominant compute of the run.

## Deviations & violations

- **BIG DEFECT — test-fixer ended its turn 4× while background Codex/test runs
  were live.** Evidence: plan Workflow-log line 37 ("Fixer required 4 resume
  nudges (ended turns while background codex/test runs were live)"); the 25m
  fixer→reviewer gap; fixer Bash calls after the recorded subagent-stop. The
  agent's own instruction — "Monitor every Codex call with bounded `Monitor`
  until it exits; do not end the turn while one runs" — did **not** hold. Root
  cause is structural: `codex-loop.md` step 2 tells every agent to run Codex as a
  **background** task + arm a Monitor; the Monitor fires in a *future* turn, so
  the model retains a window to decide it is "done" and stop. Nothing at the
  enforcement layer catches a subagent premature-stop: `phase_state`'s
  `stop_blocks` counts only the **orchestrator's** own Stop-hook blocks (this run:
  0), so all four fixer stops were uncounted and invisible to the gate — the
  orchestrator had to notice them by hand. → Proposal 1.
- **Escalation handshake collapsed (minor).** The reviewer brief says: on
  multiple round-1 keeps, *notify the orchestrator → orchestrator escalates →
  grants the remaining cap*. In practice the reviewer self-continued to the full
  3 rounds and the `phase_state escalate` event landed at 11:02:49, **after** the
  reviewer completed at 11:01:42. It worked out (every extra round kept real
  findings), but the "notify-then-grant" round-trip was elided — the reviewer
  minted its own cap. Low severity; no fix proposed, noted for pattern-watch.
- **Tooling papercut — reviewer's first Codex launch wrote an empty prompt.**
  Ledger line 173: "first launch collided its `-o` path with the prompt file →
  empty prompt; re-launched clean." One wasted launch from a prompt-file /
  answer-file path collision. → Proposal 4.
- **No true violations.** `block_test_edits.log` for session f7adaf7d shows **zero**
  test-edit denies — the task-blind fixer never attempted the e2e specs (the two
  08-04 denies at 08:45/08:56 belong to an earlier aborted attempt, not this run).
  Planner correctly absent (easy lane). Escalation correctly audited. The prior
  abandoned run (phase-state.log lines 1-13: old planner/**coder**/reviewer naming,
  05:29 stop-gate blocks, 08:45 out-of-order agent-pre denies) was cleanly reset
  and re-run at 09:34 — the gates did their job during that reset.

## Codex loop efficiency

- **Author R1 — high signal.** 7 keeps / 3 skips of 10 findings, all hardening a
  *permanent* gate (visibility guards, input/textarea text-bearing, monospace
  fallback, schema minItems/maxItems + `auditVerdict` completeness, awaited
  attaches, per-test tmp paths). The 3 skips were principled (broad paint-property
  parsing = false-positive risk on a permanent gate; per-pixel proof out of R1
  scope; fail-loud on a broken judge). Genuinely red specs delivered.
- **Fixer 3 rounds — one churn, one mandated-dry.** R1 kept F1 (extend the CSS
  reset to form-control bg/border) and skipped F2 (canvas padding); R2 **reversed**
  the F2 skip (Codex's `SPEC.md §1 "40px"` + mockup `var(--space-5)` argument
  overturned it) and added F3/F4. The skip→reverse on F2 is one round of churn but
  cheap and correctly re-litigated. R3 was a pure verification pass — **0 keeps,
  clean bill** — the only zero-yield round in the run; it exists solely because of
  the fixed-3 mandate ("always, depends on nothing"). Policy-correct, but this is
  where the fixed-3 rule spends a round for confirmation, not contribution.
- **Reviewer 3 rounds — escalation earned it.** R1's 4 keeps (w8.3 chat-open
  sweep, `fonts.ready` await, exitCode assertion, WTID/README contract) tripped
  the multiple-keep valve; R2 added w8.4 (the *safe* dimension gate that overturned
  R-High-1's blanket skip) + w8.3 visibility gates; R3 added the border-**colour**
  assertion. No dry round — escalation bought 7 real keeps, zero prod edits, all
  strengthening the durable e2e spec. Efficient.

Net: 18 keeps across the run, near-zero waste. The only pure-confirmation round
(fixer R3) is mandated by design.

## Regeneratability

Standing lens: **the e2e suite is the spec; `harness/src` is a build artifact.**
This run makes that literal and testable — `.gitignore:28` `/harness/src/` means
**all** harness production code *and its unit tests* are gitignored generated code
(`git check-ignore` confirms both `harness/src/app/globals.css` and
`harness/src/tests/unit/breadcrumb.test.ts` are ignored). The **only** committed,
durable spec is `tests/harness/` (162 tracked files).

- **Did the fixer need knowledge the tests didn't carry?** No for the core work.
  The two task behaviors — the global Poppins/token CSS reset (demanded by w8.1/
  w8.2 red) and the persistent hairline breadcrumb bar (demanded by w9.1
  `persistent_frame` + `breadcrumb_hairline` red) — were each derived from a red
  e2e test. Task-blindness held: it read `routes.ts` and the app-router tree from
  the codebase, never the brief.
- **Did behavior ship that no durable test demands?** Yes — two pieces, both from
  the fixer's own Codex loop, both pinned **only** by gitignored unit tests:
  - **F3** — breadcrumb `NON_ROUTABLE_SEGMENTS` (the "stories" crumb renders as
    plain text, not a 404 `<Link>`). Ledger: "Not caught by w9 (rubric ignores
    label/content)."
  - **F4** — `safeDecode()` crash-safety on a malformed `%`-escape in the shell.
    Ledger: "not exercised by any current w1–w9 route (defensive fix)."
  On a fresh `rm -rf harness/src && re-run implement-task`, the task-blind fixer
  re-derives production from the red e2e specs alone; **no e2e assertion demands
  F3 or F4, and the unit tests that pin them are gitignored too** — so both would
  silently vanish (the regenerated shell would 404 the stories crumb and crash on
  a bad `%`). This is a real regeneratability leak, though a mild one: F3/F4 are
  hardening of code the fixer wrote *this session*, so a regeneration would
  re-harden whatever it re-builds — self-correcting-ish, but not guaranteed.
- **Would deleting `harness/src` reproduce the app?** For everything the task
  *specified* — yes: w8.1/8.2/8.3/8.4 + w9.1 durably pin the token conformance,
  the chat-open form controls, the 40px canvas padding, the breadcrumb hairline
  (width + style + `--border-hairline` colour), and the vision-judged three-region
  frame. The **reviewer already closed the CSS-reset half of the leak** by adding
  w8.3/w8.4 to pin the fixer's globals.css work into the durable suite. It did
  **not** close the F3/F4 (breadcrumb-behavior / crash-safety) half.

The structural gap: when the fixer's Codex loop surfaces a genuine bug in code it
just wrote, the only pin it can create is a gitignored unit test (it is
hard-blocked from e2e tests and task-blind), and it has no channel to *request*
durable e2e coverage. The reviewer — who *can* edit e2e tests — is the natural
place to catch these, but nothing in its brief tells it to. Proposals 2 (reviewer
audits) and 3 (fixer surfaces) close that handoff; they rank above the cost fixes.

## Proposals

### Proposal 1 — Run the test-fixer's Codex calls in the FOREGROUND (kill the premature-stop window)

**Problem.** The run's biggest defect. The fixer ended its turn 4× mid-round
(plan line 37; 25m fixer→reviewer gap; Bash activity past the recorded
subagent-stop), forcing manual SendMessage resumes. The background-Codex +
future-turn-Monitor pattern (`codex-loop.md` step 2) inherently leaves a window
where the model stops while the Monitor is still pending, and `phase_state`
doesn't count/gate subagent stops (`stop_blocks` 0 this run). A blocking
foreground Bash call *cannot return* until Codex exits, physically removing the
window — the strongest, self-contained fix (no new hook script).

```diff
--- a/.claude/skills/implement-task/references/codex-loop.md
+++ b/.claude/skills/implement-task/references/codex-loop.md
@@
 2. **Run the Codex wrapper** (path + usage in paths.yaml) as a **background** task and
    arm a Monitor that fires on exit. **Always read-only** — Codex suggests; the agent
    applies the keeps and runs the tests itself. Never let Codex edit files directly
    (that would bypass the scoring gate). The answer lands in the artifacts dir.
+   **Test-fixer exception — run Codex in the FOREGROUND.** The test-fixer invokes the
+   wrapper as a single **blocking** Bash call (no `run_in_background`, a generous
+   `timeout`), never background+Monitor. A blocking call cannot return until Codex
+   exits, so the turn physically cannot end mid-round — closing the premature-stop
+   window that forced four manual resume nudges in the design-conformance-tests run.
+   Wait out background test runs the same way before yielding.
    **Status line (one, when you launch each round):** name the round, the cap, and the
```

```diff
--- a/.claude/agents/implement-task-test-fixer.md
+++ b/.claude/agents/implement-task-test-fixer.md
@@ ## Status
 
 Print start, baseline confirmed, each iteration's counts, each Codex round N/3, and
-any escalation. Monitor every Codex call with bounded `Monitor` until it exits; do
-not end the turn while one runs.
+any escalation. Run every Codex call as a single **foreground/blocking** Bash
+invocation (no backgrounding), and wait out every background test run, before you
+continue. **Never end your turn with a Codex round or a test run still in flight** —
+if one is running you are not done; there is no valid reason to yield mid-round.
```

### Proposal 2 — Reviewer runs a regeneratability audit that pins fixer-added behavior into the durable e2e spec (ranked #1)

**Problem.** `harness/src` (production + unit tests) is gitignored generated code;
only `tests/harness/` survives a commit. The fixer's Codex loop shipped F3
(non-routable "stories" crumb) and F4 (`safeDecode` crash-safety), each pinned
**only** by a gitignored unit test (ledger: "Not caught by w9" / "not exercised by
any current w1–w9 route"). On regenerate-from-tests both vanish. The reviewer
already pins the CSS-reset half (w8.3/w8.4) but has no instruction to hunt the
unit-only behaviors, so F3/F4 stayed durably unspecified.

```diff
--- a/.claude/agents/implement-task-reviewer.md
+++ b/.claude/agents/implement-task-reviewer.md
@@
 1. Run the lane-capped, read-only Codex loop from paths.yaml. Each round sends the
    full task, diff, plan, challenges, and all prior findings + their dispositions. Ask about coverage,
    missing or weak assertions, flakiness, whether each test fails when behavior is
    broken, and code correctness/quality. Codex only finds and never edits. You score
    every finding; keep only composite ≥7 with all four gates satisfied. Apply kept
    changes yourself to e2e tests and production code.
+   **Regeneratability audit (durable-spec check).** The e2e suite (paths.yaml →
+   e2e_tests) is the ONLY committed spec; `harness_code_root` — production code AND
+   its unit tests — is gitignored, regenerated-from-tests code. Enumerate every
+   production behavior in the diff that is pinned ONLY by a unit test (nothing in the
+   e2e suite asserts it): on a fresh regenerate-from-tests it silently vanishes. For
+   each, either add an e2e assertion (you may edit e2e tests) or record it in
+   residual risk as accepted regeneration-loss — never leave it unstated.
 2. Always attempt each applicable live surface:
```

### Proposal 3 — Test-fixer surfaces the production behaviors no red e2e test demands (feeds Proposal 2)

**Problem.** Same evidence. The fixer is task-blind and hard-blocked from e2e
tests, so when its Codex loop surfaces a genuine bug in code it just wrote (F3/F4),
the only pin it can create is a gitignored unit test, and it has no channel to flag
that a *durable* test is missing. Making it name these gaps in its return gives the
orchestrator/reviewer the list to act on.

```diff
--- a/.claude/agents/implement-task-test-fixer.md
+++ b/.claude/agents/implement-task-test-fixer.md
@@ ## Output
 
 Return at most 30 lines: final passing/total result with the literal pass/fail tail,
 changed production/unit-test paths, and kept/skipped counts per Codex round (3
-rounds, always). If stopped, include the test, reason, Codex verdict, and attempts.
-Otherwise state plainly that all tests pass.
+rounds, always). **Also list any production behavior you added that the red e2e/BDD
+specs do NOT assert — pinned only by your (gitignored) unit tests** — so the
+orchestrator can route it to the reviewer for durable e2e coverage; write "none" if
+every added behavior is demanded by a driving spec. If stopped, include the test,
+reason, Codex verdict, and attempts. Otherwise state plainly that all tests pass.
```

### Proposal 4 — Guard the Codex prompt-file / `-o` answer-file path collision

**Problem.** The reviewer's first Codex round wrote an empty prompt because its
`-o` answer path collided with the prompt-file path (ledger line 173), wasting a
launch and needing a clean re-launch. A one-line naming rule prevents it for all
agents.

```diff
--- a/.claude/skills/implement-task/references/codex-loop.md
+++ b/.claude/skills/implement-task/references/codex-loop.md
@@
 1. **Write the prompt** — full task + all current changes + all prior-round findings
    + their dispositions (the four ingredients above) — to a prompt file under the
-   Codex artifacts directory (paths.yaml). Ask for the three jobs in order (verify
+   Codex artifacts directory (paths.yaml). Give the prompt file and the wrapper's
+   `-o` answer file DISTINCT paths — a shared path overwrites the prompt with an
+   empty file and wastes the launch (this bit the reviewer's first round this run).
+   Ask for the three jobs in order (verify
    applications / challenge skips / fresh findings), tell Codex to run commands to
    verify its claims, and to return findings only (no scores).
```

_Not proposed but noted:_ `total_tokens`/`total_duration_ms` are `null` in the
metrics schema, so no retro can judge token cost — a phase_state/hook capture gap
(out of this analyst's edit scope; flagged for the user).

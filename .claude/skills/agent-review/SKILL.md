---
name: agent-review
description: >-
  Audit a Claude Code agent (.claude/agents/*.md) together with its full
  dependency web — map fan-in/fan-out across the referenced reference,
  template, and prompt files, score how clearly each file's architectural role
  is defined, and surface instruction-level conflicts, role mismatches, and
  DRY/KISS violations — then, once approved, apply the fixes. Use this whenever
  the user wants to review, audit, critique, sanity-check, clean up, simplify,
  or refactor an agent definition or the instruction files it leans on — e.g.
  "review my deploy agent", "audit the research agent", "are this agent's
  instructions consistent?", "find conflicts in my agent", "is this agent
  over-specified?", "clean up the agent's references". Trigger even when the
  user just names an agent and asks "what's wrong with this?" or "can this be
  simplified?" without saying the word "review".
---

# Agent Review

Reviewing a Claude Code agent is not reviewing one file. An agent
(`.claude/agents/<name>.md`) is the visible tip of a **web of instruction
files** — reference docs, prompt templates, path registries — that together
decide how the agent behaves. A conflict two files down the graph breaks the
agent just as surely as a bug in its own frontmatter. So this skill audits the
whole reachable web, not just the entry file.

## The mental model: an instruction corpus is a dependency pyramid

Every file in the web has two numbers that matter:

- **fan-in** — how many other files depend on it (reference it).
- **fan-out** — how many other files it depends on.

A healthy corpus is a pyramid. Broadly-depended-on files (**high fan-in**) sit
at the base and should state **general, stable, role-defining truths** — e.g. a
shared reference that itself depends on almost nothing yet is cited by nearly
every other file, so its one job is a single generic contract every caller
assumes. Files that depend on many others (**high fan-out**) sit at the top:
they are orchestrators and consumers, and should carry **specific, narrow
logic** that composes the base.

The whole point of the audit is to find **mismatches between a file's role and
its position in the pyramid**, because those are where instructions rot:

- A **high fan-in file with a narrow/volatile role** — everyone depends on it,
  but it says something niche or churny. Every change there ripples; callers
  start working around it.
- A **high fan-out (leaf) file stating generic truths** — a truth that belongs
  once, at the base, gets restated in a consumer. That is duplication waiting
  to drift out of sync.

Hold this model the entire way through. Roles are judged against it; the
clarity score measures it; the fix plan restores it.

## Before you start — know the target

You audit **one agent** per run: a file under `.claude/agents/*.md`. If the
user hasn't named one, ask exactly: **"What agent to analyse?"** and wait.
(Only `.claude/agents/*.md` files are valid roots. If the user points at a
skill, hook, or command, say so and ask them to name an agent instead.)

---

The procedure has five phases and **two hard approval gates**. Do not cross a
gate without an explicit "yes" from the user — everything downstream is built
on what the gate locks in, and the final phase edits the very files that govern
other agents.

## Phase A — Map the dependency web (spawn a subagent)

Spawn one subagent to build the graph. Fan this out rather than doing it inline
for two reasons: the web can span a dozen files and thousands of lines — more
than you want to hold while also reasoning about roles — and a subagent reports
back a clean structure instead of a pile of file dumps.

Give the subagent the root agent path and this task:

1. **Follow every edge out of the root**, transitively, until the web closes.
   An agent references its dependencies **two ways**, and you must resolve
   both:
   - **Literal paths** — a relative or absolute path to another file.
   - **Indirect references** — many setups don't hardcode paths; an agent names
     a key, alias, or convention that resolves to a file through a central
     registry (a paths/config file the agent reads), an environment value, or a
     naming rule. Before walking, **detect whether this project uses such
     indirection** — look for a registry or config file the agent reads up
     front — and resolve those references too. A graph walk that only greps for
     slashes will miss every indirect edge.
2. **Compute reverse edges (fan-in)** for each file discovered: search the
   agents directory and wherever the instruction files live for references to
   it — by literal path **and** by whatever key/alias resolves to it.
3. Return, for every file in the web: its path, its **fan-out** (deps it
   references), its **fan-in** (files that reference it), and a one-line note on
   what it appears to contain.

## Phase B — Assign roles, score clarity → table → GATE 1

Using the graph from Phase A, for **each file** determine:

- **Role** — one sentence naming its single architectural job, phrased against
  the pyramid. The more files depend on it, the more generic and stable its
  role should read; the more it depends on others, the more specific.
- **Clarity score (1–10)** — reading *only this file*, how unambiguously does
  its single role come through, and does that role match its graph position?
  - **8–10** — one obvious role in a sentence; matches its fan-in/fan-out.
  - **4–7** — role inferable but muddied: multiple responsibilities crammed
    together, or a stated role that drifts as the file goes on.
  - **1–3** — no discernible single role, or the role **contradicts** its
    position (e.g. high fan-in but the content is narrow and volatile).
- **Flag** any file scoring **< 7** (mark it — e.g. a leading ⚠️) so low-clarity
  files stand out in the table.

Print the table:

| File | Fan-out (deps) | Fan-in (dependents) | Role | Clarity | Notes |
|------|----------------|---------------------|------|---------|-------|

**GATE 1 — stop and get approval.** Ask the user to validate the roles and
scores, and correct any that are wrong. **Do not proceed until they explicitly
approve.** This gate exists because every finding in the next phase is judged
*relative to a file's role* — if a role is wrong here, every conflict you'd
report against it is wrong too. Lock the roles before reviewing instructions.

## Phase C — Per-file instruction review (fan out, one subagent per file)

Now spawn **one subagent per file, in parallel**. Each gets its file plus the
**approved role** for that file (from Gate 1). A fresh subagent reviewing a
single file in isolation gives an independent read that isn't anchored by
everything you've already seen. Each subagent reports three lists:

1. **Instructions that conflict with the file's role** — anything the file
   tells the reader to do that doesn't belong to its one job, plus how to fix
   it (move it to the file whose role owns it, or cut it).
2. **Instructions that conflict with each other** — two directives in the same
   file that can't both be followed, plus the resolution.
3. **Simplification / duplication / DRY & KISS violations** — restated truths,
   dead qualifiers, over-specification, pure verbosity that changes nothing if
   removed.
4. **Phantom coupling / needless cross-reference** — a phrase that makes this
   file mention or point at *another* file (or an unrelated concept) it has no
   functional reason to know about. Judge it by **two tests, both of which must
   fail**:
   - **Removal test** — cut the phrase. Does any logic, behavior, or meaning
     change? If nothing changes, it's a candidate.
   - **Coupling test** — does the phrase exist *only* to reference something
     this file doesn't functionally need? (A description of tool X that name-
     drops tool Y "for parallel," a note that points at a sibling file purely to
     say they're related.)

   A phrase that fails **both** is not mere wordiness (that's #3) — it is a
   dependency edge carrying zero load. Cutting it doesn't just shorten the file;
   it **deletes a false edge from the graph** and decouples two files that were
   never functionally related. This is the finding this skill is uniquely
   positioned to make: the instruction audit and the fan-in/fan-out graph meet
   exactly here, because a narration-only cross-reference is a phantom edge
   inflating coupling for no benefit.

**Every finding quotes the exact offending text and shows the concrete fix** —
so the user can judge the call, not trust a vague "this is unclear." Format
(here, a phantom-coupling finding):

> **path:** `references/tool-x.md`
> **finding (phantom coupling):** the description reads
> *"Tool-X mechanics + gotchas (the Tool-X parallel to tool-y-mechanics)"*.
> **Removal test:** cut *"(the Tool-X parallel to tool-y-mechanics)"* and the
> entry means exactly the same thing. **Coupling test:** the clause exists only
> to point `tool-x.md` at `tool-y-mechanics`, which `tool-x.md` has no
> functional need to know about. It fails both — a narration-only edge.
> **fix:** drop the parenthetical → *"Tool-X mechanics + gotchas."* This also
> removes a phantom `tool-x → tool-y` edge from the dependency graph.

**Simplify with care — some repetition is load-bearing.** In instruction files,
a rule restated at the point it matters, or emphasis on an easy-to-miss
guardrail, is often deliberate, not redundant. Before flagging wording as
cuttable, ask whether removing it would let a reader make a mistake the wording
was preventing. If yes, it stays. The goal is a clearer agent, not a shorter
one.

Collect every subagent's findings and print an accumulated table:

| File | Type | Offending text (quoted) | Suggested fix |
|------|------|--------------------------|---------------|

## Phase D — Whole-corpus review (spawn a subagent)

Per-file review can't see across files. Spawn one more subagent with **all the
files at once** and ask it for what only a corpus-wide view reveals:

- **Instructions that overlap in meaning across files** — match by **meaning,
  not wording**. Verbatim copies are the easy case; the dangerous one is two
  files saying *almost* the same thing — a paraphrase, a partial restatement.
  That drifts over time, and worse, near-duplicates often **already disagree**
  in the detail where they differ. Name the single file that (by its role)
  should own the instruction; the others defer to it or drop it.
- Instructions in **different files that contradict** outright — two directives
  that can't both be followed.
- Instructions that are **unnecessary given the whole** — covered elsewhere, or
  no longer load-bearing.

Add these to the findings.

## Phase E — Fix plan → GATE 2 → implement or stop

Spawn a subagent (or do it inline if the findings are few) to turn the
confirmed findings into an ordered **fix plan**: grouped by file, each item
naming the change and the finding it resolves, sequenced so base-of-pyramid
files are corrected before the consumers that depend on them.

Present the plan. **GATE 2 — stop and get explicit approval.** This gate is
non-negotiable because the next step edits the files that define how other
agents behave — high blast radius.

- **If the user approves** — apply the fixes by editing the files, one grouped
  change at a time, exactly as the approved plan describes. Nothing beyond the
  plan.
- **If the user declines or wants changes** — stop. The report and tables stand
  on their own as the deliverable; do not touch the files.

---

## Notes

- **Two gates, always.** Roles before instruction review (Gate 1), plan before
  editing (Gate 2). Skipping either produces confident findings built on an
  unvalidated foundation, or edits the user never signed off on.
- **Findings must be quotable and checkable.** A finding the user can't verify
  against the exact text is noise. Quote, then fix.
- **Subagent boundaries are deliberate** — graph (A), per-file review (C),
  corpus review (D) each want either scale or an independent read. The
  orchestrator (you) owns the tables, the gates, and the final edits.

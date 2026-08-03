---
name: identify-task
description: First half of the task workflow (identify-task then implement-task) — define a task as Goal + Requirements (behavioral should/must statements from a user or system perspective; NO implementation), then kill ambiguity by asking the user ONE question at a time until no two distinct user-perspective interpretations remain, using Codex to surface missed risks. Takes the task idea from the skill argument — the text after /identify-task (e.g. `/identify-task make true-bdd connect to the harness host on Vercel`); if none is given, asks for it as the very first question. Use when scoping/defining a substantial change before implementation ("define/spec/scope this task", "what should this do") — this project requires the Codex-involved, test-first task workflow for substantial changes, so scope them here first. Also handles Prototype mode — when the user says "let's prototype X" / "prototype this", requirements are discovered by building a throwaway Sonnet-agent prototype under live user steering, then reverting and distilling the dialog into the brief. Produces docs/tasks/<slug>.md (one brief per task, slug from the goal); hand the slug to implement-task once the goal is locked.
---

# Identify task

Define the task as **Goal + Requirements** — behavior, not implementation. Then remove every ambiguity by questioning the user. Output: `docs/tasks/<slug>.md` (a kebab-case slug of the goal).

**If the task idea asks to prototype** ("let's prototype X", "prototype this") —
enter **Prototype mode** (section below) instead of the normal process order:
discover the requirements by building, not by questioning.

## The brief format

- **Goal** — the outcome in 1–2 sentences. What changes for whom. Not how.
- **Requirements** — behavioral `should`/`must` **capabilities**, never process/doc chores or "how". **For how to phrase and file them — Product vs System, allowed subjects, capability phrasing, tagging, non-goals — read and follow `docs/context/requirements-guide.md`.** **Draft a complete list — both what the user revealed AND what you suggest** (your suggestions are valuable: infer likely requirements from the goal, the code, and Codex); tag each `[revealed]` (from the user) or `[suggested]` (your inference). Then **validate them with Keep / Drop / Other** (step 8) — keep only the ones the user Keeps; never write an unvalidated requirement.
- **Established facts** (optional appendix) — short, machine-oriented lines (path + one-line fact) recording what your exploration already verified: fixture shapes, contract files, run commands, structures. The implement-task planner is told to TRUST these instead of re-deriving them — write down exactly what you'd hate to re-discover.

## The process

**First, get the task idea** — it comes from the skill argument, the text after
`/identify-task` (e.g. `/identify-task make true-bdd connect to the harness
host on Vercel`). If none was given, your very first action is to ask the user
one question — "What's the task?" — and wait. Don't analyze or plan without it.

1. **Analyze** the task + relevant files. Read `docs/context/requirements.md`, `docs/context/requirements-guide.md`, `docs/context/terms.md` (Harness/System/Product) and `docs/context/task_template.md`.
2. **Explore current behaviour** - read the code, the docs, use playwright, cli tools, mcp servers to explore the current behaviour, continuous integration, structure, architecture and system design. Dig deep. Identify the current behaviour and identify the goal. **Exploration cost discipline:** in the browser, look at screenshots first and take a full `browser_snapshot` only when you must read specific element state — and then targeted/limited, or saved to a file and grepped; read an image only when a decision depends on seeing it. Record what you verify as Established facts (brief appendix) so later phases don't re-derive it.
3. **Draft a full list of requirements** — both what the user has revealed AND what you suggest from the goal, the code, and Codex. Tag each **[revealed]** (from the user) or **[suggested]** (your inference).
4. **Ambiguity test — the loop's engine.** Can you construct **two distinct, valid user-visible outcomes** that both satisfy the current requirements?
   - **Yes → it's ambiguous.** Form the sharpest disambiguating question and **ask the user ONE question** (never a batch). Update the requirements from the answer. Repeat.
   - **No → it's unambiguous.** Stop asking.
5. **If the user flags a wrong direction, shrink the task's purpose** — narrow scope, re-confirm the goal. Pushback means reconsider, not defend.
6. Save the task to `docs/tasks/<slug>.md` (a kebab-case slug of the goal; create the folder if needed; update in place if it exists).
7. **Ask codex to validate your requirements, find gaps and risks in your task** — Ask codex to read  `docs/context/requirements.md` and `docs/context/terms.md`, and current task you saved. Ask codex to analyse the requirements and find any gaps, risks, or ambiguities. Analyse the response and incorporate only relevant suggestions. Repeat until no relevant suggestions found or 3 times.
8. **Validate requirements with Keep / Drop / Other, in batches.** Present requirements **in full** (the complete sentence, not a compressed label), each naming whether it is `[revealed]` (from the user) or `[suggested]` (your inference). Use **one `AskUserQuestion` call per batch of 3–4 requirements** — one question per requirement inside the call (the tool takes up to 4 questions), each with two options — **Keep** (accept into the brief) and **Drop** (remove it); the harness auto-adds **Other**, which the user picks to clarify, reword, swap the subject, or push back. **Format each question as the full requirement text, then TWO line breaks, then `Keep or drop? (Pick Other to reword or clarify.)`** — optionally followed by a short, requirement-specific example of what Other could change. **From the second batch on, add one final question offering "Keep all remaining"** (list the remaining requirements in the question text so the user sees exactly what they're accepting); if chosen, mark the rest Kept and stop the sweep. Your suggestions are valuable — do propose them — but keep only the requirements the user **Keeps**; never write a requirement the user did not Keep. Apply any rewording from "Other" and re-confirm. Repeat until every requirement is Kept or Dropped. **Batching applies ONLY to this Keep/Drop sweep — the ambiguity-killing loop (step 4) stays strictly ONE question at a time.**
9. **Exit** - show summary of each step from this skill, especially codex validation and user validation, and the final task brief. Ask the user if they want to save it to `docs/tasks/<slug>.md` (yes/no). If yes, save it; if no, discard it.

## Prototype mode

The prototype is **throwaway**; the brief is the product. The user steers; you
orchestrate: clarify only what's ambiguous, relay everything else untouched.

1. **Baseline.** Save `git status --porcelain` to `./tmp/proto-baseline.txt` and
   note the current branch. Step 5's revert restores exactly this state — warn the
   user once if the tree is already dirty.
2. **Build immediately.** No exploration, no Codex, no upfront questions. Spawn
   the `implement-task-prototyper` agent (Sonnet — deliberate: disposable code, speed over
   quality) with the idea verbatim.
3. **Steering loop.** For each user message:
   - unambiguous → relay it verbatim to the SAME agent via `SendMessage` (never a
     fresh spawn — its context IS the prototype);
   - two valid readings → ask ONE clarifying question first (step 4's ambiguity
     rules), then relay.
   After each iteration surface: what changed + the one-line "how to try it".
   Keep a running **steering log** — every steering and reaction is a future
   requirement.
   **Validate each iteration — two tiers, both mandatory:**
   - *Tier 1 — the prototyper self-validates* (its agent definition requires
     it): exercises what it changed and returns VERIFIED/UNVERIFIED evidence —
     for UI, before/after screenshot pairs around each interaction (headless
     Playwright via Bash); for non-UI, real command output.
   - *Tier 2 — you review, cheaply.* Read the evidence screenshots/output and
     check they show what the report claims. Then re-drive exactly ONE
     interaction yourself (Playwright MCP / CLI) — the riskiest one: the
     behavior whose failure would most embarrass the demo. Escalate to full
     re-verification ONLY if evidence is missing, contradicts the report, or
     your spot-check fails. Screenshots prove looks, not behavior — never
     accept a "looks right" screenshot as proof that a click works, and never
     surface an unverified claim to the user as working.
4. **End** when the user says so ("done", "enough", "make it a task").
5. **Distill, then ask about the prototype's fate.** From the steering log +
   final prototype state, draft the requirements — steerings and reactions are
   `[revealed]`, your inferences `[suggested]`; record what the prototype proved
   as Established facts. THEN ask the user ONE `AskUserQuestion` — "Revert the
   prototype?" — with options: **Revert** (restore the step 1 baseline:
   `git restore` changed tracked files, delete untracked files created since —
   nothing survives except the brief) and **Keep** (preserve it: copy the
   prototype into a tracked folder, e.g. under `harness/design/`, minus
   node_modules/build artifacts, with a README naming the brief; leave the
   working tree uncommitted — never commit without an explicit instruction).
   The harness adds Other for anything in between. Honor the choice; revert
   exclusions noted in `./tmp/proto-baseline.txt` survive either way.
6. **Rejoin the normal process** at step 6 (save the brief), then 7 (Codex
   validation) and 8 (Keep/Drop) — the user now knows what they want, so
   validation is fast.

Run Codex non-interactively — without a sandbox flag it hangs:
```bash
codex exec -s read-only --ephemeral -C "$PWD" --color never \
  -c model_reasoning_effort=low -o ./tmp/codex-review.md - < ./tmp/codex-prompt.md
```
Background it; full guide + wrapper: paths in `docs/context/paths.md` → Codex.

---
name: identify-task
description: First half of the codex-task workflow — define a task as Goal + Requirements (behavioral should/must statements from a user or system perspective; NO implementation), then kill ambiguity by asking the user ONE question at a time until no two distinct user-perspective interpretations remain, using Codex to surface missed risks. Takes the task idea from the skill argument — the text after /identify-task (e.g. `/identify-task make true-bdd connect to the harness host on Vercel`); if none is given, asks for it as the very first question. Use when scoping/defining a substantial change before implementation ("define/spec/scope this task", "what should this do"), or when the codex-task orchestrator calls it. Produces docs/tasks/<slug>.md (one brief per task, slug from the goal).
---

# Identify task

Define the task as **Goal + Requirements** — behavior, not implementation. Then remove every ambiguity by questioning the user. Output: `docs/tasks/<slug>.md` (a kebab-case slug of the goal).

## The brief format

- **Goal** — the outcome in 1–2 sentences. What changes for whom. Not how.
- **Requirements** — each is a `should`/`must` statement whose SUBJECT is an exact term from `docs/context/terms.md` (a Role, a System, or the Harness), observable and implementation-free — e.g. "A Developer should be able to …", "The true-bdd CLI must …", "The true-bdd harness should …". No bare "System"/"user", no tech choices, no file/function names, no "how." State a **non-goal** only when it prevents a double-interpretation.

## The process

**First, get the task idea** — it comes from the skill argument, the text after
`/identify-task` (e.g. `/identify-task make true-bdd connect to the harness
host on Vercel`). If none was given, your very first action is to ask the user
one question — "What's the task?" — and wait. Don't analyze or plan without it.

1. **Analyze** the task + relevant files. Read `docs/context/requirements.md` (Harness/System/Product) for standing requirements not in code.
2. **Draft** an initial Goal + Requirements.
3. **Ambiguity test — the loop's engine.** Can you construct **two distinct, valid user-visible outcomes** that both satisfy the current requirements?
   - **Yes → it's ambiguous.** Form the sharpest disambiguating question and **ask the user ONE question** (never a batch). Update the requirements from the answer. Repeat.
   - **No → it's unambiguous.** Stop asking.
4. **Use Codex (≤3 rounds)** to surface risks/ambiguities you missed: give it the goal + requirements, repo, `CLAUDE.md`, `docs/`, read + command access; ask what's underspecified or risky. Feed only the relevant gaps back into step 3 as questions.
5. **If the user flags a wrong direction, shrink the task's purpose** — narrow scope, re-confirm the goal. Pushback means reconsider, not defend.
6. **Exit** when the two-interpretations test passes (and Codex has nothing relevant left). Write the brief to `docs/tasks/<slug>.md`, where `<slug>` is a short kebab-case slug derived from the goal (e.g. goal "Make true-bdd connect to the harness host on Vercel" → `docs/tasks/make-true-bdd-connect-to-the-harness-host-on-vercel.md`). Create the folder if needed; if that file already exists from a prior run of the same task, update it in place.

Run Codex non-interactively — without a sandbox flag it hangs:
```bash
codex exec -s read-only --ephemeral -C "$PWD" --color never \
  -c model_reasoning_effort=low -o ./tmp/codex-review.md - < ./tmp/codex-prompt.md
```
Background it; full guide + wrapper: `.claude/skills/codex-task/`.

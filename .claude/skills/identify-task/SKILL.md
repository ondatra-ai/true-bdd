---
name: identify-task
description: First half of the codex-task workflow — define a task as Goal + Requirements (behavioral should/must statements from a user or system perspective; NO implementation), then kill ambiguity by asking the user ONE question at a time until no two distinct user-perspective interpretations remain, using Codex to surface missed risks. Takes the task idea from the skill argument — the text after /identify-task (e.g. `/identify-task make true-bdd connect to the harness host on Vercel`); if none is given, asks for it as the very first question. Use when scoping/defining a substantial change before implementation ("define/spec/scope this task", "what should this do"), or when the codex-task orchestrator calls it. Produces docs/tasks/<slug>.md (one brief per task, slug from the goal).
---

# Identify task

Define the task as **Goal + Requirements** — behavior, not implementation. Then remove every ambiguity by questioning the user. Output: `docs/tasks/<slug>.md` (a kebab-case slug of the goal).

## The brief format

- **Goal** — the outcome in 1–2 sentences. What changes for whom. Not how.
- **Requirements** — behavioral `should`/`must` **capabilities**, never process/doc chores or "how". **For how to phrase and file them — Product vs System, allowed subjects, capability phrasing, tagging, non-goals — read and follow `docs/context/requirements-guide.md`.** **Draft a complete list — both what the user revealed AND what you suggest** (your suggestions are valuable: infer likely requirements from the goal, the code, and Codex); tag each `[revealed]` (from the user) or `[suggested]` (your inference). Then **validate them one by one with Keep / Drop / Other** (step 8) — keep only the ones the user Keeps; never write an unvalidated requirement.

## The process

**First, get the task idea** — it comes from the skill argument, the text after
`/identify-task` (e.g. `/identify-task make true-bdd connect to the harness
host on Vercel`). If none was given, your very first action is to ask the user
one question — "What's the task?" — and wait. Don't analyze or plan without it.

1. **Analyze** the task + relevant files. Read `docs/context/requirements.md`, `docs/context/requirements-guide.md`, `docs/context/terms.md` (Harness/System/Product) and `docs/context/task_template.md`.
2. **Explore current behaviour** - read the code, the docs, use playwright, cli tools, mcp servers to explore the current behaviour, continuous integration, structure, architecture and system design. Dig deep. Identify the current behaviour and identify the goal.
3. **Draft a full list of requirements** — both what the user has revealed AND what you suggest from the goal, the code, and Codex. Tag each **[revealed]** (from the user) or **[suggested]** (your inference).
4. **Ambiguity test — the loop's engine.** Can you construct **two distinct, valid user-visible outcomes** that both satisfy the current requirements?
   - **Yes → it's ambiguous.** Form the sharpest disambiguating question and **ask the user ONE question** (never a batch). Update the requirements from the answer. Repeat.
   - **No → it's unambiguous.** Stop asking.
5. **If the user flags a wrong direction, shrink the task's purpose** — narrow scope, re-confirm the goal. Pushback means reconsider, not defend.
6. **Draft** the task brief as a working file under `./tmp/` (e.g. `./tmp/task-<slug>.md`, the kebab-case slug of the goal) — NOT yet in `docs/tasks/`. The final brief is written to `docs/tasks/<slug>.md` only after validation, at step 9; nothing lands in `docs/tasks/` until the user confirms, so no unvalidated requirement is ever persisted there.
7. **Ask codex to validate your requirements, find gaps and risks in your task** — Ask codex to read  `docs/context/requirements.md` and `docs/context/terms.md`, and the draft brief you saved under `./tmp/`. Ask codex to analyse the requirements and find any gaps, risks, or ambiguities. Analyse the response and incorporate only relevant suggestions. Repeat until no relevant suggestions found or 3 times.
8. **Validate requirements one by one with Keep / Drop / Other.** Present each requirement **in full** (the complete sentence, not a compressed label), **one requirement at a time**, naming whether it is `[revealed]` (from the user) or `[suggested]` (your inference). Use **one `AskUserQuestion` per requirement** with two options — **Keep** (accept into the brief) and **Drop** (remove it); the harness auto-adds **Other**, which the user picks to clarify, reword, swap the subject, or push back. **Format the question as the full requirement text, then TWO line breaks, then `Keep or drop? (Pick Other to reword or clarify.)`** — optionally followed by a short, requirement-specific example of what Other could change. Your suggestions are valuable — do propose them — but keep only the requirements the user **Keeps**; never write a requirement the user did not Keep. Apply any rewording from "Other" and re-confirm. Repeat until every requirement is Kept or Dropped.
9. **Exit** - show summary of each step from this skill, especially codex validation and user validation, and the final task brief. Ask the user if they want to save it to `docs/tasks/<slug>.md` (yes/no). If yes, write `docs/tasks/<slug>.md` (creating the folder if needed; update in place if it exists); if no, discard the draft and leave `docs/tasks/` untouched.

Run Codex non-interactively — without a sandbox flag it hangs:
```bash
codex exec -s read-only --ephemeral -C "$PWD" --color never \
  -c model_reasoning_effort=low -o ./tmp/codex-review.md - < ./tmp/codex-prompt.md
```
Background it; full guide + wrapper: `.claude/skills/codex-task/`.

---
name: identify-task
description: First half of the codex-task workflow — define a task as Goal + Requirements (behavioral should/must statements from a user or system perspective; NO implementation), then kill ambiguity by asking the user ONE question at a time until no two distinct user-perspective interpretations remain, using Codex to surface missed risks. Takes the task idea from the skill argument — the text after /identify-task (e.g. `/identify-task make true-bdd connect to the harness host on Vercel`); if none is given, asks for it as the very first question. Use when scoping/defining a substantial change before implementation ("define/spec/scope this task", "what should this do"), or when the codex-task orchestrator calls it. Produces docs/tasks/<slug>.md (one brief per task, slug from the goal).
---

# Identify task

Define the task as **Goal + Requirements** — behavior, not implementation. Then remove every ambiguity by questioning the user. Output: `docs/tasks/<slug>.md` (a kebab-case slug of the goal).

## The brief format

- **Goal** — the outcome in 1–2 sentences. What changes for whom. Not how.
- **Requirements** — each is a behavioral `should`/`must` **capability** (never a process/doc chore like "verified by a test" or "docs updated"). Prefer the **user** perspective for functionality — subject = the role by self-assessment: does the actor develop true-bdd itself (→ "A Developer") or their own software using true-bdd (→ "A BDD System Architect" / "A BDD Product Owner")? use a **system** subject ("The true-bdd CLI"/"The true-bdd harness") only for architecture/infra decisions (e.g. "must use Redis on the backend"). Subjects must be exact terms from `docs/context/terms.md`. Group the brief's requirements under **Product** (functionality) and **System** (architecture). No bare "System"/"user", no "how". **Draft a complete list — both what the user revealed AND what you suggest** (your suggestions are valuable: infer likely requirements from the goal, the code, and Codex). **Tag each requirement [revealed] (from the user) or [suggested] (your inference).** Then **validate them ONE BY ONE** with the user, naming the tag for each, and keep only the ones the user confirms — never write an unvalidated requirement. State a **non-goal** only when it prevents a double-interpretation.

## The process

**First, get the task idea** — it comes from the skill argument, the text after
`/identify-task` (e.g. `/identify-task make true-bdd connect to the harness
host on Vercel`). If none was given, your very first action is to ask the user
one question — "What's the task?" — and wait. Don't analyze or plan without it.

1. **Analyze** the task + relevant files. Read `docs/context/requirements.md` and `docs/context/terms.md` (Harness/System/Product) for standing requirements not in code.
2. **Draft a full list of requirements** — both what the user has revealed AND what you suggest from the goal, the code, and Codex. Tag each **[revealed]** (from the user) or **[suggested]** (your inference).
3. **Ambiguity test — the loop's engine.** Can you construct **two distinct, valid user-visible outcomes** that both satisfy the current requirements?
   - **Yes → it's ambiguous.** Form the sharpest disambiguating question and **ask the user ONE question** (never a batch). Update the requirements from the answer. Repeat.
   - **No → it's unambiguous.** Stop asking.
4. **If the user flags a wrong direction, shrink the task's purpose** — narrow scope, re-confirm the goal. Pushback means reconsider, not defend.
5. Save the task to `docs/tasks/<slug>.md` (a kebab-case slug of the goal; create the folder if needed; update in place if it exists).
6. **Ask codex to validate your requirements, find gaps and risks in your task** — Ask codex to read  `docs/context/requirements.md` and `docs/context/terms.md`, and current task you saved. Aks codex to analyse the requirments and find any gaps, risks, or ambiguities. Analyse response and incorporate only relevant suggestuins. Repeat until no relevant suggestions found or 3 times.
7. **Validate requirements via user one by one** — present each to the user individually, naming whether it is [revealed] (from them) or [suggested] (your inference); your suggestions are valuable, so do propose them — but keep only the ones the user confirms. Repeat until each surviving requirement is user-validated.
8. **Exit** when the two-interpretations test passes, Codex has nothing relevant left, and every requirement is user-validated. Write the brief to `docs/tasks/<slug>.md` (a kebab-case slug of the goal; create the folder if needed; update in place if it exists).

Run Codex non-interactively — without a sandbox flag it hangs:
```bash
codex exec -s read-only --ephemeral -C "$PWD" --color never \
  -c model_reasoning_effort=low -o ./tmp/codex-review.md - < ./tmp/codex-prompt.md
```
Background it; full guide + wrapper: `.claude/skills/codex-task/`.

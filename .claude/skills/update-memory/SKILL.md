---
name: update-memory
description: Check the pending diff against CLAUDE.md and update it when the changes alter something it records — repo structure, commands, conventions, workflows. Invoked from pr-commit before every commit; also usable standalone when the user asks to sync CLAUDE.md.
---

# Update Memory

CLAUDE.md is a **cache of the repository**, and a cache earns its load only where the lookup is
expensive. You are its write path, `audit-memory` the compaction pass: delete freely, add reluctantly.

**Never edit between the `KARPATHY:BEGIN`/`END` markers** — `scripts/check-karpathy-block.sh` fails
the commit. Keep the file **under 200 lines**; an addition that would breach it frees its lines first.

## Steps

1. **Scope.** `git --no-pager diff HEAD --stat` + `git status --short`. Nothing changed →
   report `memory: nothing to update` and stop.
2. **Per change, preferring earlier outcomes:**
   - **Correct** a line the diff made false.
   - **Delete** a line the diff made redundant — you documented the fact at its point of
     use, or added a gate that now enforces it. Enforcement beats prose.
   - **Add** — last resort, and only after step 3.
3. **Before adding, find a cheaper home and use that instead:** the package or function
   doc comment, the script's own header, `README.md`, a config file's comments,
   `docs/for_further/`, a ClickUp ticket, a linter rule, a `settings.json` deny rule.
   Write it there in the same commit and leave CLAUDE.md alone. Only a fact with no such
   home earns a line — the unwritten convention, the reason behind a choice, the gotcha
   no file confesses.
4. **Verify, never assume.** Grep that the fact is not already documented and that every
   name you write exists. Report `memory: +N/-M in ## <section>`, or `memory: no update needed`.

## Never write

What `ls`, `--help` or a doc comment answers · design rationale and post-mortem narrative
(→ a docstring or `docs/for_further/`) · code examples (point at one real file) · historical
notes ("X replaced Y", "Y is gone") · volatile counts · emphasis without content.

Cosmetic diffs need no update. Leave `CRITICAL` lines alone unless the change demands it.
A durable *user* preference is not a repo fact — it belongs in `~/.claude/CLAUDE.md`.

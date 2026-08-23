---
name: update-memory
description: Check the pending diff against CLAUDE.md and update it when the changes alter something it records — repo structure, commands, conventions, workflows. Invoked from pr-commit before every commit; also usable standalone when the user asks to sync CLAUDE.md.
---

# Update Memory

CLAUDE.md is a **cache of the repository**, and a cache earns its load only where the lookup is
expensive. You are its write path, `audit-memory` the compaction pass: delete freely, add reluctantly.

**Never edit between the `KARPATHY:BEGIN`/`END` markers**; keep the file under 215 lines and 80
columns. `scripts/lint-claude.md.sh` fails the commit on any of the three.

## Steps

1. **Scope.** `git --no-pager diff HEAD --stat` + `git status --short`. Nothing changed →
   report `memory: nothing to update` and stop.
2. **Per change, preferring earlier outcomes:** **correct** a line the diff made false;
   **delete** one it made redundant (you documented the fact at its point of use, or added
   a gate that now enforces it — enforcement beats prose); **add**, last resort, and only
   after step 3.
3. **Before adding, find a cheaper home and use that instead:** a doc comment, a script
   header, `README.md`, a config file's comments, `docs/for_further/`, a ClickUp ticket, a
   linter rule, a `settings.json` deny rule — or `.claude/rules/<topic>.md` when the fact
   matters in one part of the tree only, since a path-scoped rule loads just for files it
   matches. Write it there in the same commit. Only a fact needed in EVERY session with no
   such home earns a line: the unwritten convention, the reason behind a choice, the gotcha
   no file confesses.
4. **Verify, never assume.** Grep that the fact is not already documented and that every
   name you write exists. Report `memory: +N/-M in ## <section>`, or `memory: no update needed`.

## Never write

What `ls`, `--help` or a doc comment answers · design rationale and post-mortem narrative
(→ a docstring or `docs/for_further/`) · code examples (point at one real file) · historical
notes ("X replaced Y", "Y is gone") · volatile counts · emphasis without content.

Cosmetic diffs need no update. Leave `CRITICAL` lines alone unless the change demands it.
A durable *user* preference is not a repo fact — it belongs in `~/.claude/CLAUDE.md`.

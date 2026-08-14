---
name: update-memory
description: Check the pending diff against CLAUDE.md and update it when the changes alter something it records — repo structure, commands, conventions, workflows. Invoked from pr-commit before every commit; also usable standalone when the user asks to sync CLAUDE.md.
---

# Update Memory

CLAUDE.md is the project memory: repository layout, CLI subcommands, dev
commands, conventions, and workflow rules. A commit that changes any of
those makes CLAUDE.md stale. This skill checks the pending diff and updates
CLAUDE.md so the fix lands in the same commit.

## Steps

1. **Scope the pending diff.** `git --no-pager diff HEAD --stat` plus
   `git status --short`. If nothing changed, report `memory: nothing to
   update` and stop.
2. **Compare against CLAUDE.md.** For each change, ask: does CLAUDE.md
   state something this diff makes wrong or incomplete? Typical triggers:
   - files/directories moved, added, or removed from the documented layout;
   - CLI subcommands, flags, or behaviour added or changed;
   - dev commands (build, test, lint, run) changed;
   - a convention, rule, or workflow added or altered
     (including new/changed `.claude/skills/`);
   - config keys or document paths renamed.
3. **Update CLAUDE.md** — edit only the sections the diff invalidates;
   match the file's existing tone and density. Cosmetic diffs (wording,
   formatting, content-only doc edits) need no update: report
   `memory: no update needed` and stop.
4. **Report** what was updated in one line. No staging needed when run
   from pr-commit — its commit step stages everything.

## Rules

- Never record session-temporary facts, task narration, or anything
  derivable from the code itself — CLAUDE.md states durable conventions.
- Never touch the sections marked CRITICAL without the change clearly
  requiring it.
- If a durable *user preference* (not a repo fact) surfaced, it belongs in
  the auto-memory directory, not CLAUDE.md.

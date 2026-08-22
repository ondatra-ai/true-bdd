---
name: update-memory
description: Check the pending diff against CLAUDE.md and update it when the changes alter something it records — repo structure, commands, conventions, workflows. Invoked from pr-commit before every commit; also usable standalone when the user asks to sync CLAUDE.md.
---

# Update Memory

The project memory is CLAUDE.md plus the path-scoped rules in
`.claude/rules/` (`bdd-harness.md`, `engine-internals.md`,
`go-conventions.md`): repository layout, CLI subcommands, dev commands,
conventions, and workflow rules. A commit that changes any of those makes
the memory stale. This skill checks the pending diff and updates the file
whose scope the change belongs to, so the fix lands in the same commit.
Scoping rule: always-relevant facts go in CLAUDE.md; facts needed only
when working under a rule's `paths:` go in that rule file.

CLAUDE.md is loaded into every session — every line spends context on every
turn, whether or not it matters that turn. A line earns its place only if
Claude would behave differently without it.

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
3. **Update the owning file** — CLAUDE.md or the matching
   `.claude/rules/*.md`; edit only the sections the diff invalidates.
   Deleting is as much the update as adding: a line the diff made stale or
   redundant comes out, it does not get a correcting neighbour. Cosmetic
   diffs (wording, formatting, content-only doc edits) need no update:
   report `memory: no update needed` and stop.
4. **Report** what was updated in one line. No staging needed when run
   from pr-commit — its commit step stages everything.

## What earns a line

- **Record the unwritten**: conventions, rationale, gotchas, and refusal
  behaviours nothing else states. The test for a candidate line: without
  it, would Claude do the wrong thing?
- **Never cache a source of truth.** Anything stated by the code, a
  script's docstring, a config's own comments, a skill's SKILL.md, or
  discoverable by `ls`/`--help` gets at most a one-line pointer
  (e.g. "design rationale lives in merge.py's docstrings"). A restated
  copy goes stale the day its source changes.
- **Match the file's density**: compressed prose, bold lead phrases,
  tables over paragraphs. New material at the same altitude as its
  section — no war stories, no narration.

## Rules

- Never record session-temporary facts, task narration, or anything
  derivable from the code itself.
- Never touch the sections marked CRITICAL without the change clearly
  requiring it.
- CLAUDE.md files are the ONLY memory. A durable user preference belongs
  in CLAUDE.md's Notes section — never in the auto-memory directory,
  which this project does not use.

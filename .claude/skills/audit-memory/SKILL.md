---
name: audit-memory
description: Audit CLAUDE.md band by band, one read-only memory-auditor agent per band, gathered into one report of verdicts, stale claims and replacement text. Use when the user wants CLAUDE.md audited, shrunk, pruned, checked for repetition, or cut to a line target.
---

# Audit CLAUDE.md

0. **Skip the fenced region.** `CLAUDE.md` opens with a block between
   `<!-- KARPATHY:BEGIN … -->` and `<!-- KARPATHY:END -->` — a verbatim mirror of an
   upstream file, enforced by `scripts/check-karpathy-block.sh`. It contains `##`
   headings, so step 1 would band it like any other section: don't. Never audit,
   budget, rewrite or spawn an agent for anything inside those markers, and subtract
   its line count from the target before setting budgets. Auditing it can only produce
   a diff the gate rejects.
1. Split the REST of `CLAUDE.md` at its `##` headings into bands; set per-band
   **budgets** summing to the remaining target (ask; default 100), sized by value, not
   current length.
2. Spawn one `memory-auditor` per band **in parallel**, each told its line range,
   headings, and budget — plus what the fenced block already covers, so no band spends
   lines restating it.
3. Recreate `tmp/memory-audit/` empty; write each report there as `NN-<band>.md`.
4. Assemble `cat tmp/memory-audit/[0-9]*.md > tmp/memory-audit.md` under a summary: per-band now→proposed lines, stale claims, must-move facts, decisions for the human.
5. Hand over the file and stop — acting on it is the user's call.

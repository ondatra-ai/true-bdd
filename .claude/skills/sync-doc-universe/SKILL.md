---
name: sync-doc-universe
description: Audit the current state of the documents declared in true-bdd/true-bdd.yaml (documents:, document dirs in paths:, templates) against the doc universe (docs/doc-universe.md + docs/doc-universe.html), in both directions, and resolve every inconsistency by asking the user. Not diff-based — it checks what exists now. Invoked from pr-commit before every commit; also usable standalone when the user asks to align the doc universe.
---

# Sync Doc Universe

The doc universe — `docs/doc-universe.md` and its interactive twin
`docs/doc-universe.html` — describes the *structure* of the spec documents:
which files exist, which fields they carry, how they join, and what each
command reads and writes. This skill audits the **current state** of both
sides — not the diff; drift is found wherever it came from — and resolves
every inconsistency **by asking the user — never silently**.

## The two sides

The documents side is **not a hardcoded list** — resolve it from
`true-bdd/true-bdd.yaml` at run time, so relocations in the config are
picked up automatically:

| Side | Resolved from |
| --- | --- |
| **Documents** | every file under `documents:` (product, architecture_yaml, scenarios_yaml); every document directory under `paths:` (epics_dir, stories_dir, checklists_dir); every template file under `templates.prompts:` |
| **Universe** | `docs/doc-universe.md`, `docs/doc-universe.html` |

Exclude the runtime paths (`tmp_dir`, `tmp_glob`, `test_write_globs`) —
they hold artifacts and code trees, not documents. `true-bdd.yaml` itself
is in scope too: the universe describes its keys (joins 13–16).

The two universe files are two renderings of the same content (the html
adds the hoverable join map — cards like `card-product`, anchor rows like
`a-product-roles`, the `#joins` table). They must also stay consistent
with *each other*.

## What counts as an inconsistency

Structural claims only:

- a file or directory the universe names that does not exist (or moved);
- a field the universe references (`roles[].name`, `vocabulary`,
  `stories[].id`, `as_a`, `service:`, `quality_gate.tests`, …) that is
  absent, renamed, or shaped differently in the actual document — or a
  structural field the documents carry that contradicts what the universe
  says that document contains;
- a join whose mechanics no longer match (id derivation, path resolution,
  who reads/writes what);
- a command behaviour stated in the universe's tables that no longer
  matches the engine's documented behaviour;
- the two universe files disagreeing with each other.

Prose rewording, tone, or language-level differences that leave structure
and meaning intact are **not** inconsistencies — do not flag them.

## Steps

1. **Resolve the scope.** Read `true-bdd/true-bdd.yaml` and collect the
   in-scope paths per the table above.
2. **Inventory the universe's claims.** Read both universe files and list
   every checkable claim: named paths, per-document field lists, the
   command read/write table, the numbered joins.
3. **Universe → documents.** Verify each claim against what actually
   exists: the files, their fields, the config keys, the templates.
4. **Documents → universe.** Walk each in-scope document's actual
   structure and check the universe's description of that document still
   covers it — a field or file the universe's account of that document
   omits or contradicts is an inconsistency.
5. **md ↔ html.** Compare the two renderings' content claim by claim;
   any content present or stated differently in only one is an
   inconsistency.
6. **Ask about every inconsistency.** Use AskUserQuestion — one question
   per inconsistency (batch up to 4 per call), quoting the exact text on
   both sides. Every option label must state the direction explicitly:
   **what is truth → what gets updated**. The standard options:
   - **«&lt;document&gt; is truth → doc universe updated»** —
     `doc-universe.md` AND `doc-universe.html` are edited to match the
     document.
   - **«doc universe is truth → &lt;document&gt; updated»** — the document
     is edited to match the universe.
   - When it applies, also offer **«… is truth → nothing updated»** —
     e.g. the universe states the host contract and the document's
     omission is a legitimate empty or optional instance.
   - **Custom** — the built-in "Other" free-text answer; the user writes
     how to approach it. Follow that instruction exactly.
7. **Apply the chosen resolutions.** Keep md and html telling the same
   story; in the html, remember the join map (card anchors, `#joins`
   table, the SVG edges) may also encode the claim being fixed.
8. **Report** the list of inconsistencies and how each one was resolved
   (or `doc universe: consistent` when none were found). No staging
   needed when run from pr-commit — its commit step stages everything.

## Rules

- Never resolve an inconsistency without asking — even an "obvious" one.
- Never publish `doc-universe.html` as an artifact or anywhere else —
  merging it to main IS its deploy (GitHub Pages).
- Do not invent inconsistencies from style or wording; structure only.
- When the universe's dated footer (`*Drawn from … — YYYY-MM-DD.*`) is
  present and content changed, refresh the date as part of the fix.

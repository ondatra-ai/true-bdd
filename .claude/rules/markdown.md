---
paths:
  - "**/*.md"
---

# Markdown

`go run ./scripts/cmd/linters markdown [FILE...]` gates every `.md` this repository
authors, with markdownlint-cli2 and `.markdownlint-cli2.yaml`. Named files are
auto-fixed; bare, it only reports. It runs per-edit from the PostToolUse hook
and again in the gates and CI, so a breach surfaces at the edit that made it.

**Write long lines.** `MD013` is off: prose runs to whatever length the
sentence wants, and hand-wrapping at 80 columns is churn the gate will not
thank you for. `MD060` is off too: the corpus is 58 compact table pipes to 3 aligned, and reflowing a
table that already reads fine is what Surgical Changes forbids.

**Give every file a top-level heading.** `MD041` is ON — except for the file
kinds that open with prose because a model reads them top-down as
instructions, which `overrides:` in the config exempts by glob: `.claude/**`,
`**/*.prompt.md`, `**/SKILL.md`. Everywhere else — `README.md`, `docs/**` — a
missing `#` is a real defect and the gate says so.

**What still needs a real edit** (everything else `--fix` handles): a fenced
block needs a language — `text` is the repo's choice for ASCII trees and
diagrams — and an ordered list numbers `1/2/3` with no gaps, so deleting a
step means renumbering the ones after it and any cross-reference that counts
them.

**A stray config file silently rewrites the rules under it.** cli2 finds its
config by walking up from each linted file and merging what it passes, so a
committed `.markdownlint.yaml` or `.markdownlint-cli2.yaml` in any
subdirectory changes that subtree's rules with nothing to announce it — the
same trap as an implicit CLI configuration file. One config, at the root.

**Out of scope, and not yours to fix.** The `.claude/skills/mpc_*` directories
are MIT copies of `mattpocock/skills` (manifest:
`.claude/skills/VENDORED-mattpocock.md`); the gate skips them by that prefix,
because a fix here turns the next re-sync from a copy into a three-way merge. Also
skipped: `*/testdata/` goldens and `proto-product-snapshot.md`, where the
bytes are machine output rather than prose.

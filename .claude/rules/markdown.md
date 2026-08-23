---
paths:
  - "**/*.md"
---

# Markdown

`./scripts/lint-markdown.sh [FILE...]` gates every `.md` this repository
authors, with markdownlint-cli2 and `.markdownlint-cli2.yaml`. Named files are
auto-fixed; bare, it only reports. It runs per-edit from the PostToolUse hook
and again in `gates.sh` and CI, so a breach surfaces at the edit that made it.

**Write long lines.** `MD013` is off: prose runs to whatever length the
sentence wants, and hand-wrapping at 80 columns is churn the gate will not
thank you for. `CLAUDE.md` is out of this gate's scope entirely — `.alint.yml`
owns its line budget, its opening block, its width and its mirror. `MD060` is
off too: the corpus is 58 compact table pipes to 3 aligned, and reflowing a
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
same trap as a root `crush.json`. One config, at the root.

**Out of scope, and not yours to fix.** The 24 directories named in
`.claude/skills/VENDORED-mattpocock.md` are MIT copies of
`mattpocock/skills`; the gate parses that manifest and skips them, because a
fix here turns the next re-sync from a copy into a three-way merge. Also
skipped: `*/testdata/` goldens and `proto-product-snapshot.md`, where the
bytes are machine output rather than prose.

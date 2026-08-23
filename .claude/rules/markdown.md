---
paths:
  - "**/*.md"
---

# Markdown

`./scripts/lint-markdown.sh [FILE...]` gates every `.md` this repository
authors, with markdownlint-cli and `.markdownlint.yaml`. Named files are
auto-fixed; bare, it only reports. It runs per-edit from the PostToolUse hook
and again in `gates.sh`, so a breach surfaces at the edit that made it.

**Write long lines.** `MD013` is off: prose runs to whatever length the
sentence wants, and hand-wrapping at 80 columns is churn the gate will not
thank you for. `CLAUDE.md` is the single exception — it is 80-column and
`scripts/lint-claude.md.sh` owns it, which is why this gate skips it.

**Leave tables as they are.** `MD060` is off because the corpus is 58 compact
pipes to 3 aligned; reflowing one that already reads fine is exactly the churn
Surgical Changes forbids. `MD041` is off too — Claude Code files open with
frontmatter, not an h1, and that is the format working as designed.

**What still needs a real edit** (everything else `--fix` handles): a fenced
block needs a language — `text` is the repo's choice for ASCII trees and
diagrams — and an ordered list numbers `1/2/3` with no gaps, so deleting a
step means renumbering the ones after it and any cross-reference that counts
them.

**Vendored skills are out of scope.** The 24 directories named in
`.claude/skills/VENDORED-mattpocock.md` are MIT-licensed copies of
`mattpocock/skills`; the gate parses that manifest and skips them, because a
fix here turns the next re-sync from a copy into a three-way merge. Golden
files under `*/testdata/` are skipped for the same class of reason: there the
bytes are the assertion.

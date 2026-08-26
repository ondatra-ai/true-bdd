# Vendored skills — mattpocock/skills

The 24 skill directories listed below are copied from
<https://github.com/mattpocock/skills> (`skills/engineering/` and
`skills/productivity/`), MIT licensed, Copyright (c) 2026 Matt Pocock.

They are **project-scoped on purpose**: they live in this repository's
`.claude/skills/`, so they apply to work in this repo and nowhere else.
Installing via `claude plugins install mattpocock-skills` or
`npx skills add` would have put them on every project instead.

Not taken: `setup-matt-pocock-skills` — it configures the upstream author's
issue-tracker and domain conventions, which are not this repo's. The one
artifact it would have produced, `docs/agents/issue-tracker.md`, is written
for this repo instead, so `code-review`'s "tell the user to run
`/setup-matt-pocock-skills`" branch never fires.

Also stripped: each skill's `agents/openai.yaml`. Upstream ships every skill
for Codex as well as Claude Code, and that file is the Codex half — a
`display_name`, a `short_description` and an `allow_implicit_invocation`
policy. Claude Code reads none of it and no SKILL.md references it. The
companion `.md` and `.sh` files were kept: every one of them IS referenced
by its own SKILL.md, so removing them would break the skill.

Consequence to know when re-syncing from upstream: these deletions make a
straight diff against the source repo noisy. Re-take the skill directories
wholesale and strip `agents/` again rather than merging file by file.

Taken deliberately despite a clash: `code-review` **shadows Claude Code's
built-in skill of the same name**, so `/code-review` now runs Matt's version
and the built-in `ultra` / `--fix` / `--comment` behaviour is unavailable in
this repo.

Engineering: ask-matt, code-review, codebase-design, diagnosing-bugs,
domain-modeling, grill-with-docs, implement, improve-codebase-architecture,
prototype, research, resolving-merge-conflicts, tdd, to-spec, to-tickets,
triage, wayfinder, wizard.

Productivity: grill-me, grilling, handoff, teach, to-questionnaire,
wait-what, writing-for-agents.

## Licence

MIT License

Copyright (c) 2026 Matt Pocock

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.

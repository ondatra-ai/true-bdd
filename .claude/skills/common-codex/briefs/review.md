# Review, {{standards or spec}} axis: {{change}}

Review one TrueBDD change along one axis. Change nothing; your final message is your report.

## Change

- Diff: `git diff {{fixed point}}...HEAD {{path filters}}`, from {{checkout or worktree path}}.
- Commits: `git log --oneline {{fixed point}}..HEAD`.
- The Owner's decisions: {{the `docs/qna/` log and its `Q<n>` this run carries out, or none}}. Read `docs/qna/` as `docs/agents/domain.md` says: oldest to newest, a newer entry overriding an older one.

## Axis

Do what `.claude/skills/mpc_code-review/SKILL.md` prescribes for its {{Standards or Spec}} reviewer in step 4. Its other steps remain Claude's. Keep one axis below:

- Standards: `CLAUDE.md`, `GLOSSARY.md`, relevant `docs/adr/`, matching `.claude/rules/*.md` and {{other standards}}, with the smell baseline from step 3. Check the typed CLI, disk and logging boundaries, and evidence of alint, gates and required BDD replay.
- Spec: {{spec and ticket issue numbers, saved through `--issue`, or document paths}}.

## Report

The shape that step 4 prescribes for this axis, with `file:line` and evidence for every finding. No preamble.

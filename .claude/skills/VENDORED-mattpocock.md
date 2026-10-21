# Vendored skills: mattpocock/skills

The 35 `mpc_*` skill directories are copied from
<https://github.com/mattpocock/skills> at commit `b0618bc` (all of `skills/engineering/`,
`skills/productivity/`, `skills/in-progress/` and `skills/misc/`), by way of `boarlabsxyz/ovchynme`, which
took 38: this repository leaves out `mpc_writing-beats`, `mpc_writing-fragments` and `mpc_writing-shape`. MIT licensed, Copyright (c) 2026 Matt Pocock;
see `LICENSE-mattpocock`.

Local changes against upstream:

- Every skill is renamed `mpc_<name>`: the directory, the frontmatter `name:`, and every reference one
  skill makes to another (`/name`, `"name"` in Skill-tool calls, `` `name` `` where it means the skill).
  Backticked words that are labels or ticket types rather than skills (`wayfinder:<type>` values in
  `mpc_wayfinder/SKILL.md` and `mpc_setup-matt-pocock-skills/issue-tracker-*.md`, the `prototype` path
  hint in `mpc_prototype/UI.md`) are left as-is.
- Each skill's `agents/openai.yaml` is dropped. It is the Codex half of upstream's packaging and
  Claude Code doesn't read it.
- Bundled `.sh` and `.cjs` files carry a `.tmpl` suffix, and the links to them in each SKILL.md follow:
  this repository's `.alint.yml` allows no shell and fences JS (`template.sh.tmpl`,
  `hitl-loop.template.sh.tmpl`, `block-dangerous-git.sh.tmpl`, `dependency-cruiser.config.cjs.tmpl`).
- `mpc_grilling` is rewritten to ask one question per turn, as plain chat text rather than through
  AskUserQuestion, and to research the environment and the web before each question. Research alone
  settles only facts and consequences an earlier answer forces; every decision is asked, and no ADR is
  written for an unanswered one. Upstream asks the whole frontier in one round. The lines that
  describe grilling in rounds in `mpc_loop-me`, `mpc_triage` and `mpc_ask-matt` now say one question
  at a time.
- `mpc_grilling` also writes a Q&A log of each session to `docs/qna/<UTC-ts>-<slug>.yaml`, in the
  format of `.claude/qna.example.yaml` (a "Log it" step in the per-turn loop and a "The Q&A log"
  section). Upstream keeps no log. The `mpc_grill-me` entry in `mpc_ask-matt` no longer calls it
  stateless and says the Q&A log is its only trail.

To re-sync, re-take the directories wholesale from upstream and repeat the rename and the drop, rather
than merging file by file. Then restore the local `mpc_grilling/SKILL.md` and the four one-line edits
(three for one question at a time, one for the `mpc_grill-me` trail), folding in anything new from
upstream's `grilling`.

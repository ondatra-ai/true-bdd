# Fixture workspace: build-code-fix-by-codex

Synthetic host project driven by TrueBDD's `build code --fix`. This file
and the task you are given are the only instructions that apply here;
ignore any AGENTS.md of a directory above this one.

- Production source lives under `services/calc/`. That is the only tree
  the fix loop may edit.
- Executable tests live under `tests/suite/`. **Never** modify a test to
  make it pass — the failing assertion is the specification, and the
  production code is what is wrong.
- Never modify `docs/architecture/architecture.yaml` or `true-bdd/`.

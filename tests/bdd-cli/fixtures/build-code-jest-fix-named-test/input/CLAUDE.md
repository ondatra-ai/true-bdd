# Fixture workspace: build-code-jest-fix-named-test

Synthetic host project driven by TrueBDD's `build code --fix`.

- Production source lives under `services/greeter/`. That is the only tree
  the fix loop may edit.
- Jest tests live under `tests/unit/`. **Never** modify a test to make it
  pass: the failing assertion is the specification, and the production
  code is what is wrong.
- Never modify `docs/architecture/architecture.yaml` or `true-bdd/`.

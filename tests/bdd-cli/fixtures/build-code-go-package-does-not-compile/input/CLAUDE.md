# Fixture workspace: build-code-go-package-does-not-compile

Synthetic host project driven by TrueBDD's `build code --fix`.

- Production source lives under `services/calc/`. That is the only tree
  the fix loop may edit.
- Executable tests live under `tests/suite/`. **Never** modify a test to
  make it compile or pass: the test is the specification, and the
  production code is what is missing.
- Never modify `docs/architecture/architecture.yaml` or `true-bdd/`.

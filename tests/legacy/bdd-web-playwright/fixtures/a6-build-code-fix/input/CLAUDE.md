# Fixture workspace: a6-build-code-fix

Synthetic Go host project driven by TrueBDD's `build code --fix` from the
web harness.

- Production code lives under `services/calculator/` (Go module
  `calculator`).
- The failing test is `services/calculator/calc_test.go`. Fix the
  PRODUCTION source (`calc.go`) so the test passes — never edit the test
  file, never weaken an assertion.
- Only write or edit files under `services/`.

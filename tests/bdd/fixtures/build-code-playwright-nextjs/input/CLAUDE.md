# Fixture workspace: build-code-playwright-nextjs

Synthetic host project driven by TrueBDD's `build code --fix`.

- Production source lives under `services/`; the frontend Next.js service is
  expected at `services/frontend/`.
- Playwright tests under `tests/` are read-only. Make failing tests pass by
  creating or editing production source under `services/` only.
- Never modify `tests/`, `docs/requirements.yaml`, or `bdd-cli/`.

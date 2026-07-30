# Terms

The only allowed SUBJECTS for requirements in `docs/context/requirements.md`.
Every requirement is phrased as `<one of these terms> should/must …` and filed
under the matching section — never the bare words "System" or "user".

## Roles
Subjects for `# Product` (user-experience) requirements.
- **A BDD System Architect** — the end user of true-bdd who uses the CLI and web
  interface and is responsible for the architecture of the developed software.
- **A BDD Product Owner** — the end user of true-bdd who uses the CLI and web
  interface and is responsible for the PRD and requirements of the developed
  software.
- **A Developer** — the one who develops true-bdd itself (e.g. Peter) and is
  responsible for its architecture, code, requirements, and so on.

## Systems
Subjects for `# System` (system-design) requirements.
- **The true-bdd CLI** — the CLI part of true-bdd; the real harness that does
  most of the work. Lives in `./src`.
- **The Claude Code** — the agent (you) that writes and creates the code.

## Harness
Subjects for `# Harness` requirements.
- **The true-bdd harness** — the web part of true-bdd; the interface for managing
  requirements, architecture, and documents. Lives in `./harness`.

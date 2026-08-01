# Terms

The only allowed SUBJECTS for requirements in `docs/context/requirements.md`.
Every requirement is phrased as `<one of these terms> should/must …` and filed
under the matching section — never the bare words "System" or "user".

| requirements.md section | terms category | subjects | captures |
|---|---|---|---|
| `# Product` | **Roles** | A BDD System Architect, A BDD Product Owner | end-user product experience |
| `# System` | **Systems** | The true-bdd CLI, The true-bdd harness | architecture/infrastructure decisions only (technology choices, not behavior) |
| `# Harness` | **Harness** | A Developer | the dev harness / tooling ("it is you") |

## Roles
Subjects for `# Product` (user-experience) requirements.
- **A BDD System Architect** — the end user of true-bdd who uses the CLI and web
  interface and is responsible for the architecture of the developed software.
- **A BDD Product Owner** — the end user of true-bdd who uses the CLI and web
  interface and is responsible for the PRD and requirements of the developed
  software.

## Systems
Subjects for `# System` (system-design) requirements.
- **The true-bdd CLI** — the CLI part of true-bdd; the real harness that does
  most of the work. Lives in `./src`.
- **The true-bdd harness** — the web part of true-bdd; the interface for managing
  requirements, architecture, and documents. Lives in `./harness`.

## Harness
Subjects for `# Harness` improvements. Dont confuse with **The true-bdd harness**. Harness it is you. e.g. system where Developer develop **The true-bdd CLI** and  **The true-bdd harness**  The only role possible here is:

- **A Developer** — the one who develops true-bdd itself (e.g. Peter) and is
  responsible for its architecture, code, requirements, and so on.
# A host's configuration, checklists and templates come only from `true-bdd init`

A released binary carried none of the files a host needs, so a user unpacked 36 of them by hand. A missing template surfaced only after a full test run.

We decided:

- **What `init` writes.** `true-bdd init` writes the host's configuration, without this repository's `scripts` section, plus every checklist and every prompt template. It never overwrites an existing file.
- **Nothing embedded for running.** The binary may carry copies only for `init` to write. At run time the engine reads the project's files and falls back to nothing.
- **The check at start.** Each command that uses these files checks them before its first test run, item or AI turn:
  - a missing file stops it, naming the file and `true-bdd init`;
  - an invalid file stops it with its parse error.

  `version` and `init` need none of them.

## Considered Options

- **Embedded defaults with per-file override.** The binary would run from the release alone. But a host would run on files it cannot see, and an upgrade would change its behaviour silently.
- **A resources archive beside the binary.** That means two downloads, the same drift as `init`, and no guidance when a file is missing.

Grilling log: `docs/qna/20261026-175738-triage-215-216-221.yaml`: Q10, Q11, the delegated and forced entries after Q10, and the corrections entry.

# Re-spec the `us-refine-*` fixtures onto the current architecture shape

**Status**: known and accepted 2026-08-23, not started. Moved here out of `CLAUDE.md`
during the memory audit, because it is time-limited debt and that file is permanent.
**Kind**: internal test-fixture maintenance. No registry scenario changes.

## The inconsistency

Two documents still describe an architecture shape the engine no longer uses:

1. **The eleven `us-refine-*` fixtures** ship an `architecture.yaml` in the OLD
   `quality_gate.tests.{integration,e2e}` shape — e.g.
   `tests/bdd-cli/fixtures/us-refine-happy-path/input/docs/architecture/architecture.yaml:16`.
   The current shape is one repository-wide `testing.suites[]` block beside `services:`.
2. **`true-bdd/checklists/us-refine.yaml:288`** still asks the model to cite "the
   framework per service and per layer" — per-layer being the vocabulary of the old
   shape.

## Why nothing breaks today

Nothing parses those documents. They reach the prompt as text, so the model reads
whatever the fixture ships and the cassettes stay valid. The suite is green and stays
green. The cost is only to a reader: **do not read those fixtures as the current spec
shape.**

## Why it was not fixed in place

Changing either document changes the prompt, which changes the request hash, which
invalidates every affected cassette. Aligning both means re-recording eleven fixtures
at roughly five minutes of real model time each — about an hour of live model calls,
plus the review of eleven regenerated recordings. That belongs in its own commit, not
bolted onto whatever change happens to notice the drift.

## What to change

- Rewrite the `architecture.yaml` under each `tests/bdd-cli/fixtures/us-refine-*/input/docs/architecture/`
  into the `testing.suites[]` shape (`ls -d tests/bdd-cli/fixtures/us-refine-*` for the current set).
- Reword `true-bdd/checklists/us-refine.yaml:288` off "per service and per layer" onto
  the suite vocabulary.

## Verification

```bash
go test -tags bdd -run '^TestE2E0..$' ./tests/bdd-cli/ -mode=record   # re-record the affected ids
go test -tags bdd ./tests/bdd-cli/ -mode=replay                        # then replay must be green
./scripts/lint-schemas.sh
```

The one thing this could silently break: a re-recording that passes for the wrong
reason. Read the golden diffs rather than trusting the exit code — a fixture whose
prompt changed shape may still pass while checking something weaker than before.

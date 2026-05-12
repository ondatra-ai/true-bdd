# Expectations for `us create 99.1 --fix`

The synthetic epic at `docs/epics/jsons/epic-99-bdd-test.yaml`
deliberately contains an implementation term (`REST endpoint`) inside
the story's `so_that` clause. The `--fix` flow should:

1. Detect that the "implementation terms in so_that" check fails
2. Generate a fix prompt
3. Apply the fix (Claude rewrites the `so_that` to remove the
   implementation term)
4. Re-validate; once all checks pass, write the final story to
   `docs/stories/`

## What MUST be true after the run

1. **Exactly one new file** under `docs/stories/` with a name matching
   `99.1-<slug>.yaml`. No other files in `docs/stories/`.
2. The new file is **valid YAML** with a top-level `story:` key.
3. Inside `story:`, the field `id` (or `story_number` — whichever the
   CLI emits) is the string `"99.1"`.
4. The fields `as_a`, `i_want`, and `so_that` from the epic input are
   preserved in the story output, with the following caveat for
   `so_that`:
     - The original `so_that` contained `REST endpoint` (an
       implementation term). The final story's `so_that` MUST NOT
       contain `REST`, `endpoint`, `API`, `SDK`, `database`,
       `WebSocket`, `JWT`, `microservice`, `cache`, `GraphQL`, or any
       other internal-implementation term. Equivalent user-facing
       wording is fine — anything along the lines of "into a separate
       tool", "into another app", "into another application", "into a
       separate workflow" is acceptable.
     - The SUBJECT must stay the same: avoiding the need to download
       the document or copy-paste large sections of it elsewhere just
       to get a quick gist.
   - `as_a` is "Claude User"
   - `i_want` is about Claude producing a short summary of a Google
     Doc on request
5. The story has at least 3 acceptance criteria. Each AC has:
   - an `id` matching the regex `^AC-\d+$`
   - a non-empty `description` string
   - the SUBSTANCE of the three input ACs (summary of a shared Google
     Doc, error path when the doc is not shared, partial-summary path
     when the doc is too long) is covered — order may differ, wording
     may be polished, additional ACs are allowed.

## What MUST NOT happen

- No files modified or created outside `docs/stories/` and `tmp/`. In
  particular, no edits to `bdd-cli/`, `docs/epics/`, `docs/architecture*`,
  `docs/prd.md`, or `scripts/`.
- The synthetic epic file must be unchanged.

## Tolerances

- Exact wording, exact AC count, ordering, and additional helpful
  fields the CLI may emit (status, dev_notes, tasks, embedded
  Given/When/Then steps, etc.) are all acceptable as long as the rules
  above hold.
- If the diff includes files inside `tmp/`, ignore them — they are
  scratch artifacts (per-prompt request/response/result files plus
  per-fix-iteration story versions), not part of the user-facing
  output.

If all of the above hold, reply `PASS`. Otherwise reply
`FAIL: <one-sentence reason>` describing the first violation you find.

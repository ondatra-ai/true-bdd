# Expectations for `us create 99.1`

The command extracts story `99.1` from the synthetic epic at
`docs/epics/jsons/epic-99-bdd-test.yaml` and writes a refined story
file to `docs/stories/`.

## What MUST be true after the run

1. **Exactly one new file** under `docs/stories/` with a name matching
   the pattern `99.1-<slug>.yaml` where `<slug>` is a non-empty
   kebab-case string. No other files in `docs/stories/` should be
   created or modified.
2. The new file is **valid YAML** with a top-level `story:` key.
3. Inside `story:`, the field `id` (or `story_number` — whichever the
   CLI emits) is the string `"99.1"`.
4. The fields `as_a`, `i_want`, and `so_that` from the epic input are
   preserved in the story output. Light rewording is acceptable
   (Claude may polish phrasing) but the SUBJECT must stay the same:
     - `as_a` is "Claude User"
     - `i_want` is about Claude producing a short summary of a Google Doc on request
     - `so_that` is about avoiding the need to download the doc or copy-paste sections into a separate tool just to get a quick gist
5. The story has at least 3 acceptance criteria. Each AC has:
   - an `id` matching the regex `^AC-\d+$`
   - a non-empty `description` string
   - the SUBSTANCE of the three input ACs (summary of a shared Google
     Doc, error path when the doc is not shared, partial-summary path
     when the doc is too long) is covered — order may differ, wording
     may be polished, additional ACs are allowed.

## What MUST NOT happen

- No files modified or created outside `docs/stories/`. In particular,
  no edits to `bdd-cli/`, `docs/epics/`, `docs/architecture*`,
  `docs/prd.md`, or `scripts/`.
- The synthetic epic file must be unchanged.

## Tolerances

- Exact wording, exact AC count, ordering, and additional helpful
  fields the CLI may emit (status, dev_notes, tasks, embedded
  Given/When/Then steps, etc.) are all acceptable as long as the rules
  above hold.
- If the diff includes files inside `tmp/`, ignore them — they are
  scratch artifacts (per-prompt request/response/result files), not
  part of the user-facing output.

If all of the above hold, reply `PASS`. Otherwise reply
`FAIL: <one-sentence reason>` describing the first violation you find.

# Stories are written directly; epics and `us create` leave the pipeline

Epics and `us create` leave the pipeline, engine included. A Story is written directly, out of a discussion of a Feature, into `docs/product/stories/`, and the pipeline starts at `us refine`; a Feature is a label a Story carries, not a document of its own. Nothing ever produced epics and nobody could say what one should hold, while Stories themselves are expected to give way to grouped Scenarios, so designing a new `us create` would have been building for a concept due to disappear.

## Considered Options

- A Feature as a renamed epic file, read by `us create` as before: a new word for the same seed file, leaving open where a feature discussion lands.
- Features inside `product.yaml`, holding the story seeds: the product document would grow with every feature, and every checklist reads it whole.
- `us create` writing a Story from a free-form idea: a new command to design and record, for a concept expected to go.

## Consequences

- `us create`'s nine checks read only the story statement (`as_a`, `i_want`, `so_that`), so no check on Acceptance Criteria or Scenarios is lost; the statement checks are gone until moved into `us refine`.
- Engine-wide behaviour that only `us create` scenarios covered (the `--fix` refusal without `F:`, a prompt citing an unknown document) gets a replacement scenario on a surviving command in the same change.
- A Story id is a slug (`^[a-z0-9][a-z0-9.-]*$`), so the old `<epic>.<story>` ids such as `96.2` stay valid and no cassette changes.

Decided in `docs/qna/20261021-112516-self-spec-and-doc-validation-chain.yaml`, Q3 to Q6 and Q16.

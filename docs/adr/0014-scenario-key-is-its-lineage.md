# A Scenario's Registry key is its lineage id

A Scenario's Registry key is its lineage id, `<story-id>-<NNN>-<slug>` (for example `scen-check-001-unknown-id-refused`), and its generated test is named from it in CamelCase; `E2E-NNN` numbering retires. The author writes `<NNN>-<slug>` as the Acceptance Criterion's `id:`, so the key survives reordering the Story's criteria and never depends on a model. The key is what a reader sees in `go test` output, and a bare number said nothing about what is tested; one identifier from Story to test output replaces two.

## Considered Options

- The slug in the lineage id only, Registry keys staying `E2E-NNN`: cheap, but the test output stays unreadable.
- A separate `slug:` field beside unchanged keys: a third name for the same Scenario.
- The number taken from the criterion's position: reordering a Story renumbers every later key and test.
- A slug invented by the model during `us apply`: a repeated apply may name the same Scenario differently.

## Consequences

- `us apply` derives the key and its `F:` no longer picks "the next free `E2E-NNN`", so its cassettes are re-recorded once.
- The test-name renderer CamelCases the key, since `go test` never runs a name like `Testscen…`.
- Legacy `AC-N` ids in `us refine` fixtures stay: `us refine` derives no key.

Decided in `docs/qna/20261021-112516-self-spec-and-doc-validation-chain.yaml`, Q16 to Q18.

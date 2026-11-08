# Offline replay audit for #300

The completed full replay in tmp/300/replay-final.log ran 362 Scenarios: 268 passed, 94 failed, none skipped. It used services:replay,tests:replay and made no live model call.

All 91 selectors in #299's saved inventory still fail on stale recordings; #302 adds no fixtures. There are exactly three additional failed selectors:

- TestInit002ExistingFiles: the newly shipped conditional template bytes differ from its old golden.
- TestStepArguments001WholeSection: the new step-arguments-present fixture has no recording yet.
- TestStepArguments002AbsentUnchanged: the new step-arguments-absent fixture has no recording yet.

Every failed test block has a direct request-hash/stale-cassette, missing-cassette or differing-golden diagnostic. Downstream unconsumed cassettes and assertions stopped by those errors do not introduce another failure class. Init-empty was already inventoried by #299 and gains the shipped-defaults reason from #300.

The narrowed shipped description-format Q and the whole saved-prompt assertions of the two new fixtures were checked through the public ChecklistEvaluator seam in TestStepArgumentScenarioPromptsBeforeRecording, with an in-process provider reply. That verifies the assertions without creating, copying or editing a cassette.

The final rerun, after selected-architecture and flow/alias loader checks, is saved as tmp/300/replay-final2.log and checked against this same selector set. The changed-recording inventory is tmp/300/rerecord.txt. No cassette or golden changed relative to c210343b277dce1214798f74a91e75678091e232.

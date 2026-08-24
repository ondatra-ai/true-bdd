# Implement `true-bdd scen check`

**Status**: designed 2026-08-24, not started. Spans more than one session — this file
is the cold-pickup record. Delete it when the work lands.
**Kind**: new CLI command plus a new checklist. Adds registry scenarios E2E-291…E2E-297.
**Design record**: `docs/adr/0001-scen-check-is-advisory-and-per-scenario.md` holds the
two decisions a reader would otherwise mistake for oversights.

## What the command is

`scen check [scenario-id...]` walks entries in the registry
(`documents.scenarios_yaml`, conventionally `docs/scenarios.yaml`) against a new
`scen-check` checklist — one cell per (scenario, prompt), exactly the shape `us apply`
already walks. With no argument it walks every scenario; with ids it walks only those,
in ascending id order.

It is another `runner.Spec[I]` instantiation. Nothing about the walk, the report or
the fix loop is new; what is new is the item source, the subject type and the checklist.

## Decisions

Numbered as they were settled, so a question about any one of them has an answer here.

1. **Checklist source is `us-refine` only.** Not `us-create` — its nine prompts check
   the "As a… I want… so that…" sentence, which a scenario does not have.
2. **Surface**: `scen check` with no argument walks all; variadic ids walk a subset.
   `--fix` is registered but refuses at startup, because the checklist ships no `F:`
   and `runner.validateFixTemplates` already refuses that combination.
3. **Exit code 0 on a failed walk**, like the whole `us` family. See the ADR.
4. **An unresolvable id is a startup refusal** — `Cannot start: …`, nothing walked, no
   paid turn. Same for a duplicate id in the argument list.
5. **The prompt sees only the scenario**, never the registry file. See the ADR.
6. **8 of the 12 `us-refine` prompts survive** — the table is below.
7. **No net-new prompts this task.** `service:` and `path:` validity is deterministic
   and belongs in Go, not in an AI turn. Deliberately left undone.
8. **Nothing is inherited.** `scen-check.yaml` is standalone with all 8 prompts written
   out; the command gets its own template set, its own subject type, its own evaluator.
9. **No `nil` dependencies.** All six templates are written now, even though the three
   fix templates cannot run until a `F:` exists, and `runner.Run` refuses a `nil`
   `Evaluator`/`FixGenerator`/`FixApplier`.
10. **`Prompt` is the canonical term** for one checklist item, matching
    `validation_prompts:` and `PromptWithContext`. See `CONTEXT.md`.

## The prompt mapping

| # | `us-refine` prompt | `scen-check` |
|---|---|---|
| 1 | Count ACs, pass if 3–7 | dropped — thresholds over a set |
| 2 | `description` is rule-based (must/should, one sentence, no Gherkin) | kept, against `description:` |
| 3 | `steps` has at least one given / when / then | kept, against `merged_steps` |
| 4 | `description` free of `forbidden_qualifiers` | kept, doc pointer corrected |
| 5 | `steps` free of `forbidden_qualifiers` | kept, against `merged_steps` |
| 6 | steps free of `forbidden_actions` verbs | kept, against `merged_steps` |
| 7 | describes observable behaviour, not implementation | kept, with a carve-out |
| 8 | at least 1 AC is happy path | dropped — cross-item |
| 9 | at least 2 ACs are error scenarios | dropped — cross-item |
| 10 | at least 50% of ACs measurable | kept, threshold becomes binary |
| 11 | every AC has a binary outcome | kept |
| 12 | at least 80% convertible to a test sketch | dropped — `build tests` proves binding mechanically |

Prompt 7 carries a carve-out the `us-refine` original does not need: a CLI's observable
behaviour includes its exit code, stdout, the files it writes and the subprocesses it
spawns, so `the engine spawned exactly 1 test runner` is behaviour, not implementation.

**`docs:` keys**: `product` on prompts 4, 5, 6 and 7 only; `architecture_yaml` nowhere.
`forbidden_qualifiers` and `forbidden_actions` live at `docs/product/product.yaml:118`
and `:132` — the `us-refine` prompts that cite `architecture.yaml` for them are wrong,
and that pre-existing bug is not fixed here.

Prompt wording was deliberately written without review and is expected to be revised in
a later task. The focus of this one is the command and the walk.

## Steps

1. Write E2E-291…E2E-296 into `docs/scenarios.yaml`, `path:
   tests/bdd-cli/scen_check_test.go`; and E2E-297, the zero-prompt-checklist guard,
   into `build_tests_test.go` beside E2E-015. Fixture set, in order: happy path,
   validation fails, `--fix` refused, unknown id, empty registry, id filter — plus a
   seventh, `build-tests-empty-checklist`, for E2E-297.
2. `true-bdd build tests --fix` renders seven Go tests. **Zero AI turns** — every step
   these scenarios need already has a definition under `tests/bdd-cli/steps/`,
   including `stdout does not match (.+)` and
   `the "([^"]+)" checklist is narrowed to its "([^"]+)" prompt`.
3. The seven tests are red. The command does not exist. This is the intended state.
4. Implement: `true-bdd/checklists/scen-check.yaml`; six templates under `templates/`
   plus their `templates.prompts` keys in `true-bdd/true-bdd.yaml`; a
   `template.ScenarioCheckData` subject type; an evaluator in `bootstrap`;
   `cmd/scen.go` and `internal/app/commands/scen_check.go`; and two engine-wide guards
   in `runner.Run` — a checklist with zero prompts, and a `nil` generator dependency.
5. Build six fixture trees under `tests/bdd-cli/fixtures/scen-check-*/`. Each ships its
   own tiny `docs/scenarios.yaml` of one to three scenarios, never the real 290-entry
   registry. The three that actually walk — happy path, validation fails, id filter —
   each carry a `checklist-prompts.yaml` narrowing to prompt 2, which declares no
   `docs:`, so those trees need no `product.yaml` at all. Without that narrowing the
   checklist declares `product`, `validateRequiredDocs` refuses at startup, and the
   test measures the wrong refusal. E2E-297 needs a seventh tree,
   `build-tests-empty-checklist`, whose `input/` ships a zero-prompt variant of
   `true-bdd/checklists/build-tests.yaml` — fixture files win the overlay, and
   `true-bdd/` schemas are host lint contracts the engine never reads. It cannot be
   expressed as an empty `checklist-prompts.yaml`; the harness refuses that file shape.
6. `go test -tags bdd -run '^TestE2E29[126]$' ./tests/bdd-cli/ -mode=record` against a
   live key. Three fixtures pay: E2E-291 one turn, E2E-292 one turn, E2E-296 two turns
   (two scenarios walked, one prompt each — that count is the scenario's own
   assertion). The other four refuse before any AI turn and have no cassettes. This is
   the only paid step.
7. `-mode=replay` green, `./scripts/lints.sh`, `go test ./...`, `golangci-lint run`.

## Traps

- **`Spec.StoryNumber` must stay empty.** `runner.validateStoryNumber` expects the `4.1`
  shape and would reject `E2E-291`. Scenario ids travel in their own field.
- **Walk order must be ascending id.** Replay matches cassettes by sequence per binary,
  so a nondeterministic order breaks replay. `build tests` already emits in id order.
- **The empty-registry refusal is free** — `registry.RegistryLoader` already produces
  `requirements registry has no scenarios to walk`, pinned by E2E-015. Load through it.
- **The zero-item guard must not run after `Prepare`.** `build tests` legitimately
  narrows to zero items on a converged repository, and that is success.
- **Between steps 2 and 6 the repository has seven red BDD tests**, so `gates.sh` is
  red. Expected. The branch is not green until step 7.

## Also in scope

`README.md` gains the subcommand and the six new `templates.prompts` keys in its
configuration reference; `CLAUDE.md` gains the subcommand in its CLI Subcommands block.
`docs/doc-universe.md` and `.html` are untouched — no new document is declared — but
confirm that by running `sync-doc-universe`, not by eye.

## Explicitly not in scope

Adding `F:` templates; deterministic `service:`/`path:` checks; fixing the `us-refine`
document pointer; running `scen check` over the real registry in any gate; and any
non-zero exit variant. No follow-up tickets were filed for these — this section is
the record.

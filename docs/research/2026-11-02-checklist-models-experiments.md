# Checklist models experiments, 2 November 2026 (#288)

Recommend **V2: JSON Schema 2020-12 written in YAML, with a short construction rule at the scheme root, validated in process by `github.com/santhosh-tekuri/jsonschema/v6@v6.0.2`**. The type-only schemes lost an `and` modifier. Adding the construction rule restored byte-identical system and user prompts on **3, then 10, then 11 real Cells**. This is evidence for a model boundary and its reconstruction contract, not a completed cut-A rebuild or a statistical reliability claim.

The literal requested experiment also exposes an input gap: a Subject source cannot identify its Checklist Q/F, document selection, resolved CLI or run artifact path. **Every variant scored 0/3 with only description, templates and Subject source.** Supplying the real non-Subject caller inputs permits the diagnostic comparison below. These two experiments have different evidence and must not be combined into a success rate.

**35 live `claude-opus-5` calls cost $3.199744 of $100.** All live commands finished in the foreground. Only this report is committed. No engine, template, test, cassette or Specification file changed; the supplied round-4 cut was read only.

## Results at a glance

“Identical” requires both complete rendered prompts to equal the saved UTF-8 bytes; there is no whitespace or semantic tolerance. Architecture bytes are the four-model block, with external schemes counted separately below.

| Variant | Architecture bytes | Restricted inputs: identical Cells | Source-complete diagnostic: identical Cells | Total new spend |
| --- | ---: | ---: | --- | ---: |
| V0 | 11,804 | 0/3 | 3/3 | $0.693260 |
| V1 | 1,057 | 0/3 | 2/3 | $0.511375 |
| V2 | 1,057 | 0/3 | 2/3; annotated scheme **11/11** | $1.540644 |
| V3 | 553 | 0/3 | 0/3 | $0.454465 |

V0 uses the existing prose; V1 uses the templates and a Yamale scheme; V2 uses the templates and a JSON Schema in YAML; V3 uses the templates alone. V0 and V3 have no scheme validator. V1 uses the experiment's restricted Go Yamale-to-JSON-Schema translator plus the same Go validator as V2. V2's first diagnostic is its unannotated scheme; the 11/11 result is the explicitly revised scheme, not 11 repeats of the original 2/3 design.

## Inputs, calibration and isolation

Branch: `impl/288-checklist-models`. Starting commit: `d301440a5c65aa5145554ae6d52877ae1a3bdc9d`. Raw artifacts remain under **`tmp/checklist-models-experiments/` in this worktree**: `inventory.json`, `cases.json`, `source-audit.json`, `calibration/`, `designs/`, `annotated-designs/`, `runs/`, `judgements/`, the Go bridge/translator, Python drivers, validation requests/results and aggregates. Each live run keeps its exact argv, system/user request, native JSON envelope, stderr, answer YAML, rendered prompts and diff. Costs come from native `total_cost_usd`, including cache accounting; historical cassette/round-4 costs are not new spend.

Readings included the saved #288/#249 tickets, both named Q&A logs including later corrections, the two earlier research reports, CLAUDE.md, the domain instructions, GLOSSARY and ADR-0035/0037/0038. The later Q6/Q7 decisions in the formats log govern: a model is YAML data, schemes check produced data in process, and only checklists are in scope. Earlier statements about validating whole rendered prose are superseded. Neither this report nor its recommendation revives #285's rejected compaction/digest choices.

The cassette census found **111 Story, 29 apply and 92 Scenario ordinary checklist turns** among 753 Claude cassettes. Eleven Cells were selected from original fixture inputs, retaining actual Q/F block chomping, document paths and normalized `tmp/{{RUN_DIR}}` paths. Their source fields were independently compared with the reference Subject: **11/11 matched**. No model saw the saved answer, reference model instance, target rendered prompt or engine implementation.

The bridge is a throwaway nested Go module under tmp that imports the real engine's `TemplateLoader[ChecklistPromptData]`, real Story/criterion/Scenario types and methods. It therefore uses the engine's Go `text/template`, Sprig and yaml.v3 `toYaml` override directly. It preserves Story struct field order, YAML indentation/chomping, modifier marshaling, lexical document-map order and literal leading/trailing template bytes. **Calibration passed 22/22 complete saved prompts**, system and user for all eleven Cells, before their comparison was accepted.

There is one scorer correction worth retaining: the first bridge insisted on raw backing fields for methods. V0 legitimately returned `FormatSteps` as an already formatted string. A plain Subject map exposes that string to the same engine template loader, so the bridge was corrected to accept either the actual typed backing state or explicit method-result fields for non-Story Subjects. Original typed-only scores/renders remain as `typed-only-*`; every existing answer was regraded without a model rerun. This changed V0's source-complete apply score to a pass. No missing step text was supplied by the scorer. Story still uses its actual typed YAML representation, because map serialization would change field order. These are explicit candidate model representations, not a claim that the current engine serializes methods.

Live calls used Claude Code 2.1.308, `claude-opus-5`, high effort, a custom data-reconstruction system prompt, empty setting sources, strict MCP, safe mode, tools disabled, advisor/CLAUDE.md/auto-memory/bundled-skills/workflow/background-task suppression and no session persistence. The request went through stdin. Each process had a $3 cap and a 180-second timeout; neither was reached. The shared output instruction specified YAML, top-level Go access names and Story snake_case. It did not supply hidden constructor rules.

### Why no ordinary build-steps or build-code fidelity score

Neither 753 fixture cassettes nor round 4's 43 saved user prompts contain the ordinary `# Step definition body` checklist prompt. The shipped body path is a Built-in check; grouping and pattern proposals instead bypass these templates. `build-code.checklist` reads no data and explicitly states that tests-pass is answered by the engine. A synthetic “real live turn” would be misleading.

All variants include the fourth, build-steps model design and size. Its scheme was exercised on one **constructed shape-only instance**, using round 4's actual `project tree` suite and Concept meaning, plus 17 mutations. It is not included in the eleven recorded Cells or presented as a captured ChecklistPromptData instance. Build-code's two no-field template declarations stay out of the four-model count. No fifth live model was invented. This archive gap limits the conclusion for the fourth model.

## Designs and what the calls established

The experimental model view retains top-level PascalCase template inputs. Story Subject uses the actual source Story mapping; apply Subject carries StoryID/StoryPath/ACID/LineageScenarioID/Description/RequirementsScratchPath and raw `Steps` arrays; Scenario Subject carries ID/Description/Service/Path, raw `Steps` and `UserStories`; build-steps projects Suite/Label/Concept.Means. Unread SubjectID/SubjectTitle and internal per-walk evidence are excluded from the scheme view. The model view is explicit: default yaml.Marshal on the whole current Go struct is not this wire representation. V0 can instead expose computed method strings, as its prose describes.

V1 and V2 declare required keys, types, nested shapes, optional Story feature/steps and strict unknown-field refusal. They share a 303-byte rendering rule: real Go templates, Sprig, yaml.v3 Marshal with its final newline, no final trim, ordered lists, sorted string map ranges and the same data for both templates. V3 has only template references. The four pairs' original templates are unchanged in every call.

### Restricted-input pilot: stop after three per variant

Each pilot used one Story Cell, one apply Cell and one Scenario Cell. It supplied only the variant description, both original templates and the real Subject source. All twelve returned data and all twelve failed the full byte comparison. Serial model-call time was 59–75 seconds per variant; no failing design was scaled for hours.

The missing information was observable, not merely formatting noise. Models invented Q/F text and document names/paths, guessed section names in ResultPath, and sometimes chose the wrong Structured branch. V1's apply answer also used ACID alone as lineage. The Subject does not determine any selected Checklist Prompt or the run identifier. Even V0's provenance prose explains how to use these values without revealing which values this Cell received. A scheme cannot recover absent inputs.

### Source-complete diagnostic: distinguish missing inputs from lost rules

The next three calls per variant additionally received exact caller values for Docs, Question, Rationale, FixTemplate, ResultPath and Structured, with the source's Host-relative path. Apply also received its actual runtime RequirementsScratchPath; a later multi-criterion source explicitly identified the selected criterion. These values were copied from the reference call, but **no expected Subject projection or expected rendered prompt was supplied**. This is a declared diagnostic supplement to the restricted-input experiment, not a silent relaxation of its result.

- **V0: 3/3.** Its prose supplies Story unwrapping, step flattening, modifier preservation and exact lineage behavior. Its explicit method-result representation is valid for those template reads.
- **V1: 2/3; V2 unannotated: 2/3.** Both fail the same apply Cell by changing `and the response body is ...` to `the response body is ...`. The raw source is an `and:` mapping. A list-of-strings scheme does not specify whether to retain that key when flattening. Both wrong instances pass their structural schemes. Stop the type-only designs after this pilot.
- **V3: 0/3.** Story output retains an extra `story:` wrapper; apply reuses the source's list-of-blocks as Steps, which cannot decode into the merged backing structure; Scenario output keeps `merged_steps` rather than the Steps its method needs. Templates name reads and methods but do not establish their input representation. Stop V3.

### V2 revision: a construction rule at the scheme root

The revised V2 retains the same validation shapes and adds one root `$comment` per scheme, with no per-property prose descriptions. For apply, the exact rule is:

> Construct Subject from the selected source criterion. StoryPath is the Host-relative source path. LineageScenarioID is StoryID + "-" + ACID. RequirementsScratchPath is the supplied runtime path, unchanged. Flatten source steps by Given, When, Then, retaining within-block order; turn each modifier mapping into its key, one space, then its value (including and/but), rather than dropping the modifier. Omit no source statement.

The Story rule fixes unwrapping/exclusions and typed YAML; the Scenario rule fixes map-key identity, modifier flattening and ordered lineage projection without merge_date. Build-steps has a shape-only rule and explicitly records its untested dispatch status. **The revision passed 3/3, then 10/10, then 11/11**; the eleventh Cell specifically exercises nonempty lineage. The second stage adds seven distinct Cells rather than rerunning the pilot three. Both prompts match on all eleven, and all eleven produced instances pass V2's in-process scheme.

The 11/11 result is 4 Story, 3 apply and 4 Scenario Cells, including Feature preservation, multiline Q/F scalars, document-map rendering, `and` flattening, a selected criterion whose id differs from its position, and both empty/nonempty lineage. It does not cover every whitespace edge, every schema override, sketches' dynamically replaced Q, arbitrary host data or a live cut-A generator. Root comments guide a model rebuilding constructors; JSON Schema does not execute those comments. The production engine still needs deterministic constructors and checks on actual constructed values.

## In-process scheme checks

V2 uses **`github.com/santhosh-tekuri/jsonschema/v6@v6.0.2`**, compiling a draft-2020-12 schema loaded from YAML with yaml.v3 and validating JSON-compatible values. The bridge uses local schemas with no external schema retrieval. V1 uses **`yamale.go` in this experiment**: a small Go parser/translator for the used `str`, `bool`, `int`, `null`, `list`, `map`, `include`, `any` and `required=False` forms, strict objects and includes, then the same JSON Schema compiler. Unsupported expressions refuse. This is a tested subset, not a general Yamale Go port or a claim of complete Yamale compatibility. No `yamale` command was spawned.

For **each** V1/V2 scheme set, all **11 reconstructed real input views pass**, and **311/311** ordinary-instance mutations are rejected: missing and wrong-type required top-level and nested fields, plus unknown top-level keys. The separate build-steps constructed view passes and **17/17** mutations fail, for **12 positive views and 328 negative views per validator** including that non-live sample. Optional feature omission is accepted. The revised V2 changes annotations only and was additionally checked against all eleven generated answers.

Every original V1 and V2 generated answer also passes its scheme, including all six restricted-input hallucinations and both `and`-dropping diagnostic instances. A deliberately wrong but well-typed ResultPath also passes both validators. This is the exact limit of the claim: a scheme catches absent keys and incompatible types, not provenance, identical prompts or the correct string value. Runtime validation complements the retained behavior requirements; it cannot replace them.

Prefer V2 because it requires an existing general Go validator instead of owning a Yamale language subset. V1's scheme files are smaller; that does not remove the translator's compatibility and maintenance work. No model-quality advantage of JSON Schema over Yamale was established by their equal 2/3 shape-only pilots.

## Bytes and the all-32 estimate

Counts are UTF-8 bytes. Reference blocks use standalone canonical YAML before service nesting; the existing V0 bodies retain their original indentation. Whole-file indentation and retained-anchor handling are not charged to those candidate blocks. The actual architecture is **122,452 bytes**. Its on-disk prompt_data entry bodies occupy **52,476 bytes**; serializing all resolved values canonically produces **127,530 bytes**. YAML aliases explain much of the difference. Counting the latter as physical architecture bytes would overstate the cut.

The selected eight user/system entries occupy **11,804 physical bytes**: Story 6,545 + 128, apply 1,756 + 102, Scenario 1,871 + 107, build-steps 828 + 467. The shared rendering anchor is counted once. A canonical alias-expanded four-model block is 29,048 bytes.

| Description | Four-model architecture block | External four-model schemes | Combined candidate bytes |
| --- | ---: | ---: | ---: |
| V0, actual existing entries | 11,804 | 0 | 11,804 |
| V1, references and shared rendering | 1,057 | 3,248 | 4,305 |
| V2, unannotated | 1,057 | 9,007 | 10,064 |
| V2, tested annotated revision | 1,057 | 10,371 | 11,428 |
| V3, template references | 553 | 0 | 553 |

The annotated V2 architecture block is **91.0% smaller** than the selected physical prose block. Its combined files are only **3.2% smaller** in this deliberately self-contained design, whose schemas repeat unused shared definitions. Thus this is primarily a much smaller architecture entry plus machine-checkable external contracts, not a 91% reduction in all contract bytes. Against alias-expanded V0, the combined reduction is 60.7%. No schema deduplication saving is silently credited.

For all 32 templates, an explicit 16-paired-model reference block measures **3,921 bytes** for V1/V2 and **2,639** for V3. Keeping the older one-entry-per-template layout instead measures **5,267 / 5,267 / 2,912**. These are reference-block encodings, not quality trials of the other roles. A rough four-times extrapolation of this sample's scheme bytes gives V1 12,992 bytes, V2 base 36,028 and V2 annotated 41,484; paired V2 plus schemes would be about 45,405 bytes. The other roles are larger and structurally different, so this is a sizing scenario, not a forecast or recommendation to migrate them now.

These are measured candidate blocks, not a complete replacement architecture patch. Removed entries own YAML anchors that surviving fix/applier entries still reference. A later implementation must preserve those surviving values and repair anchor ownership; mechanical alias expansion can change the whole-file physical delta. Concept judgement procedures currently mixed into the rendering anchor also need to remain with their existing contracts, including ADR-0037's corrected shared-binding rule. Their deletion was not validated here.

## Judgement payloads: size and scheme only

The request payloads are built by `concept_evaluator.go` through `json.MarshalIndent`, outside the templates. The table measures their **user JSON bytes only**, excludes the Q/system instruction and counts recorded occurrences rather than unique semantic requests. Round 4 contributed four groups, nineteen shared bindings and nineteen proposals; fixture cassettes supply the remaining shapes. No live judgement was requested and new judgement spend is $0.

| Payload | Fixture + round-4 occurrences | Min / median / max bytes | YAML JSON Schema bytes | Real passes; missing/type mutations caught |
| --- | ---: | ---: | ---: | --- |
| group | 30 + 4 | 607 / 757.0 / 140,963 | 534 | 34/34; 306/306 |
| link | 74 + 0 | 671 / 1,401.5 / 3,573 | 540 | 74/74; 666/666 |
| binding | 84 + 19 | 1,612 / 2,468.0 / 156,971 | 1,209 | 103/103; 1,133/1,133 |
| words | 15 + 0 | 1,215 / 1,680.0 / 2,037 | 579 | 15/15; 90/90 |
| propose | 21 + 19 | 1,451 / 2,693.0 / 69,966 | 1,064 | 40/40; 720/720 |

All **266/266** payloads pass and **2,915/2,915** mutations fail in process with the same Go validator. The schemes hold strict envelope fields, collection types and step/concept shapes. Embedded product is a string; Public interface and architecture binding interiors are only required to be objects, and refused proposal pattern interiors are not fully constrained. This is intentionally a request-shape check, not validation of the product text or every interface format, and not the engine's existing dynamic answer schemas. No fidelity, judgement-quality or prompt-transport conclusion is drawn from it.

## Concrete recommendation for a later implementation

Place these four paired entries on the existing service, independently of `engine.models` AI tier bindings:

```yaml
services:
  - name: true-bdd
    models:
      story:
        templates:
          system: templates/us-checklist.system.prompt.tpl
          user: templates/us-checklist.prompt.tpl
        scheme: true-bdd/models/story.schema.yaml
      apply:
        templates:
          system: templates/us.apply-checklist.system.prompt.tpl
          user: templates/us.apply-checklist.prompt.tpl
        scheme: true-bdd/models/apply.schema.yaml
      scen:
        templates:
          system: templates/scen-check.checklist.system.prompt.tpl
          user: templates/scen-check.checklist.prompt.tpl
        scheme: true-bdd/models/scen.schema.yaml
      steps:
        templates:
          system: templates/build-steps.checklist.system.prompt.tpl
          user: templates/build-steps.checklist.prompt.tpl
        scheme: true-bdd/models/steps.schema.yaml
```

The names above are the exact prototype names. A later naming change can use existing checklist configuration keys without changing the model's meaning. Keep one short shared rendering rule, as measured, and place the necessary source-construction recipe at each scheme's root. Do not restore field-by-field prose. Both user/system templates share the model even when the system reads only Structured or reads nothing. Whether to split those into per-template declarations remains a design choice; this experiment needs no empty system scheme.

In the **architecture**, replace only the eight checklist prompt_data entries with these four models, declare the Go validator library and its build graph at the tested versions and preserve all unaffected roles and anchor values. Retain the short rules that decide bytes: no trimming, YAML source decoding/chomping, document selection/path preservation, source-to-Subject projection, artifact-name construction and Structured resolution. The experiment supplies resolved caller values, so it does not prove those production rules can be discarded. Do not delete the non-template concept protocols along with a rendering alias. Amend the relevant architecture document schema to allow service models in the later change.

In the **engine**, add explicit source-to-model constructors or validation views, encode/check the produced YAML-compatible data in process before rendering and before any AI turn, compile/cache schemes, and report validation errors with model identity and field path. Preserve the actual Story codec and method semantics; do not round-trip Story through an unordered map before toYaml. Resolve scheme paths under the host's resource contract, validate host overrides against their declared models and preserve the same populated input for system/user templates. Keep the renderer and AI answer contracts unchanged. This adds deterministic validation, not a model turn. The exact scheme view must be deliberate because current Go default YAML names and method serialization do not match the candidate's PascalCase view.

In **init**, ship the four schemes beside the existing templates, declare their resource paths, write missing files while preserving host-owned existing files, and include the same resources in generated defaults and drift checks. Runtime must validate the host's scheme, not silently substitute a packaged default for a host edit. The implementation's init fixture/golden will need the new resources; unchanged checklist prompt bytes do not require rerecording all historical AI turns. None of this production work is part of #288.

This recommendation preserves ADR-0035/0037/0038's pipeline and judgement boundaries. The stale build-steps binding clause in the old shared rendering anchor must not override ADR-0037. Do not broaden the change to fix generators, appliers, critique or general file formats. A later cut-A rebuild with the original replay oracle remains the end-to-end acceptance check.

## Runs, spend and stopping decision

| Batch | Calls | Identical Cells | New USD | Serial model-call seconds |
| --- | ---: | ---: | ---: | ---: |
| `restricted-V0` | 3 | 0/3 | $0.330880 | 74.518 |
| `restricted-V1` | 3 | 0/3 | $0.234350 | 59.329 |
| `restricted-V2` | 3 | 0/3 | $0.261745 | 66.089 |
| `restricted-V3` | 3 | 0/3 | $0.226545 | 72.961 |
| `complete-V0` | 3 | 3/3 | $0.362380 | 63.347 |
| `complete-V1` | 3 | 2/3 | $0.277025 | 74.697 |
| `complete-V2` | 3 | 2/3 | $0.294295 | 64.584 |
| `complete-V3` | 3 | 0/3 | $0.227920 | 52.487 |
| `annotated-V2` | 11 | 11/11 | $0.984604 | 212.768 |

Total **$3.199744**, leaving **$96.800256**. Per variant totals include its failed pilots, diagnostics and V2's revision; no unsuccessful call is omitted. The eleven annotated trials cost $0.984604. Native usage, including cache reads/writes and output tokens, remains in each saved envelope; model-call seconds exclude local preparation and Go checks. There were no timeouts, budget terminations or unavailable-model failures.

The stop points are decisive for this scope: restricted inputs fail for missing caller facts; template-only data loses structure; type-only schemes lose a modifier; the annotated scheme recovers it and survives the broader sample plus targeted lineage case. More identical calls would not answer the remaining architectural question or certify cut A. V0 was kept as a three-Cell source-complete control, not scaled into a reliability study. No full pipeline, fix generator, applier or critique was run.

## Owner decisions and remaining limits

The actionable choice is **V2 with a short root construction annotation**, accepting that a purely structural scheme does not contain all byte-deciding knowledge. The Owner should decide whether that root annotation is an acceptable home for the source-construction recipe, or whether all such behavior must instead be stated by retained Scenarios/other existing contracts. The latter would need a separate source-complete reconstruction experiment before deleting those rules. Moving the rules invisibly into a helper is not evidence they became unnecessary.

A second choice is to adopt four paired checklist models as shown, or one declaration per template referencing shared schemes. The pair is smaller and matches today's shared data. The fourth model still needs a real ordinary checklist dispatch if its byte fidelity is to be claimed; none exists in the supplied archive. No answer was requested or assumed during this run. Fix/applier/critique models remain out of scope.

## Repository verification and divergence review

`go run ./scripts/cmd/alint` passed on the throwaway Go files and on this report. `go run ./scripts/cmd/gates run --changed main` passed, exit 0, all nine selected gates; the inherited branch diff from main selected the full table, including build/test in all three modules, BDD vetting/coverage guards, release rehearsal, init-defaults drift and the recordings scan. Full output is `tmp/checklist-models-experiments/gates.log`. `git diff --check` passed. Alint was rerun after recording these verification results. No temporary Go file was placed in services or tests, so the task itself does not require a fresh BDD replay.

The Specification divergence review finds no new runtime behavior, code/Scenario contradiction or new domain field in the committed scope: this commit is research only. The proposed model view, schema-loading behavior, resource/init changes and library declaration above are explicitly future work, not claims about the current engine. This experiment changes no protected gate configuration and leaves its raw artifacts ignored.

## Selected real Cells

The final row was added after the ten-Cell stage specifically for nonempty lineage. Paths identify the exact cassette, whose argv/stdin is the reference; source file paths and reference data are in cases.json. Sizes below are saved system plus user UTF-8 bytes.

| Cell | Reference cassette | System + user bytes |
| --- | --- | ---: |
| `story-us-refine-shipped-no-failure` | `tests/true-bdd/fixtures/us-refine-shipped-no-failure/cassettes/claude-001/meta.json` | 2,585 + 4,620 |
| `story-us-refine-fix-keeps-feature` | `tests/true-bdd/fixtures/us-refine-fix-keeps-feature/cassettes/claude-001/meta.json` | 2,585 + 4,766 |
| `story-us-refine-fix-binary` | `tests/true-bdd/fixtures/us-refine-fix-binary/cassettes/claude-001/meta.json` | 2,585 + 4,183 |
| `story-us-refine-fix-implementation` | `tests/true-bdd/fixtures/us-refine-fix-implementation/cassettes/claude-001/meta.json` | 2,585 + 4,216 |
| `apply-us-apply-duplicate-collapse` | `tests/true-bdd/fixtures/us-apply-duplicate-collapse/cassettes/claude-001/meta.json` | 2,076 + 5,111 |
| `apply-us-apply-key-apart-from-lineage` | `tests/true-bdd/fixtures/us-apply-key-apart-from-lineage/cassettes/claude-001/meta.json` | 2,076 + 4,958 |
| `apply-us-apply-names-service` | `tests/true-bdd/fixtures/us-apply-names-service/cassettes/claude-001/meta.json` | 2,076 + 4,768 |
| `scen-engine-claude-minimal` | `tests/true-bdd/fixtures/engine-claude-minimal/cassettes/claude-001/meta.json` | 2,191 + 3,045 |
| `scen-scen-check-reverse` | `tests/true-bdd/fixtures/scen-check-reverse/cassettes/claude-001/meta.json` | 2,191 + 2,935 |
| `scen-scen-check-concurrent` | `tests/true-bdd/fixtures/scen-check-concurrent/cassettes/claude-001/meta.json` | 2,191 + 2,935 |
| `scen-scen-check-happy-path` | `tests/true-bdd/fixtures/scen-check-happy-path/cassettes/claude-002/meta.json` | 2,191 + 3,179 |

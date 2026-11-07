# Step argument caller matrix

Every row is a runtime turn family, including retries and clarification-driven regenerations. Template paths below are under `templates/`. The concept protocols use the Q text in the named checklist rather than a template. Read-enabled means Read/Glob/Grep are available; it does not imply architecture access. No new read or write grant is needed. Build commands select evidence from their actual architecture path, including --architecture; flow-style sections and aliases retain every decoded value. `step_arguments` is optional at the architecture document's top level. Its absence emits zero bytes in every transport.

| Command | Turn | Template or Q | Read-enabled | Present architecture access | Declared Model | Transport |
|---|---|---|---|---|---|---|
| us refine | ordinary validation | us-checklist.prompt.tpl | yes | requested Docs, if selected | us_refine_checklist | Model StepArguments: named section by existing path, otherwise whole text |
| us refine | sketch validation | us-checklist.prompt.tpl; us-refine.yaml sketch Q | yes | deliberately none after sketch projection | us_refine_checklist | Model StepArguments: whole text |
| us refine | concept link | us-refine.yaml link Q | yes | none; product embedded | no | whole text in protocol payload |
| us refine | concept words | us-refine.yaml words Q | yes | none; product embedded | no | whole text in protocol payload |
| us refine | fix proposal, clarification, refinement | us-checklist.fix-generator.prompt.tpl | yes | DocPaths when selected | no | named section by existing path, otherwise whole text |
| us refine | application | us-checklist.fix-applier.prompt.tpl | yes | none in the Story applier | no | whole text |
| us apply | validation | us.apply-checklist.prompt.tpl | yes | requested Docs, if selected | us_apply_checklist | Model StepArguments: named section by existing path, otherwise whole text |
| us apply | fix proposal, clarification, refinement | us.apply-checklist.fix-generator.prompt.tpl | yes | DocPaths when selected | no | named section by existing path, otherwise whole text |
| us apply | application | us.apply-checklist.fix-applier.prompt.tpl | yes | none; scratch Registry only | no | whole text |
| scen check | ordinary validation | scen-check.checklist.prompt.tpl | yes | requested Docs, if selected | scen_check_checklist | Model StepArguments: named section by existing path, otherwise whole text |
| scen check | concept link | scen-check.yaml link Q | yes | none; product embedded | no | whole text in protocol payload |
| scen check | shared concept words | scen-check.yaml words Q | yes | none; product embedded | no | whole text in each shared protocol payload |
| scen check | shared binding judgement | scen-check.yaml bindings Q | yes | binding/interface embedded; product by path, architecture not by path | no | whole text in each shared protocol payload |
| scen check | fix proposal, clarification, refinement | scen-check.fix-generator.prompt.tpl | yes | DocPaths when selected | no | named section by existing path, otherwise whole text |
| scen check | application | scen-check.fix-applier.prompt.tpl | yes | architecture working copy for applicable fixes only | no | named section by existing working-copy path, otherwise whole text |
| build steps | ordinary validation (reserved evaluator seam; current walk uses code Q) | build-steps.checklist.prompt.tpl | yes | requested Docs, if selected | build_steps_checklist | Model StepArguments: named section by existing path, otherwise whole text |
| build steps | concept grouping | build-steps.yaml group Q | yes | none; product embedded | no | whole text in protocol payload |
| build steps | shared binding judgement | scen-check.yaml bindings Q | yes | binding/interface embedded; product by path, architecture not by path | no | whole text in each shared protocol payload |
| build steps | pattern proposal and reproposal | build-steps.yaml propose Q | yes | binding/interface embedded, no architecture path | no | whole text in protocol payload |
| build steps | body proposal, clarification, refinement | build-steps.fix-generator.prompt.tpl | yes | DocPaths when selected; semantic core embedded | no | named section by existing path, otherwise whole text |
| build steps | body application | build-steps.fix-applier.prompt.tpl | yes | semantic core embedded, no architecture path | no | whole text |
| build steps | verification of definitions (critique) | build-steps.critique.prompt.tpl | no | filtered semantic core embedded | no | whole text, within router byte limit |
| build code | repair planner, clarification, refinement | build-code.fix-generator.prompt.tpl | yes | Subject.Documents and DocPaths | no | named section by existing architecture path |
| build code | writer | build-code.fix-applier.prompt.tpl | yes | Subject.Documents | no | named section by existing architecture path |

Counts: us refine 6; us apply 3; scen check 6; build steps 7 (one reserved Model seam); build code 2. Total 24 rows. Shared runtime implementations are intentionally listed for each command using them.

The template inventory and provider dispatch sites were searched for additional step readers. `build tests` has no AI turns. Document checks have no step-reading AI turn. The test runtime's judge is outside the engine: its contract is clauses and diff, not Scenario steps, and this change does not add Host-project semantics to it.

# `build steps` writes one step definition per Catalogue phrase, from the Phrase mapping

**Superseded by [ADR-0035](0035-build-steps-writes-definitions-per-concept.md).** Concepts replace the Catalogue's templates ([ADR-0034](0034-concepts-replace-catalogue-templates.md)), so a definition's pattern no longer comes from a template. Historical rationale follows; the steps are still designed, not derived from Scenarios, and the engine still writes the skeleton.

The live run of 28 October had to invent the operation behind `the number is halved`. The Catalogue said what the phrase means, and the Word mapping mapped its one term, `number`. Nothing said which operation of the calculator's Public interface the step performs. Before that, `build tests` walked Scenarios with an unbound step, and its fix prompt asked the model to bind about ten neighbours' steps as well, to save cycles.

We decided:

- **The steps are designed, not derived from Scenarios.** The Catalogue in the product document is the designed set of operations and checks the system offers. A Scenario that fits no phrase fails. Then a phrase is added or the Story is corrected, and `us refine` and `us apply` run again.
- **The Phrase mapping.** The architecture document maps every Catalogue phrase to what a step of that shape does through the Public interface:
  - for a `when` phrase, the operation it calls;
  - for a `given` phrase, how it sets up state;
  - for a `then` phrase, what it observes and compares with.

  A phrase without an entry fails a `D:` check before `build steps`, and the System Architect adds the entry. The Word mapping still covers the terms.
- **One Item per missing definition.** An Item of `build steps` is a Catalogue phrase that a suite has no step definition for. Its definition is written once. The writer gets the phrase's meaning, its Phrase mapping entry, its terms' glossary entries and Word mappings, the service's Public interface, and every Scenario of the suite that uses the phrase. The run without the target and the critique pass (ADR-0031) are judged over those Scenarios.
- **The engine writes the skeleton, the AI writes the body.** The engine derives each definition's pattern from its phrase's template, the same compiled pattern the Closure check uses, and the function's signature from the template's slots. The AI writes only the body: the call through the Public interface and the check, as the Phrase mapping says.

## Considered Options

- **An Item per Scenario with an unbound step**, as before. One phrase shared by several Scenarios gets walked from one of them, and the prompt has to reach for the neighbours.
- **An Item per suite**, one plan for every missing definition, like `build code`'s Failing set.
- **The AI chooses the operation** from the phrase's meaning and the Word mapping, and the critique judges it. Or the Word mapping's `observed_by` names the operation, which makes every operation a term.
- **The AI writes the pattern too**, checked afterwards by the Closure check. The pattern is fully set by the phrase, so this only adds a way to get it wrong.

## Consequences

- TrueBDD's 76 Catalogue phrases need a Phrase mapping entry each before its own suite goes through `build steps`.
- A phrase that the Public interface cannot serve is found when the Specification is checked, not halfway through a build.

Grilling log: `docs/qna/20261028-094259-build-tests-new-behaviour.yaml`, Q12, Q13, Q14 and the entry after Q13.

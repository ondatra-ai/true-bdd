# A concept is one Public interface item and kind of use, with a closed meaning in declared words

ADR-0034 replaced the Catalogue's templates with concepts in prose and left #254 to decide how to write them. Spike #255 wrote 19 provisional concepts for TrueBDD's Registry and measured them. 19 concepts folded all 76 templates, and the link pass linked 3,104 of 3,108 Registry steps at 9. Every systematic error came from one overlap: a concept defined by what a step means to the role ("what the engine log records") reached the same steps as a concept defined by what the step's definition does ("read files by glob").

We decided:

- **What decides a step's concept is what its step definition does, as the step's wording fixes it.** A concept is one way of putting state in place (Given), calling (When), or observing and comparing (Then) through the Public interface. The target an argument names never changes the concept: `every file matching "tmp/true-bdd.log.json" matches …` is a file-content step, even though it reads the engine log.
- **One concept per Public interface item and kind of use, per block.**
  - The item is what the use is about: the project tree, the engine's commands, the engine log. A binding may reach it through several declared references (ADR-0035). For example, the project tree's Given concept references the project tree, the configuration and the checklists, and a role running a command references every command.
  - The variations of that use are the concept's patterns and captures, not concepts of their own: the steps' shapes, their alternatives (matches or does not match, a kind out of a set) and their values. For example, `engine activity` covers test runners, AI turns and the build identity, since all of them are read from the engine log and compared.
- **A new concept only for a new item or kind of use.** A step that needs a Public interface item, or a kind of use of one, that no concept covers justifies a new concept. Any other new variation widens the concept that already covers that item and use, and then its binding. What keeps the count down is the number of these pairs a product's steps reach.
- **A Given concept states a state.** A Given step worded as an action is reworded as the state it puts in place (GLOSSARY, Given).
- **A concept's `means` is closed.** It names the interface item, the kind of use and every variation the concept admits, in prose; the shape stays `concept`, `means` and `terms`. A step whose variation `means` does not state does not link, and its fix widens the concept at the menu, where the Product Owner validates it before any step is written (ADR-0034).
- **A `means` relies only on its declared `terms` and plain English.** Any other domain word is declared, with a glossary entry and a Word mapping, or reworded in product words. A model Prompt of its own judges the `means` of every concept an Item links to for undeclared domain words, in `us refine` per Story and in `scen check` per Scenario. Its fix is the Product Owner's: a declared term (a glossary entry and the concept's `terms`) or a reworded `means`. The Word mapping a newly declared term needs is the System Architect's, through `scen check`'s existing term-mapping check, so `us refine` still never waits for the architecture.

These rules are the ones a later change of mind would cost a migration of every host's concepts. A decision that is cheap to change later stays provisional until it is seen working: what a concept is called, which concepts TrueBDD declares, and how a binding is worded.

## Considered Options

- **A concept per kind of state, action or outcome, as the step means it to the role.** This is how the spike wrote its concepts. The prose of two such concepts can claim the same step, and nothing deterministic sees it.
- **A concept per domain object, as Cucumber groups its step definitions.** The engine log, the Story, the Registry and the project tree would each be a concept. A generic step shape such as `every file matching …` would then have to be split by the object its argument names.
- **A concept per distinct operation.** About 50 to 55 concepts for TrueBDD, close to the 76 templates the Owner set out to avoid. Sibling concepts on one interface item would differ only by a pattern detail, not by meaning.
- **An open `means`, its variations only examples.** A new variation would reach the step definitions' code without the Product Owner or the System Architect seeing it.
- **No check of a `means`'s words beyond the declared `terms`.** The spike's concepts relied on six words the glossary does not define, all already used by today's Catalogue.

## Consequences

- #257's link Prompt reads `means` as closed, and its fix offers a wider concept unless the step needs a new item or use. The word-check Prompt is added to `us refine` and `scen check`.
- #258's grouping reads `means` the same way.
- In #259, TrueBDD's concepts start from the spike's 19:
  - `engine activity` covers only steps worded as the engine's own activity;
  - its 15 Givens worded as actions (`claude pipes …`) are reworded as states;
  - the six undeclared words (*fixture*, *stand-in*, *proxy*, *read root*, *staging file*, *hold*) are declared or reworded.

Grilling log: `docs/qna/20261029-131704-how-to-write-concepts.yaml`, Q1 to Q8 and the entries Claude settled as forced consequences.

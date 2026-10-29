# Concepts in prose replace the Catalogue's templates, and an LLM links steps to them at authoring time

Issue #248 found that `build steps` cannot tell a phrase's meaningful bracket alternatives apart: `[matches|does not match]`, `[validation|fix|apply]`, `[browser ]`. Fixing it within the template grammar meant more grammar (Cucumber's parameter types) or more templates. TrueBDD's Catalogue already holds 76 templates for 309 Scenarios, and grew by about one template per five or six new Scenarios. Large Cucumber projects report hundreds to thousands of step definitions. The Owner judged that if product concepts become too many, maintaining them costs more than writing the code, and the system loses its point

We decided:

- **Concepts, not templates.** The product document declares `concepts:` keyed by `given`, `when` and `then`. Each is a list of entries with `concept`, its `means`, and the glossary `terms` that meaning relies on. Concepts are prose, with no slots and no square brackets. A step is free text. A Given is a state, a When an action or event, and a Then an observable outcome (GLOSSARY).
- **An LLM links each step to exactly one concept of its own block,** with a score out of 10 and a reason. The pass mark is a checklist setting, 9 by default, and every verdict records its score and reason. A step two concepts reach at the pass mark fails as ambiguous, naming both, and a missing or malformed answer fails. A `judge:` clause is not linked. An `llm:` step links to a concept whose binding states intent (ADR-0033). The link is judged in three places, and each stops rather than carrying an error forward:
  - in `us refine`, against the product concepts only. A concept's architecture binding is the System Architect's and comes later, so `us refine` never waits for it;
  - in `scen check`, afresh for every Scenario, trusting no earlier result. The Scenario's concepts must also be bound for its service, and their bindings judged unambiguous enough for an LLM to write a step from;
  - in `build steps`, which stops before any skeleton when a step links to no concept or to two, or when a linked concept's binding is missing for the suite's service or is judged ambiguous.
- **Nothing stores the link.** A step names no concept and the Registry records none. The link is judged again wherever it is needed.
- **People validate concepts before any step is written.** When the LLM writes a step it is too late to involve a person. The Product Owner applies a new or wider concept at the `us refine` or `scen check` menu, and the System Architect applies its binding at the `scen check` menu.
- **Transition.** A Host project declares `catalogue:` or `concepts:`, never both, until TrueBDD's own concepts replace its templates and the template form is removed.

This departs from ADR-0015 on purpose. A model now judges where a structural check stood: `D: step-phrases` matched a step against its template with no model turn. The determinism that counts is kept at run time. The link is judged before any code exists, and tests run and replay with no LLM.

How to write concepts so that the system can use them deterministically afterwards is #254's.

## Considered Options

- **Capture every bracket group.** Grammatical alternatives such as `[s]` would become arguments too, and `[doubled|halved]` would be one phrase for two operations.
- **Cucumber Expressions' rule.** Brackets stay grammar only, a domain choice becomes a named parameter type, and negation becomes separate phrases. It is deterministic, but it keeps the Catalogue growing with every operation and wording.
- **An LLM at run time** ([Botrytis](https://github.com/sublayerapp/botrytis)), matching a step to a definition on every run. It pays per run, replays only from recorded choices, and a near match becomes a silently wrong test.
- **Templates kept, with an LLM folding paraphrases together.** The Catalogue still grows into the hundreds.
- **Recording each step's concept,** in the Story or in the Registry. It would be one more thing to maintain on every step.

## Consequences

- The template grammar of the self-spec log's Q34 is retired: quoted, number, role and pattern slots, and brackets.
- `us refine`'s and `scen check`'s `D: step-phrases` gives way to the link pass. `build tests` no longer refuses a step outside the Catalogue, since it runs no model and `scen check` stands before it.
- The closure checks keep their deterministic part over the documents. Every term a concept declares is defined and mapped, every binding names a declared concept, and every binding's references are declared in the Public interface.

Grilling log: `docs/qna/20261029-092034-meaningful-bracket-alternatives.yaml`, Q2, Q3, Q4, Q6, Q7, Q8, Q9 and the entries Claude settled on delegation.

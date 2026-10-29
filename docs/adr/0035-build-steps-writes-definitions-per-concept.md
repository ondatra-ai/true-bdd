# `build steps` writes step definitions per concept, from patterns the AI proposes and the engine checks

ADR-0032 derived each step definition's pattern from its Catalogue template, which ADR-0034 retires. The pattern needs another source, and one concept now serves many step texts: TrueBDD's single suite holds 391 distinct Given, 70 When and 881 Then texts.

We decided:

- **The architecture binds each concept.** `architecture.concepts` replaces `architecture.phrases`. An entry names the `concept`, the `service`, its `references` into that service's Public interface, and what a step definition does through them, by block, in `does`: a Given puts the state in place, a When calls, a Then observes and compares. An entry may state an `intent` instead, under ADR-0033.
- **An Item is a concept and a suite.** A grouping pass, run per suite and block, assigns the suite's steps that no definition binds to concepts of their block, or stops (ADR-0034). Before any skeleton, each grouped concept must be bound for the suite's service, and its binding is judged unambiguous again, as `scen check` judges it. A `judge:` clause and an `llm:` step get no definition.
- **Two passes per Item, with the engine keeping the skeleton.**
  1. The AI first proposes the concept's patterns as data.
  2. Before any body, the engine checks that every step of the concept in the suite binds exactly one pattern among the concept's existing definitions and the proposed ones, and that no proposed pattern binds a step that another definition or another Item's proposal binds.
  3. The engine then writes the skeleton: a registration, a function that takes the captures in order and records its concept, and a body that fails with "step not written yet".
  4. The AI writes only the bodies.

  A meaningful alternative, such as `(matches|does not match)`, is a capture of the proposed pattern.
- **The critique judges the patterns as well as the bodies.** It reads each against the concept's `means`, its binding, the glossary and Word mapping entries of its terms, the Public interface, and the Scenarios whose steps the pattern binds. A finding against a pattern sends the Item back to the proposal, and the engine checks the new pattern and rewrites its skeleton; a finding against a body sends the body back.
- `build steps`' whole-suite checks stay as #192 built them: Step coverage, the test list, and red without the target.

This supersedes ADR-0032. It amends ADR-0031: the critique now judges the AI's patterns and gets the Scenarios they bind. It amends ADR-0033: a phrase stated by intent becomes a concept binding stated by intent. ADR-0030 stands.

## Considered Options

- **The AI writes everything** (registrations, patterns, bodies), with the engine checking only afterwards. A bad pattern would show only after its body is written.
- **Cucumber snippets.** The engine derives patterns from step texts by rule. Unquoted words stay literal, so TrueBDD's free tails would give one definition per distinct step.

## Consequences

- A skeleton's identity is its concept and suite, and an Item may own several patterns. An interrupted walk resumes from the skeletons it finds.
- A rebuild from scratch may group steps differently and propose other patterns. The drill's oracle judges a rebuild by the behaviour it preserves, never by identical code.

Grilling log: `docs/qna/20261029-092034-meaningful-bracket-alternatives.yaml`, Q5, Q8, Q9 and the entries Claude settled on delegation.

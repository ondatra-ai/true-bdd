# A step acts on a UI by the names its Public interface declares, and a model runs only steps stated by intent

A step such as "the user clicks Save" is written before the UI's code exists (ADR-0031), so there is no DOM, no selector and no class name to find the button by. The harness has had model-run steps since August: `llm:` for a Given or When stated by intent, `judge:` for a Then about meaning. But nothing implements the `Actor` that an `llm:` step needs.

We decided on a hybrid:

- **Deterministic by default.** A UI service's Public interface declares the roles and accessible names of its elements, for example the button "Save" on the page "Order". A step finds an element by its role and name, as a user would. It runs with no model, at no cost, and stays red until `build code` builds the element.
- **A model only for intent.** A phrase whose Phrase mapping entry is stated by intent rather than by element, such as "close whatever dialog covers the list", becomes an `llm:` step. A model performs it through the harness's `Actor`, and its decision is recorded. Replay uses the recording, and a step with no recording fails offline.
- **`judge:`** stays for Then clauses about meaning.

## Rules for writing UI steps

1. **Find an element by what a user sees.** Use its role and accessible name, as the Public interface declares them. For example, `the user clicks "Save"` becomes `page.getByRole('button', { name: 'Save', exact: true }).click()`. Also allowed are `getByLabel` for a form field by its label and `getByText` for visible text. Such a step is written before the UI exists, runs with no AI and stays red while the button is missing.
2. **Never find an element by implementation.** No CSS class, element id, XPath, DOM path or test id that the Public interface does not declare. These are Internal code, and a rebuild may change them (ADR-0030).
3. **Name exactly, and keep names unique within their scope.** Use `exact: true`. When two elements share a role and name, the step scopes the search to the region the Public interface names, for example the dialog "Order", instead of picking the first match.
4. **Assert deterministically.** Counts, texts, states and an expected accessibility tree are compared by code. Playwright's `toMatchAriaSnapshot` may compare the page with an expected tree written from the Specification, never with one captured from the built UI. `judge:` is only for meaning that code cannot compare.
5. **Use `llm:` only where the Phrase mapping states intent.**
   - The model's decision, what it did and on which element, is recorded and replayed.
   - A missing recording fails offline.
   - The target's observations are never replayed as a pass: a Then still runs against the real UI.
6. **No self-healing that hides behaviour.** No step may skip, relax or rewrite itself when the element is missing. A missing element is the red that `build code` must turn green.

## Research

A Codex research run of 28 October compared four families of tools, all sources consulted that day:

- **User-facing locators chosen from a specification:** [Playwright locators](https://playwright.dev/docs/locators), the [Testing Library guiding principles](https://testing-library.com/docs/guiding-principles/), [ARIA snapshots](https://playwright.dev/docs/aria-snapshots) and the [W3C ARIA patterns](https://www.w3.org/WAI/ARIA/apg/patterns/button/).
  - They need no model, cost nothing and can be written before the UI.
  - Reported failures: [duplicate labels break strict matching](https://github.com/microsoft/playwright/issues/36045) (May 2025) and a [Testing Library label-resolution defect](https://github.com/testing-library/dom-testing-library/issues/1352) (March 2025).
  - Behaviour behind a role is still the code's to build: [ARIA semantics do not implement behaviour](https://www.w3.org/WAI/ARIA/apg/practices/read-me-first/).
- **Steps in natural language that a model runs every time:** [ZeroStep](https://zerostep.com/), [auto-playwright](https://github.com/lucgagan/auto-playwright) and [Shortest](https://github.com/antiwork/shortest).
  - Every run pays for inference, and none has a replay layer. auto-playwright's [cache request](https://github.com/lucgagan/auto-playwright/issues/42) is still open.
  - Reported failure: ["No valid target found"](https://github.com/zerostep-ai/zerostep/issues/46).
- **Resolve once with a model, then replay from a cache:** [Stagehand caching](https://docs.stagehand.dev/v3/best-practices/caching) and [Midscene caching](https://midscenejs.com/caching).
  - Midscene never caches its AI assertions.
  - Users report [poisoned caches and silent wrong actions](https://github.com/web-infra-dev/midscene/issues/2529) (May 2026), and Stagehand's changelog records stale caches.
- **Agents that write ordinary tests:** [Playwright test agents](https://playwright.dev/docs/test-agents) (planner, generator, healer).
  - Generated tests run with no model.
  - The healer may skip broken functionality, which conflicts with tests that must stay red until `build code`.

What the run concluded: a model cannot find an element that does not exist yet, so before `build code` every UI step is red whatever runs it. A model helps only to find an element that exists but that the Specification did not name.

## Considered Options

- **Deterministic only.** Every step would need an element the Specification names, even where the product's behaviour is stated by intent.
- **A model runs every UI step**, as ZeroStep, auto-playwright and Shortest do. Every run pays for inference, and it buys nothing before `build code`.
- **Resolve once and cache**, as Stagehand and Midscene do. Caches go stale or wrong in silence.

## Consequences

- No current host has a UI, so this lands after the `halve` acceptance, with a UI host of its own.
- The UI kind's standard form in the architecture (ADR-0030) is its elements' roles and accessible names.

Grilling log: `docs/qna/20261028-094259-build-tests-new-behaviour.yaml`, Q18 and Q19.

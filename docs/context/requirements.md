# Harness
## A Developer should be able to call the identify-task skill (`.claude/skills/identify-task`) with a task idea supplied as its context.
## A Developer should receive requirements from the context archivist and identify-task skill that use approved subject terms, prefer the applicable end-user role for functionality, reserve system subjects for architecture or infrastructure, split task briefs into Product and System groups, and omit automated-test verification and documentation-update chores.
## A Developer should be able to use the identify-task skill to review a complete candidate requirement list one requirement at a time, see each candidate's full text and Developer-revealed or assistant-suggested label, and choose Keep, Drop, or Other to accept, reject, or reword it before receiving a task brief containing only the candidates they validate.
## A Developer should have the implement-task skill isolate planning and end-to-end test creation in separate Opus agents, production-code implementation in a separate Sonnet agent, and final review in a separate agent with Fable preferred.
## A Developer should receive an implementation plan from the implement-task skill that compares the current code with the task goal and defines the required end-to-end tests before production implementation begins.
## A Developer should have the implement-task skill refine its plan, end-to-end test implementation, and final solution through Codex critique loops grounded in the task goal and plan, incorporating every suggestion rated 7/10 or higher for relevance and stopping when none remain or after three rounds.
## A Developer should be able to complete the end-to-end-test phase with changes limited to end-to-end tests and the architectural startup scaffolding they require, with the test-author agent running the end-to-end tests it creates so required services can start and those tests fail because the new behavior is still absent.
## A Developer should be able to have the production implementation agent reproduce the intended red end-to-end test failures from the test-author phase before implementing production changes, and then make those tests pass without modifying any test files.
## A Developer should receive a research-backed, Codex-informed escalation whenever satisfying an end-to-end test appears to require changing it, and should retain the decision to pursue a code-only solution or approve a test change before implementation continues.
## A Developer should receive a final independent review that considers the task, plan, implementation challenges, solution changes, and gaps or weaknesses in the end-to-end tests.
## A Developer should be able to configure repository folder locations used by the implement-task skill and its agents from one shared context file, without hardcoded path values in the skill or agent instructions.

# System
## The true-bdd harness must use Redis as its state backend.

# Product
## A BDD System Architect should be able to connect the true-bdd CLI to the true-bdd harness deployed on Vercel.

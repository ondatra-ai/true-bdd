# Writing requirements

Canonical rules for phrasing and filing requirements — in task briefs
(`docs/tasks/<slug>.md`) and in the living requirements tree
(`docs/context/requirements.md`). Read this before drafting requirements.

## Format: Goal + Requirements

- **Goal** — the outcome in 1–2 sentences. What changes, for whom. Not how.
- **Requirements** — each is a behavioral **capability**: a `should`/`must`
  statement of observable behavior. Never a process or documentation chore
  ("verified by a test", "docs updated", "lint passes"), and never "how" (no
  library, structure, or algorithm).

## Product vs System — the one rule that matters

A requirement is filed under **Product** or **System** by *what kind of statement
it is*, not by which component it mentions.

- **System** holds ONLY **technology/infrastructure choices** — sentences of the
  form *"must USE \<X\>"*.
  Examples: *The true-bdd harness must use Redis as its state backend.* / *The
  true-bdd harness must run Redis via a docker-compose file in dev and tests.* /
  *The true-bdd harness must deploy on Vercel.*
- **Everything else is a Product capability**, written from the **role's**
  perspective ("A BDD System Architect should be able to…"), EVEN when it
  describes the harness's behavior, correctness, protocol, liveness, or access.
  Behavior and correctness are Product — never System.

**The test:** if a sentence under System says what the system DOES or how well it
behaves, it is misfiled. Rewrite it as a Product capability from the role's
viewpoint. **When in doubt, Product.**

### Examples

❌ **System** (wrong — this is a capability, not a technology choice):

> The true-bdd harness must keep the register/poll/reply protocol correct across
> serverless invocations.

✅ **Product** (same intent, from the role's perspective):

> A BDD System Architect should be able to complete a dispatched run through the
> Vercel harness even though each HTTP request is a separate serverless
> invocation.

✅ **System** (a genuine technology choice):

> The true-bdd harness must use Redis as its state backend.

## Subjects

Every requirement's subject must be an exact term from `docs/context/terms.md` —
no bare "system" or "user".

- **Product** → a role: **A BDD System Architect** or **A BDD Product Owner**.
- **System** → a system: **The true-bdd CLI** or **The true-bdd harness**.
- **Harness** → **A Developer** (improvements to the dev harness/tooling itself).

Pick the role by self-assessment: does the actor develop true-bdd itself
(→ **A Developer**) or their own software using true-bdd (→ **A BDD System
Architect** / **A BDD Product Owner**)? Reserve system subjects for the
technology/infrastructure choices above.

## Phrasing

- Lead with the subject, then `should`/`must`, then a capability.
- Describe **observable behavior** from the outside, not implementation.
- One capability per requirement; specific enough to disambiguate.

## Tags & validation

- Tag each requirement **[revealed]** (stated by the user) or **[suggested]** (your
  inference).
- Validate them **one by one** with the user, naming the tag; keep only what the
  user confirms. Never write an unvalidated requirement.

## Non-goals

- State a **non-goal** only when it prevents a double-interpretation (e.g. "no
  auth — anyone with the URL can connect"), not as a routine disclaimer.

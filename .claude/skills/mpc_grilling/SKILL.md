---
name: mpc_grilling
description: Grill the user relentlessly about a plan, decision, or idea. Use when the user wants to stress-test their thinking, or uses any 'grill' trigger phrases.
---

Interview the user relentlessly until you reach a shared understanding. Map this as a **design tree**: every decision branches into the decisions that hang off it. The **frontier** is every decision whose prerequisites are already settled: the questions you can ask _now_ without guessing at answers you haven't heard yet.

Ask **one question per turn**, then end the turn and wait for the answer. Each question is built on every answer before it, so each turn runs this loop:

1. **Fold in the last answer.** Mark what it settled, prune the branches it closed, add the ones it opened, and recompute the frontier. When it contradicts an earlier answer, that contradiction is the next question. On the first turn, start from the plan as given.
2. **Log it**: append the last question and its answer to the [Q&A log](#the-qa-log). On the first turn, create the log instead.
3. **Research the frontier**, in two places:
   - **The environment**: code, docs, `GLOSSARY.md` and ADRs, and the tasks in the issue tracker that touch these decisions.
   - **The web**: prior art, how others solved the same problem and what bit them, current docs and limits of the tools in play.

   Finding _facts_ is your job, never the user's. Research may settle a frontier question without asking it only when it is one of two things. It can be a **fact**: something true of the world, such as a version, a limit or what the code does. It can be a **forced consequence**: with the earlier answers held fixed, exactly one reasonable option is left, and you can name the `Q<n>` that forces it. Log it and move on. Everything else is a **decision**. If a reasonable person could pick a different option, ask it, however strong your recommendation is; the ➡️ line makes accepting it a one-word "yes". When you are unsure which it is, it is a decision. Research yourself or through a sub-agent, and wait for the result: this turn's question rests on it. Research is done when you can state your recommended answer and the facts it rests on.
4. **Pick one question**: the frontier decision whose answer unblocks the most of the remaining tree.
5. **Ask it in the chat**, as plain text in your reply:

```
✅ **Settled**: <what the last answer and the research settled, one line; omit on the first question>

🔎 **Research**: <2–4 lines on what you found that bears on this question, with links; or "nothing new">

❓ **Q<n>** - **<question title>**: <question body, might be multiple paragraphs, including multiple choices>

➡️ <your recommended answer, and the facts it rests on>
```

Number questions continuously across the session. Word each question so "yes" accepts your recommended answer. Never route the question through the AskUserQuestion tool: the user answers in their own words, and its fixed options cut the answer short.

The _decisions_ are the user's: put each to them and wait. Never settle one yourself. That rules out settling it "by the user's earlier principles", settling it because the user once said they would rather not decide technical details (that answer covers its own question only), and folding it into a closing summary to be confirmed wholesale. Never write an ADR for a decision the user has not answered.

The session is done when the frontier is empty: every branch of the design tree visited, nothing left silently assumed. Do not act on it until the user confirms you have reached a shared understanding.

## The Q&A log

Every session keeps a YAML log at `docs/qna/<UTC-ts>-<slug>.yaml` in the repo root (the working directory if there is no repo), in the format of [`.claude/qna.example.yaml`](../../qna.example.yaml). The log is committed, not gitignored.

- **Create it** on the first turn, before asking Q1: `<UTC-ts>` from `date -u +%Y%m%d-%H%M%S`, `<slug>` a few kebab-case words from the plan, `intent` the plan in one or two sentences as the user stated it, `log: []`. If the user is resuming an earlier session, append to that session's file instead.
- **Append one entry** per question as soon as its answer is folded in, never batched to the end of the session, so an interrupted session keeps what it decided:
  - `decision_trace.research`: the facts from the 🔎 line, with their links; `- nothing new` when there were none.
  - `decision_trace.recommendation`: the ➡️ line, the recommended answer and the facts it rests on.
  - `decision_trace.settled`: what the answer decided, one line in your words. When it reverses an earlier answer, name the `Q<n>` it overrides.
  - `question`: the question as asked, `Q<n> - <title>: <body>`.
  - `answer`: the user's reply verbatim, even when it is just "yes".
- **A question research settled** (a fact or a forced consequence, never a decision) gets an entry too, without a `Q<n>`. Its `answer` is your own answer to the question, starting `Claude, no question asked: `, so it is never mistaken for the user's words. `settled` holds the fact, or the consequence and the `Q<n>` that forces it.
- **Decisions the user explicitly hands to you** ("decide the rest yourself") get one entry each, without a `Q<n>`, with an `answer` starting `Claude, delegated in Q<n>: ` that names the answer that handed them over. List them in your reply so the user can override any of them.

Write every free-text field as a `|` block scalar so quotes and colons in the user's words stay valid YAML. Only append: never edit or reorder earlier entries.

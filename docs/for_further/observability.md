# Scaffolding observability — spec

**Status**: design agreed 2026-08-21, not started.
**Kind**: internal experiment for this repository only. Not a TrueBDD product
feature, so it owes no registry scenarios and no `services/` entry.

## 1. The question this exists to answer

> Is the scaffolding earning its wall clock?

Concretely: for a merge, a commit, a test run or a skill invocation — how long
did each step take, and which steps did not need to run at all. The prompting
observation: a merge that takes 30 minutes as three 10-minute rounds is
30 minutes badly spent when round 1 changed nothing, and that was discovered by
reading logs by hand.

Cost scale that makes something worth acting on: one second of waste is
nothing; ten minutes of waste turns a five-minute task into an hour's delivery.

**Total wall clock is the metric**, including time spent waiting on third
parties. A 780-second wait for CodeRabbit is not excused by being someone
else's latency — it is acted on by needing fewer rounds. The *kind* of time is
recorded as an attribute because each kind has a different lever (model time →
prompt/tier, own code → optimise, third-party wait → restructure), but no kind
is excluded from the total.

### Baseline, measured

PR #79, 2026-08-19, the run that motivated this document:

| Step | Duration |
| --- | --- |
| review request → acknowledgement | 30s |
| acknowledgement → review posted | **780s** |
| read + reconcile + triage + file 2 tickets | ~120s |
| approve | 30s |
| merge + checkout main | seconds |
| **total** | **~18 min, 1 round of 3** |

That table exists only because stdout was redirected to a file by hand. Nothing
in the repository would have kept it.

## 2. Kill criterion

If this costs **more than one day of the maintainer's own time**, it is
reverted and the repository goes back to what it has today. AI time is not
counted. The design below is therefore biased at every decision toward fewer
things a human must operate, debug or eyeball.

## 3. Non-goals

- Observing production. Production is deployed and observed separately; this
  observes the *scaffolding* that builds it.
- Automatic waste detection. v1 observes. A named detector is written only for
  a pattern seen by eye twice — the first sighting is a fix in the code, as
  happened with `if not to_fix: break`.
- Hand-instrumented metrics. Derived from spans instead.
- Cross-process trace propagation. See §5.
- Backfilling history. The store starts empty.

## 4. Standard and signals

**OpenTelemetry, single and mandatory across every producer.** The standard is
what must be single, not the language: OTLP on the wire, three SDKs above it.

| Signal | How it arrives |
| --- | --- |
| **Traces** | The backbone. Every producer emits spans |
| **Logs** | Existing output (`slog`, `merge.py`'s prints) attached as **span events**, so a log line always knows which operation it belongs to. Not a separate instrumentation pass |
| **Metrics** | **Derived** from spans by the backend. Nothing hand-instrumented until a specific number proves it gets watched weekly |

## 5. Identity — three levels, all computed independently

No trace context is propagated between processes. Each producer computes all
three from git and its environment, and correlation is by **filtering on
attributes**, not by joining into one story.

| Level | Meaning | How each producer computes it |
| --- | --- | --- |
| **Trace** | one process run (`merge`, `gates`, a test run, a `code-review`) | root span per invocation |
| **Session** | one sitting, or one CI run | **local**: `start.sh` mints and exports `TRUEBDD_SESSION_ID`, so every process launched from that shell inherits it. **CI**: the GitHub Actions run id. **Claude cloud**: the agent session id |
| **Content** | what code was being exercised | commit SHA; on a dirty tree `<sha>-dirty-<short hash of the diff>`, so repeated dev runs group by the state of the tree rather than by clock time |

Also on every root span: `scaffold.source` (`local` \| `ci` \| `claude-cloud`),
branch, and PR number where discoverable.

Use OpenTelemetry's existing `vcs.*` and `cicd.*` semantic conventions for
these names wherever they exist; invent a name only where the conventions have
none, and prefix those `scaffold.`.

This is what makes "tests run per PR *and* separately during development"
answerable: both carry the same commit SHA and differ in `scaffold.source` and
session, so either question is a filter.

## 6. Producers

| Producer | Language | Instrumentation |
| --- | --- | --- |
| `merge.py` | Python | OTel Python SDK, in place. **No rewrite** — it is 1,424 lines rewritten two commits ago (#77) from 2,248 across seven files, and its design principles are documented at length in CLAUDE.md. A Go rewrite is a separate ticket judged on its own merits, and a better one once spans prove behaviour is unchanged across it |
| test harness | Go | OTel Go SDK |
| the engine | Go | Keeps its existing derivation — `reporter/` already reconstructs phases and turns from `tmp/true-bdd.log.json`, 3,375 LOC of it. Do not re-derive what is paid for |
| `gates.sh`, `commit.sh` | bash | [`otel-cli`](https://github.com/equinix-labs/otel-cli) — purpose-built for shell, and **non-recording when unconfigured**, so an uninstrumented environment is silent rather than broken |
| `code-review` and other skills | mixed | `otel-cli` around the invocation |

**New producers emit structure; the engine derives it.** Emitting is roughly
one line per call site; deriving cost 1,838 LOC for the six timeline files
alone, against a log format that is owned and stable. `merge.py`'s stdout is
prose written for a human and changes whenever the prose improves — deriving
from it would make every wording change a silent data regression.

### `merge.py` span tree

```
merge (root)                     attrs: pr, repo, head_sha, session, source
├── check_pushed
├── round 1                      attrs: round=1
│   ├── request_review           outcome: accepted | rate_limited (+attempts)
│   ├── await_review             outcome: posted | rate_limit_discovered
│   ├── read_comments            attrs: threads, body_only, after_dedupe
│   ├── reconcile                outcome: matched | gap (never fatal)
│   ├── triage_comments          attrs: findings, model, band split
│   ├── fix                      outcome: fixed | no_change   (per finding)
│   ├── create (tickets)         attrs: ticket ids
│   ├── ignore                   attrs: count
│   ├── resolve_conversations    attrs: threads answered
│   └── commit                   outcome: committed | skipped
├── merge                        outcome: plain | admin
└── postmortem                   attrs: extract bytes, tickets filed
```

Every span carries `name`, `start`, `end`, and an **`outcome` enum the
operation already computes** — these are decisions the code already branches
on, not new judgement. That is what turns "find every span over five minutes
that ended in `no_change`" into a query rather than a feature.

## 7. Transport, and the payload problem

Spans travel by **OTLP** to the server.

Large text — judge system/user/response prompts, structural diffs,
expected-vs-actual, stdout — **cannot** travel as span attributes.
OpenTelemetry's answer to large values is *truncation* (configurable attribute
length limits at SDK and collector); the spec has no blob channel. Truncating
would break the parity requirement in §10.

So: **`POST /artifacts`** on the same binary, gzipped, returning an id that the
span carries as an attribute. This is the pattern commercial systems use — store
the payload, link by id — and is not a deviation from the standard, because the
standard declines to cover it.

No separate OpenTelemetry Collector is deployed. The server is itself the OTLP
receiver. One fewer thing to operate is worth more here than collector
processors are.

## 8. Store and hosting

- **Railway**, one project.
- **A single Go monolith container** — OTLP receiver, artifact endpoint, JSON
  API and embedded UI in one binary, exactly the shape of today's report server
  (`//go:embed web`, no npm, no bundler).
- **Railway Postgres** in the same project: `DATABASE_URL` injected, private
  networking, one vendor, one bill.
- Artifacts stored gzipped in Postgres (`bytea`), not object storage — a judge
  prompt is kilobytes and a diff is at most megabytes, and a second storage
  service is a second thing to operate.
- **Grafana Cloud is a neighbour**, pointing its Postgres data source at the
  same database, deletable without touching the UI.

Why Postgres and not MongoDB: Grafana's MongoDB data source is an **Enterprise
plugin** requiring Cloud Pro/Advanced or an Enterprise licence, while the
Postgres data source is core and free. Given Grafana is wanted as a neighbour
reading the same store, Mongo puts a bill on the neighbour. Spans are also
relational with a JSONB attribute bag, and every named query is SQL-shaped.

Cost, stated: Railway has no free tier. Hobby is $5/mo as usage credit; app plus
Postgres realistically lands $10–25/mo.

## 9. Data model and UI

**Generic spans in the store, domain renderers on top.** One store, N readers —
how trace backends do service-specific views, and the only option that keeps
"single standard" true.

| Renderer | Reads attributes |
| --- | --- |
| fixture / test run | `fixture`, `checklist.cell`, `role`, `model`, `mode`, `verdict` |
| merge | `round`, `operation`, `outcome`, `pr` |
| gates / skills | `step`, `outcome` |

The cost is honest and accepted: "exactly what I see now" then depends on the
attribute mapping being right, which is what the §10 goldens exist to catch.

Screens that must survive: run list, run detail, test detail with the phase
timeline, and the two-run comparison (Myers diff, turns aligned on
`(checklist cell, role)`).

**Viewer is the Railway instance only.** There is no local mode and no
dual-source reader — the existing `:7331` server stays untouched and remains the
local, instant, offline view. The two tools then have honestly different jobs:
the old one answers "what just happened on this machine", the new one answers
"what happened anywhere, across runs".

## 10. Parity — how "preserve current behaviour" is proved

Current behaviour is the hard requirement: the maintainer must see exactly what
they see today.

1. Drive the **current** report server with playwright-go (already in this
   repo, `tests/bdd-web`) — exploratory crawl, record every path.
2. Save those paths as **goldens**: screenshots for layout, `/api/*` JSON for
   content. Screenshots catch a broken layout and miss a wrong number; JSON
   catches a wrong number and misses a broken layout. Both are needed.
3. After the move, replay the same paths against the Railway instance and
   compare.
4. The current server is **not changed**. If the new one does not hold, revert
   and lose nothing.

The point of mechanising this is the §2 budget: the maintainer reads one
green/red result, not two UIs side by side.

## 11. What must never leave the machine

The repository is **public** and the committed cassettes already carry the
prompt bodies, so the report's contents are not secret and the service needs
**no authentication**. Two exclusions remain, and they are narrow:

- **`docs/history/`** — gitignored conversation transcript. The merge
  post-mortem extracts ~300 KB of it to `tmp/merge/history-extract.md`. Never
  in a span, never in an artifact.
- **`.env` values** and anything sourced from them.

Run `.claude/skills/pr-commit/scan-recordings.sh`'s sweep over an artifact body
before upload and refuse on a hit, so the rule stays "nothing leaves unscanned".

## 12. Sequencing

All three land in one attempt, by the maintainer's decision:

1. Instrument `merge.py`, the harness and the bash gates.
2. Build the Go + Postgres OTLP monolith on Railway.
3. Port the four screens with playwright goldens proving parity.

Recorded dissent, for a cold reader: (1) alone answers §1 against any trace
viewer in an afternoon, and (3) answers none of §1 — it serves the
cross-environment test visibility that was rated "not sure this is necessary
now". The one-day budget is spendable once. The decision to do all three
together stands; the mitigation is to keep the maintainer's touchpoints few and
mechanical — one PR, and a goldens run that answers green/red.

## 13. Open

- **Does the old `:7331` server eventually get deleted**, or is it permanent as
  the offline/local view? Undecided. It stays for now either way.

## 14. Decision log

Why each branch went the way it did, so this can be picked up cold.

| # | Decision | Reason |
| --- | --- | --- |
| 1 | Purpose is scaffolding efficiency, not general observability | "Observe everything" has no failure mode and cannot be designed against |
| 2 | Cloud store | Runs happen local, in CI and on Claude cloud, with no visibility across them. `/new-task` also wipes `tmp/`, and `merge.py`'s `STATE_DIR` is not run-scoped, so each merge overwrites the last — n=1 by construction |
| 3 | Internal experiment, this repo only | Must earn extension before becoming anything more |
| 4 | Scope: merge, commit, tests, skills | Production is observed separately |
| 5 | "Worked as expected" = didn't take too long, didn't run what wasn't needed | From the 3×10-minute merge where round 1 changed nothing |
| 6 | Build on an open standard rather than research the market | Single standard across all tools is mandatory |
| 11 | Emit for new producers, derive for the engine | Derivation costs ~1.8k LOC per producer family; the engine's is already paid for |
| 12 | Total time is the metric; kind of time is an attribute | Delivery latency is what hurts, whoever caused it |
| 15 | All three signals, one instrumentation pass | Logs as span events, metrics derived |
| 16 | Custom Go UI, Grafana as a neighbour | Comparison across runs is the thing actually used, and it is not a Grafana motion |
| 17 | Instrument in place, no Go rewrite of `merge.py` | It was rewritten two commits ago; telemetry is not a reason to reopen it |
| 18 | Trace per process run, correlate by attributes | Propagation across processes that never call each other is the most expensive item on the list |
| 19 | Railway Postgres | Grafana's Mongo data source is Enterprise-only; Postgres is free and in the same Railway project |
| 23 | Upload the text | Without it the UI cannot show what it shows today, and the remote runs have no local files to fall back on |
| 26 | Start empty | Simpler |
| 27 | No auth | The repo is public and the cassettes already carry the prompts |
| 28 | No local system; old server stays | Deletes the dual-source design entirely |
| 30 | Playwright exploratory crawl → goldens → compare | Makes the revert decision a fact rather than a feeling |
| 31 | Generic spans, domain renderers | Two stores would be two standards |
| 32 | All three in one attempt | Maintainer's call, against the recommendation in §12 |

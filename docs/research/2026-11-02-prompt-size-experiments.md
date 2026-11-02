# Prompt size experiments, 2 November 2026

Recommend **lossless compact JSON for binding judgements, two-level paths with digests for body proposals and applications, and filtered embedded evidence for tool-less critiques**. Keep portions as the measured fallback. This gets both blocked turns past the size problem while preserving usable results. It does not certify a completed `build steps` or `build code` run.

The final body proposal, application and critique are **123,852 / 129,012 / 147,512 bytes**. All five proposed definitions compile against round 4's skeletons, `State`, Suite wiring and `TestMain`; a live writer produced identical source, the critique passes, and inspection agrees with the base's definitions. The complete, unfiltered `console output` binding judgement passes at **140,280 bytes** after Go-compatible JSON compaction. **25 live calls cost $24.003532 of the $250 allowance.** No further model calls are needed for this report.

## Material, scope and reproducibility

Branch: `impl/285-prompt-size-experiments`. Starting commit: `a27cc6e9bb17965afd54c89a81a05b1582ed1545`. The experiment uses round 4's cut B from `d6d0243c`, rather than replacing it with the newer checkout's generated code.

Raw data is local and ignored, at **`tmp/prompt-size-experiments/` in this worktree**. It contains:

- `original/`: copies of the two saved prompts and the binding answer, plus the round's engine log.
- `cut/`: a working copy of the supplied cut; `applied/`, `writer-cut/` and `writer-cut-v2/`: separate copies used for compilation and live application.
- `variants/<run>/{system,user}.txt` and `config.json`: the exact dispatched prompts and role, schema and byte count.
- `runs/<run>/`: argv, request digest, stream JSON, stderr, answer, usage, compilation evidence and source comparisons.
- `ledger.jsonl`, `summary.json`, `spend-by-experiment.json`: complete usage records, including the discarded harness call.
- `prepare.py`, `design.py`, `run_turn.py`, `materialize.py`, `critique.py`, `scale.py` and the remaining scratch drivers: the isolated experiment, not engine changes.

The supplied paths under `/private/var/folders/hv/59q6v5t52yn8j8s108ptk6lr0000gn/T/` were only read. The original cut is `true-bdd-2563247543`, its saved turn directory is `tmp/2026-11-02-12-39-35-77541/`, and the copied log is from `true-bdd-2094604451/runs/b-steps/engine.log`. Nothing was written to GitHub. Only this report is committed.

Readings included #285, #249 and #286's saved issues; `docs/qna/20261102-123606-prompt-size-budget.yaml`, including the corrections after Q7; `CLAUDE.md`, the domain instructions and glossary; ADR-0031, ADR-0035, ADR-0037 and ADR-0038; the concept prepare/context builders, binding evaluator, pattern proposal and critique; both CLI wrappers and recorded argv. The exact two turns contain 348 Scenarios, five `project tree` definitions and 500 distinct `console output` steps. There are 19 archived distinct binding judgements.

### Invocation and measurement

`run_turn.py <variant>` runs one foreground, isolated `claude -p` and waits for completion. It uses the recorded `claude-opus-5`, `--setting-sources ''`, `--strict-mcp-config`, `--safe-mode`, `--permission-mode acceptEdits`, stream JSON and the role's answer schema. The user prompt goes through stdin, as #284 permits. `CLAUDECODE` is removed and the advisor disabled. This machine has Claude Code 2.1.308; recordings came from an older CLI, so these are measurements of the current installed client, not a controlled reproduction of the older client's latency.

Both body proposals **and binding judgements already permit Read, Glob and Grep**; their recorded flags also permit scratch writes and forbid Bash, source edits, Agent and Task. Only the critique is stripped of file tools. Live applications use the engine-supported Claude provider with writes restricted to the steps folder and Bash forbidden. This gives the writer's native `total_cost_usd`; the normal Codex writer and its generated confinement profile were read but not exercised. The Host project's engine implementation was absent, and no Scenario was run against a target.

Each live call had a nine-minute experiment watchdog. Payloads were checked before dispatch; only the deliberately oversized whole-prompt baseline was exempt. This is an experiment safeguard, not a proposed replacement for ADR-0025's unbounded engine turns. Every live call completed normally, with no permission denial. Approaches already failing the byte check were dropped without spending a model call. The ledger was checked before each dispatch, with ample allowance for calls in flight; its final sum is authoritative. Independent read-only trials overlapped in time, so adding wall times gives serial work, not elapsed session time. Every command was foreground work and all experiment commands completed before the report was finished.

**Bytes** mean UTF-8 system plus user prompt bytes, excluding the CLI's native scaffolding and JSON schema, matching the ticket. **Input tokens** include `input_tokens + cache_creation_input_tokens + cache_read_input_tokens`; summing only `input_tokens` would misleadingly report two tokens for many turns. Output includes thinking. The peak column is the largest single assistant request's input from the stream, not the aggregate across tool requests. For example, the final paths proposal's 1,356,737 cumulative input tokens represent repeated cached requests; its largest context input was 99,854 tokens. Native envelopes still report a 1,000,000-token window and 64,000 output cap. These trials did not change the model to a 250,000-token window.

## Designs and findings

### Paths with digests

The core stays embedded: the Concept and Concept binding, all term meanings and Word mappings, the complete Public interface and all five checked definitions with every bound text and capture. Full Scenarios and fixture contents become references.

A flat list of every fixture file's path, size and SHA-256 makes the body prompt **493,655 bytes**. That version fails immediately, without a model call. Moving the list behind a manifest reference and retaining its own size and SHA-256 makes the first body prompt **123,342 bytes**. Each underlying fixture file remains named and fingerprinted in the manifest; the Scenario snapshot has its own path, size and digest. This is a two-level manifest, not a path with an untracked mutable directory behind it.

The final repeat adds a root digest for the entire evidence corpus, including `State`, the skeletons and declared library source: **2,309 files**, a **451,418-byte** manifest, only its reference embedded. This matters because file-tool reads are outside the cassette's argv/stdin hash. The request must change when any authoritative readable evidence changes, including a library helper, not just a fixture. The final proposal is **123,852 bytes**. The model searched and read selected API and Scenario evidence rather than loading the manifests wholesale. It produced no clarification questions.

Both proposals implement the same five operations as `prepareTree`, `leaveOutOfTree`, `plantLink`, `assertChecklistNarrowed` and `prepareAtomicProbes` in the base's hand-written definitions: `PrepareTree`, append to `Fixture.Absent`, append a correctly shell-quoted symlink preparation with containment checks, compare exactly one checklist snippet exactly, and require fixture preparation without doing the atomic-write operation. They add no State fields or package helpers, preserve registrations and signatures, and compile with all other definitions still skeletons. Both tool-less critiques pass.

The two live applications wrote all five files **byte-for-byte identical** to their approved proposals. The final copy differs from its input only in those five files. The first copy also has two `.DS_Store` metadata differences; `writer-change-audit.json` retains them rather than treating that whole-tree comparison as identical. The final application is **129,012 bytes**; the final critique is **147,512 bytes**. The final copy's first compilation command timed out after 120 seconds without diagnostics. An unchanged-source retry with `GOPROXY=off GOSUMDB=off`, logging `go test -x`, passed; the log includes the successful test binary result. This was not treated as evidence of a body failure. Earlier copies compiled and entered the empty test list in about two seconds.

For binding judgement, only the whole product document becomes a fingerprinted file; all 500 steps, the binding and the complete Public interface stay embedded. The corrected-schema turn passes at **92,447 bytes**, agreeing with the saved whole-prompt answer and its live rerun. The first call accidentally used the small fixture's `then:output` schema key. Its answer is discarded, its **$0.8230645** remains billed, and the corrected call uses `then:console output`. This is a harness defect, not a model failure or an approach rejection.

**Conclusion:** viable for proposal/application and viable, though unnecessary here, for judgement. A flat manifest does not solve the size problem. References require transitive digests, stable snapshots, targeted reads and a final byte gate. A short initial prompt alone does not bound agentic context growth; the measured per-request peaks are the evidence for this sample.

### Filtering

The body keeps every definition, bound text and capture, all relevant words and the complete Public interface. It removes other Scenario blocks, retains Scenario IDs for provenance and embeds fixture files consumed by these Given variations: checklist selections and the atomic-probe preparation script. Unrelated application documents are omitted because these definitions call the declared test library rather than interpreting those documents. Compact encoding makes this **124,677 bytes**; the initial pretty-printed filtered candidate was **156,200 bytes** and was re-encoded before any model call.

The five bodies compile, and the model critique says pass. **Inspection finds a semantic deviation:** the checklist body compares `strings.TrimSpace(snippets[0])` with the capture. The binding and reference compare the snippet exactly, and the library loader does not trim it. A stored snippet `" Q "` would be accepted for capture `"Q"` by the generated body and refused by the reference. This is latent on the supplied fixtures rather than an observed Scenario failure, but it prevents an unqualified fidelity pass. The critique missed it despite retaining the exact binding. It was visible after the first proposal and inspection, without scaling the approach. Do not confuse compilation or a model's pass with semantic equivalence. No source was manually corrected to hide this finding.

For judgement, filter the product to the selected Concept and the glossary entries of its declared terms. Keep all 500 steps and the whole Public interface. At **103,387 bytes**, it passes and gives the same stream, regex, polarity and observation reasons as the whole prompt. A negative control changes `does` to permit only stdout and positive matches; both the complete-product compact prompt and the filtered prompt correctly fail it. The control is synthetic evidence, never a Specification edit.

**Conclusion:** effective for judgement, but not needed before lossless compaction; it would amend ADR-0037's complete-product evidence rule. For bodies, omission must follow the definition's declared use, not a blanket fixture filter. The single generated body set has an equality defect, so prefer the paths result for this turn. The defect does not establish that filtering itself caused the model mistake.

### Portions

For the body, split by the five already checked definitions. Each independent call gets the complete common Concept, binding, words and Public interface, its own pattern/captures and every text bound by that pattern. It uses the filtered fixture evidence described above. Thus this is **filtering plus per-definition portions**, not arbitrary character slices of the 2.9 MB original.

The first three portions compile together before expanding to all five. Joining is mechanical: one distinct skeleton file per result, no State additions and no shared helper changes. All five combined compile and match the reference operations; in particular the checklist portion preserves exact equality. Their common final critique passes at **148,898 bytes**. Five proposals cost **$6.0042565** and take **466.956 serial seconds**, versus the final paths proposal's $1.834263 and 160.840 seconds. The combined critique adds $0.7227675 and 43.521 seconds. This tested decomposition works, but repeats library discovery and common context.

For judgement, divide the 500 distinct steps into balanced groups of **166 / 167 / 167**, while repeating the full product, binding and Public interface in each call. All three pass. Join by logical AND after verifying that every distinct step appears exactly once and every part returns a complete verdict for the expected binding. The closed meaning stays in every part, so a portion cannot silently narrow the admitted variations to the examples it saw. No result is reused across walks. This costs **$1.435026** and **101.957 serial seconds**, more than the full compact turn.

**Conclusion:** a viable fallback with explicit coverage and join rules, not the default for these two turns. Splitting fixture bytes into opaque slices and joining summaries was not used: it would not give each body a coherent operation to implement or provide an objective completeness check. Future portions that add shared State/helpers need a combined compilation and critique before acceptance.

### Lossless serialization

The additional, simplest approach is to remove JSON whitespace outside strings. The parsed object remains exactly equal: full product, all 500 steps, original binding and whole Public interface. Python reserialization produces **139,355 bytes** and passes. Preserving the original Go JSON escapes while removing whitespace, equivalent to `json.Compact`, produces **140,280 bytes** and also passes. The Go-compatible result is the recommended measurement; the 925-byte difference is escaping, not evidence removed.

The complete-product negative control fails. Compaction cannot solve the body: its compact context alone is **2,446,999 bytes**, before the system prompt and task. Drop compaction as a standalone body approach at preflight, with zero model spend. All four approaches were consequently exercised on both required turns or rejected on an exact size calculation before dispatch.

**Conclusion:** use compaction first for structured judgement payloads. It preserves ADR-0037 and avoids giving an otherwise one-request judgement extra tool calls. Filtering is cheaper in this sample, but compaction needs no decision about which product evidence may be discarded.

## Every live call

Run names are the raw-data directory names. Costs below are native `total_cost_usd`, rounded to six decimals; budget sums use the unrounded envelopes. Proposal rows' compilation and semantic outcomes are described above; deterministic materialization copies the exact returned Go fences and runs gofmt, with no code fixes. Only the paths chains also used live application turns. Critiques embed the complete current steps package and filtered evidence, omit duplicate Scenario IDs and unrelated Scenario blocks, retain all bound texts/captures and require no file tools.

| Run | Prompt bytes | Input tokens, including cache | Output tokens | Peak request input | USD | Wall seconds | Result |
|---|---:|---:|---:|---:|---:|---:|---|
| `paths-binding` | 92,447 | 120,389 | 3,490 | 71,118 | 0.823064 | 50.8 | Discarded: wrong schema key |
| `paths-body` | 123,342 | 1,566,208 | 9,704 | 115,802 | 2.125957 | 132.6 | 5 bodies compile; critique passes |
| `filter-body` | 124,677 | 1,573,198 | 11,328 | 108,253 | 1.997902 | 171.3 | Compiles; exact-equality drift |
| `paths-binding-v2` | 92,447 | 120,396 | 4,155 | 71,123 | 0.839741 | 87.7 | Pass; agrees with whole prompt |
| `portions-body-1` | 127,859 | 802,520 | 4,317 | 86,131 | 1.227066 | 64.9 | One body; combined suite compiles |
| `filter-binding` | 103,387 | 52,759 | 2,042 | 52,759 | 0.484855 | 30.8 | Pass; agrees with whole prompt |
| `paths-body-critique` | 147,303 | 70,685 | 3,655 | 70,685 | 0.798215 | 47.3 | Pass, no findings |
| `portions-binding-1` | 105,307 | 49,096 | 1,631 | 49,096 | 0.437951 | 24.5 | Pass; whole-prompt agreement |
| `portions-binding-2` | 108,818 | 50,644 | 2,069 | 50,644 | 0.464381 | 31.0 | Pass; whole-prompt agreement |
| `portions-body-2` | 70,315 | 679,872 | 5,967 | 74,748 | 1.098862 | 79.8 | One body; combined suite compiles |
| `filter-body-critique` | 147,194 | 70,568 | 4,754 | 70,568 | 0.751797 | 62.0 | Pass; missed equality drift |
| `portions-binding-3` | 111,223 | 105,351 | 2,561 | 53,660 | 0.532695 | 46.4 | Pass; whole-prompt agreement |
| `paths-body-apply` | 125,934 | 451,548 | 5,073 | 71,428 | 1.031209 | 71.7 | 5 files identical to proposal |
| `baseline-binding` | 157,777 | 70,538 | 3,650 | 70,538 | 0.702845 | 56.1 | Pass; agrees with round 4 |
| `portions-body-3` | 70,175 | 1,172,045 | 9,995 | 81,262 | 1.507595 | 132.9 | One body; combined suite compiles |
| `compact-binding` | 139,355 | 63,819 | 3,204 | 63,819 | 0.624506 | 49.4 | Pass; agrees with whole prompt |
| `compact-negative` | 139,327 | 63,809 | 777 | 63,809 | 0.563730 | 14.4 | Fail, correctly |
| `filter-negative` | 85,862 | 46,613 | 816 | 46,613 | 0.392746 | 15.3 | Fail, correctly |
| `portions-body-4` | 75,601 | 870,888 | 7,021 | 80,418 | 1.274603 | 101.9 | One body; combined suite compiles |
| `portions-body-5` | 69,663 | 496,281 | 6,843 | 60,767 | 0.896130 | 87.5 | One body; combined suite compiles |
| `compact-binding-go` | 140,280 | 64,402 | 1,712 | 64,402 | 0.593036 | 26.9 | Pass; agrees with whole prompt |
| `paths-body-v2` | 123,852 | 1,356,737 | 12,304 | 99,854 | 1.834263 | 160.8 | 5 bodies compile; critique passes |
| `portions-body-critique` | 148,898 | 71,250 | 3,320 | 71,250 | 0.722768 | 43.5 | Pass, no findings |
| `paths-body-v2-apply` | 129,012 | 1,373,486 | 6,512 | 82,182 | 1.534968 | 94.4 | 5 files identical to proposal |
| `paths-body-v2-critique` | 147,512 | 70,788 | 4,300 | 70,788 | 0.742648 | 56.9 | Pass, no findings |

## Scale and comparison

The experiment started with the two blocked turns and the three approach designs, compiled a three-definition portion pilot, then expanded to five definitions, three binding portions and complete application/critique chains. `scale/measurements.json` additionally builds filtered payloads for all **19 archived bindings**, examining the first 3, then 10, then all 19. They range from **23,107 to 103,120 bytes**. That scale exercise is a **size-only** result; it is not 19 live verdicts. Its JSON shape differs slightly from the single binding trial, hence 103,120 versus 103,387 bytes.

The first body's huge fixture corpus is not inherently needed to decide which harness method loads a fixture. Conversely, a definition that depends on a fixture program's input contract still needs that contract: this result does not justify dropping every fixture from every Concept. All successful tested body proposals stay below 116,000 peak input tokens per request; the final paths repeat peaks at 99,854. The final critique peaks at 70,788. This is consistent with the Owner's 250K-token target plus the declared 64K output allowance, but a byte limit is not a general proof about a later tool-heavy turn's context.

No trial reduced model effort, reused an earlier walk's verdict, changed a binding or used target code to make a definition green. Neither an entire generated suite's red-without-target run nor `build code` was attempted: 18 other Items still have skeleton bodies. The observed outcome is the usable code and judgement the next pipeline stages require, not a claim that rebuilding the engine is finished. The normal Codex writer remains a next-round validation obligation.

| Experiment | New model spend, USD |
|---|---:|
| Harness error | 0.823064 |
| Paths | 8.907000 |
| Filtering | 3.627301 |
| Portions | 8.162050 |
| Baseline | 0.702845 |
| Compaction | 1.781271 |
| **Total** | **24.003532** |

The total is **$24.003532**, leaving **$225.996468**. Historical round-4 spend ($0.757545 and 32.559 seconds for the saved binding judgement) is a reference, not new experiment spend. There was no live whole-body baseline: its original 2,916,161 bytes already fail the required budget and the ticket's window analysis. No approach was continued to consume the allowance after a decisive failure.

## Recommendation for #286

| Turn kind | Recommended context construction | Measured system plus user bytes | What #286 changes |
|---|---|---:|---|
| Binding judgement | Lossless compact JSON first; preserve the complete product, binding, Public interface and every distinct linked step | 140,280 for `console output` | Replace indented model payload serialization; keep pretty artifacts separately if wanted. No Concept or verdict rule change. |
| Body proposal | Embedded semantic core and all checked patterns/captures/texts; full Scenario and fixture evidence through two-level paths, sizes and transitive SHA-256 digests | 123,852 | Change `describeConceptItem`/fixture context rendering and the body template; fingerprint State, helpers and declared library evidence as well as fixtures. |
| Body application | The same immutable evidence identity plus the approved fix, retaining writer boundaries | 129,012 | Change the applier template too; shrinking only the proposal leaves its application oversized. Regenerate identity from the pre-apply state. |
| Tool-less critique | Filter by semantic use, embedding the Concept/binding/words, full interface, all bound steps and relevant fixture contracts; include source needed to judge the body | 147,512; portions join 148,898 | Change the critique context builder/template; keep file tools disabled. Use portions/source dependency closure when whole-package source will not fit. |
| Other structured judgements, including grouping/proposal/link/words | Compact first; byte-budget coherent batches if needed, retaining each judgement's required semantic universe | Not live-validated here | Audit every dispatcher. Grouping must retain all candidate Concepts and preserve its ranking/ambiguity protocol; a count-only batch size is no byte guarantee. |
| Large future body/critique or build-code turn | Apply the hard byte refusal, then an explicit coherent partition or digest-backed evidence design appropriate to that turn | Not yet established | Keep a distinct refusal instead of truncation. The next #249 round measures the next blocker; this report does not establish a complete build-code context protocol. |

The critique's 147,512 bytes include the whole steps package **at this cut**. As other Items acquire bodies, that package grows. Do not hard-code today's projection or assume the same bytes will hold later. A dependency closure needs the selected bodies, registrations, State, every shared helper they reach and the evidence those helpers rely on; a partial closure must not receive a pass as if complete. Per-definition portions were exercised for generation; a production partitioner for arbitrarily large critique/helper graphs was not validated here. Until it exists, the hard refusal is the correct outcome. This is a material limit of the recommendation, not a proved solution to every possible prompt.

The hard gate belongs immediately before **every** Claude and Codex dispatch, including retries, fixes, writers and critiques: measure the actual UTF-8 system/user payload delivered to that CLI, log its size and refuse an oversized request with turn identity, size and configured limit. Default to 150,000 bytes through `true-bdd.yaml`, as Q5 decided. Account for Codex's system/user assembly as well. References must not bypass confinement or permit service source to become an authority. Index digests must transitively cover file contents, sizes and path-set changes so argv/stdin replay identity cannot silently reuse an old answer after readable evidence changes. Re-encode and recheck after adding findings or approved fixes; today's spare bytes are not a retry allowance.

### Product document, architecture, ADRs and CLAUDE.md

The current product statement that a body turn embeds every using Scenario and all its fixture inputs must change: require the complete evidence to remain available and fingerprinted, with the semantic core and all checked steps embedded, and permit file-backed snapshots in read-enabled turns. State the same rule for application. For critiques, replace universal embedding with a complete declared evidence projection and a source closure or coherent portions, explicitly preserving any fixture program contract the definition uses. The architecture's prompt-data descriptions and generated templates must agree.

The compact binding route **does not contradict ADR-0037**: its full product, service Public interface and every linked step remain present, judged once per walk. Filtering the product or dividing one binding into several judgements would require an explicit amendment and complete part aggregation. Changing body/critique embedding qualifies ADR-0035's context construction; record that change in #286's ADR rather than silently claiming the old whole-evidence prompt is still sent. ADR-0031's external observation and red-without-target requirements and ADR-0038's harness/wiring ownership stay intact.

Replace `CLAUDE.md`'s runtime rule of “full embedded context, whole prompts” with: **every necessary AI turn receives a complete, auditable evidence contract within the configured byte budget; embed the semantic core, permit immutable digest-backed references where tools can read them, and use coherent portions with verified coverage when necessary. Never truncate, silently omit required evidence, skip critique, or reduce reasoning effort to meet the cap.** This changes context transport and partitioning, not the rule that a required AI turn must happen. The new ADR should cite this report and the Q&A's Q1–Q7, Q4 revisited and the post-Q7 scope/check decisions, and retain #285's window/reserve derivation.

## Owner decisions and remaining uncertainty

The Owner can choose the mixed route above without first adopting product filtering for judgements. The alternative is filtered judgements for lower cost, accepting an ADR-0037 evidence change; portions are available where a compact complete turn still does not fit. The open scope decision is whether #286 also implements a general critique source-closure/partition protocol immediately or follows the next #249 blocker after the two known turns. All later turns must still be hard-gated.

One or two successful body proposals and positive binding trials are mechanism evidence, not a statistical reliability guarantee. The missed whitespace discrepancy demonstrates why inspection and the next drill remain necessary. The next round must exercise the normal Codex writer, whole-suite checks, all remaining body/critique turns and eventual code generation. No factual question blocked this experiment and no Owner answer was assumed.

## Repository validation

- `go run ./scripts/cmd/alint docs/research/2026-11-02-prompt-size-experiments.md`: passed.
- `go run ./scripts/cmd/gates run --changed main`: passed, exit 0, all nine selected gates. The inherited diff from `main` selected the full table; this included build and tests in all three Go modules, vet, BDD coverage guards, release rehearsal and the recordings scan. Full output is `tmp/prompt-size-experiments/gates.log`.
- `git diff --check`: passed. The 25 saved system/user payloads were independently recounted and agree with the usage table; only the explicitly exempt baseline exceeds 150,000 bytes.
- Specification divergence: this commit changes only a research report. No runtime, test, cassette or Specification change is asserted or made; the proposed context-rule changes and ADR amendments above remain #286's work.

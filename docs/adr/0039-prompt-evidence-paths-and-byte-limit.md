# Read-enabled turns receive evidence by path within a hard prompt byte limit

Round 4 of #249 stopped at the `project tree` body proposal: 2,916,161 bytes, including 2,457,752 bytes of fixture inputs and 352,621 of Scenarios. The `console output` binding judgement was 157,777 bytes. [The #285 report](../research/2026-11-02-prompt-size-experiments.md) measured paths, filtering, portions and lossless serialization. Its paths proposal/application/critique chain compiled and preserved five bodies; the file-backed binding judgement agreed with the whole-product verdict.

We decided:

- Body proposals and applications embed the Concept, Concept binding, declared terms' glossary entries and Word mappings, whole Public interface, definitions, patterns, captures and every bound step. They receive the Registry path, Scenario keys and fixture directories, and read needed inputs through confined tools. There are no digests. Reads admit Specification, suite and declared libraries, never service source; write grants do not grow.
- Binding judgements read the complete product document at `product_path`; the binding, steps and complete Public interface stay embedded. They still run once per binding per walk in both commands.
- Critiques remain tool-less. They embed the semantic core, every bound step, whole suite source and fixture contracts selected by use: preparation, teardown and checklist inputs, files named by source or bound captures. Other Scenario blocks and unrelated fixture documents are omitted. A future large critique refuses at the limit; portions are separate work.
- Every dispatch through the router measures the actual UTF-8 system and user bytes. Codex includes its system/task assembly headings. At or below the limit it logs `prompt_bytes` and `prompt_limit_bytes`; above it, `PromptLimitError` names the turn (label and role), size and limit before a provider starts. `engine.max_prompt_bytes` in `true-bdd.yaml` is a positive integer, default 150,000. Retries and approved fixes are measured anew. CLI-owned system/tool bytes are outside this count.
- No compaction, silent omission of required evidence, critique skipping or reasoning downgrade meets the limit. The Owner rejected digests and compaction after #285; those later answers override the report's recommendations.

## Derivation

The Owner's target window is 250,000 tokens. The #285 ticket reserves 15,000 for Claude's system/tools (502 calls: input tokens = 0.286 × characters + 13,154), 64,000 for answer and reasoning (`claude-opus-5` output allowance; largest observed output 40,840), and 20,000 for agentic file reads (708 calls: growth p99 11,542, maximum 18,105). That leaves 151,000 tokens. At the conservative measured density of 2.5 characters per token, approximately 377,000 characters fit; 370,000 bytes was the proposed ceiling. The Owner chose the stricter 150,000 bytes to force prompt optimization. Byte measurement is deterministic, not a proof that later tool reads fit every model's context window. Another host may choose its own limit for its models.

This amends ADR-0035's body and critique transport and ADR-0037's embedded product transport; their semantics and once-per-walk judgements remain. ADR-0025's unbounded turn lifetime, ADR-0031's external observation and red without the target, and ADR-0038's Suite wiring ownership remain.

Grilling log: `docs/qna/20261102-123606-prompt-size-budget.yaml`, Q1–Q7, Q4 revisited, the hard-check confirmation, and the two post-report decisions rejecting compaction/digests and putting the product document in a file.

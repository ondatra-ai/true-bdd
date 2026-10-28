# Plan-read: {{draft, diagnosis or plan}}

Claude wrote {{a spec draft, diagnosis or plan}} in TrueBDD and will {{publish it, write the fix or carry it out}} next. You are its second reader. Change nothing; your final message is your report.

## Read first

- Document: {{path or issue #N saved through `--issue`}}.
- Evidence: {{files, logs, issues and commands the document rests on}}.
- The Owner's decisions: {{the `docs/qna/` log and its `Q<n>` this run carries out, or none}}. Read `docs/qna/` as `docs/agents/domain.md` says: oldest to newest, a newer entry overriding an older one.
- Rules: `CLAUDE.md`, `docs/agents/domain.md`, `GLOSSARY.md`, relevant `docs/adr/` and matching `.claude/rules/*.md`.

## Question

Where may this be wrong? Check facts against the code and tools themselves with commands that only read. Look for unsupported claims, false facts, steps that break the rules, omitted cases and decisions described as settled without evidence.

{{What Claude doubts most.}}

## Report

Numbered objections, most serious first. Each gives the quoted passage, what is wrong, evidence (`file:line`, a command and its result, or source), confidence (certain, likely, possible) and what would settle it. Then one line on what you checked and found sound. No preamble; at most {{600}} words.

# Plan-read: {{draft, diagnosis or plan}}

Claude wrote {{a spec draft, diagnosis or plan}} in TrueBDD and will {{publish it, write the fix or carry it out}} next. You are its second reader. Change nothing; your final message is your report.

## Read first

- Document: {{path or issue #N saved through `--issue`}}.
- Evidence: {{files, logs, issues and commands the document rests on}}.
- Rules: `CLAUDE.md`, `docs/agents/domain.md`, `GLOSSARY.md`, relevant `docs/adr/` and matching `.claude/rules/*.md`.

## Question

Where may this be wrong? Check facts against the code and tools themselves with commands that only read. Look for unsupported claims, false facts, steps that break the rules, omitted cases and decisions described as settled without evidence.

{{What Claude doubts most.}}

## Report

Numbered objections, most serious first. Each gives the quoted passage, what is wrong, evidence (`file:line`, a command and its result, or source), confidence (certain, likely, possible) and what would settle it. Then one line on what you checked and found sound. No preamble; at most {{600}} words.

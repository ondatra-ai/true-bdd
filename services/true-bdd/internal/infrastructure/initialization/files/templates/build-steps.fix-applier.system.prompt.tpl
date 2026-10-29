You apply the approved body of one Catalogue phrase in one suite.
Preserve the engine's registration, pattern and signature. Modify only the body
and the suite's shared state. Use the declared Public interface and Phrase
mapping; never implement the product's operation inside a step definition.

**Output Requirements:**
- After your Write/Edit calls succeed, output a YAML confirmation
  inside FILE_START/FILE_END markers:
  - `applied: true` (or `false` on failure)
  - `target: "<repo-relative path>"` (QUOTED — the path may contain `: `)
  - `summary: "<one short line describing what changed>"`
- Preserve all other content in the target file — only add the
  registration and the function the fix prompt specifies.

**Output Format:**
```
=== FILE_START: {{.ResultPath}} ===
applied: true
target: "<repo-relative path>"
summary: "<one-line summary>"
=== FILE_END: {{.ResultPath}} ===
```

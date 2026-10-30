{{ if .Subject.Concept }}You apply the approved bodies of one concept's step definitions in one suite.
Preserve the engine's registrations, patterns, signatures and //truebdd:concept
records. Modify only those bodies and the suite's shared state. Use the declared
Public interface and the concept's binding; never implement the product's
operation inside a step definition.{{ else }}You apply the approved body of one Catalogue phrase in one suite.
Preserve the engine's registration, pattern and signature. Modify only the body
and the suite's shared state. Use the declared Public interface and Phrase
mapping; never implement the product's operation inside a step definition.{{ end }}

**Output Requirements:**
- After your Write/Edit calls succeed, output a YAML confirmation
  inside FILE_START/FILE_END markers:
  - `applied: true` (or `false` on failure)
  - `target: "<repo-relative path>"` (QUOTED — the path may contain `: `)
  - `summary: "<one short line describing what changed>"`
- Preserve all other content in the target file — only write the
  selected body and extend shared State as needed.

**Output Format:**
```
=== FILE_START: {{.ResultPath}} ===
applied: true
target: "<repo-relative path>"
summary: "<one-line summary>"
=== FILE_END: {{.ResultPath}} ===
```

If the declared Public interface cannot set up or observe the {{ if .Subject.Concept }}concept{{ else }}phrase{{ end }}, stop without
inventing an operation. Return the same confirmation markers with:
```yaml
applied: false
interface_cannot_serve:
  phrase: {{ if .Subject.Concept }}{{ .Subject.Label | printf "%q" }}{{ else }}{{ .Subject.Phrase.Template | printf "%q" }}{{ end }}
  lacks: "<what the interface offers no way to set up or observe>"
```

Author solely from the supplied Specification and the suite's state and helpers.
Do not read service implementation code. Do not execute Scenarios, replay the
suite against the target, or require a Scenario to pass: the target may be absent
or not built yet. The engine owns compilation and the whole-suite checks. If you
compile the steps package while writing, preserve the inherited Go cache and
temporary-directory environment; never create a separate build cache.

Keep every new constant and local helper value inside the selected function body.
Do not add package-level constants, variables, helper functions or types; only
shared State fields and the selected file's imports may change outside that body.

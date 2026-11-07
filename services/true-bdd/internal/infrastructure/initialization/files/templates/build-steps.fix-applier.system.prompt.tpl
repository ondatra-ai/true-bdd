You apply the approved bodies of one concept's step definitions in one suite.
Preserve the engine's registrations, patterns, signatures and //truebdd:concept
records. Modify those bodies, the suite's shared state and helpers/. Use the declared
Public interface and the concept's binding; never implement the product's
operation inside a step definition.

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

If the declared Public interface cannot set up or observe the concept, stop without
inventing an operation. Return the same confirmation markers with:
```yaml
applied: false
interface_cannot_serve:
  concept: {{ .Subject.Label | printf "%q" }}
  lacks: "<what the interface offers no way to set up or observe>"
```

Author solely from the supplied Specification and the suite's state and helpers.{{ if regexMatch "(?m)^runtime_api:" .Subject.Context }}
The supplied runtime API also states what the runtime does.{{ end }}
Do not read service implementation code. Do not execute Scenarios, replay the
suite against the target, or require a Scenario to pass: the target may be absent
or not built yet. The engine owns compilation and the whole-suite checks. If you
compile the steps package while writing, preserve the inherited Go cache and
temporary-directory environment; never create a separate build cache.

Put code the steps share in tests/<suite>/helpers/, beside the steps/ directory
named by Suite. Write the helper functions, constants and types the bodies need
there; helpers are generated with the bodies and need no separate declaration.
Only selected bodies, their imports and shared State may change inside steps/.
Helpers must obey the same binding and meaning as their callers, and never
compute the product's answer. The critique judges their complete source too.

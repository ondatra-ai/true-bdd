{{/*
Unreachable today, for the same reason as the fix generator: no
scen-check prompt carries an `F:`, so --fix is refused at startup.
*/ -}}
## Subject — Registry Scenario

**Scenario ID:** {{.Subject.ID}}
**Description:** {{.Subject.Description}}
**Service:** {{.Subject.Service}}
**Generated test file:** {{.Subject.Path}}

### Steps
{{.Subject.FormatSteps}}
### Lineage — user_stories[]
{{.Subject.FormatUserStories}}
---

## Fix Prompt to Apply

{{.FixPrompt}}

---

## Instructions

1. Use the Read and Grep tools to locate the `{{.Subject.ID}}` entry in
   the scenario registry and read it as it stands.
2. Apply the changes described in the "Fix Prompt to Apply" section
   above with the Edit tool, matching the old_string verbatim.
3. You MAY only modify the `{{.Subject.ID}}` entry. Another scenario's
   entry, a generated test file and a step definition are all out of
   bounds — the generated test is re-rendered from the registry by
   `true-bdd build tests --fix`, so editing it here would be discarded.
4. You MUST NOT change `service:`, `path:` or `user_stories[]`.
5. Keep the behaviour the scenario specifies. You are rewording it, not
   deciding what it should assert.
6. After the changes succeed, output the confirmation block below.

=== FILE_START: {{.ResultPath}} ===
applied: true
target: "<repo-relative path of the registry file you edited>"
summary: "<one-line summary of what changed>"
=== FILE_END: {{.ResultPath}} ===

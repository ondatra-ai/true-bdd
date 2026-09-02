{{/*
Unreachable today: no scen-check prompt carries an `F:`, so the command
refuses --fix at startup and this template is never rendered. It exists
because runner.Run refuses a nil FixGenerator, and because the shape a
scen-check fix would take should be decided once, here, rather than
improvised the day a prompt grows a fix template.
*/ -}}
# Generate Fix Prompt for a Registry Scenario

## Reference Documentation
{{- range $key, $path := .DocPaths }}
- Read(`{{ $path }}`) — {{ $key }}
{{- end }}

## Where the fix goes

Into the scenario's own entry in the scenario registry, and nowhere
else. A scen-check finding is about the words of one entry: its
description, or one of its steps. The generated test file rendered from
it is not edited by hand — `true-bdd build tests --fix` re-renders it
from the registry once the entry reads correctly.

## Subject — Registry Scenario

**Scenario ID:** {{.Subject.ID}}
**Description:** {{.Subject.Description}}
**Service:** {{.Subject.Service}}
**Generated test file:** {{.Subject.Path}}

### Steps
{{.Subject.FormatSteps}}
### Lineage — user_stories[]
{{.Subject.FormatUserStories}}
## Validation Failure

The following check failed and needs to be fixed:

### Failed Check: {{ .FailedCheck.SectionPath }}

**Question:** {{ .FailedCheck.Question }}
**Actual:** {{ .FailedCheck.ActualAnswer }}

**Suggested Fix Template:**
{{ .FailedCheck.FixPrompt }}

{{- if .UserAnswers }}

## User Clarifications (from previous questions)

The user provided the following clarifications:
{{- range $id, $answer := .UserAnswers }}
{{- if ne $id "_user_refinement" }}
- **{{ $id }}**: {{ $answer }}
{{- end }}
{{- end }}

Use these answers to generate a confident fix. Do not ask these
questions again.
{{- end }}

{{- if index .UserAnswers "_user_refinement" }}

## REFINEMENT MODE — User Feedback (CRITICAL)

The user has reviewed your PREVIOUS fix prompt and is providing
feedback to CORRECT IT:

> {{ index .UserAnswers "_user_refinement" }}

**CRITICAL INSTRUCTIONS FOR REFINEMENT:**
1. **DO NOT ask more questions**
2. **Address the specific issue** — Fix exactly what the user pointed out
3. **Keep everything else** — Preserve parts of your previous fix that
   weren't criticized
4. **Output a fix prompt** — You MUST output FILE_START/FILE_END,
   NEVER QUESTIONS_START/QUESTIONS_END
{{- end }}

## Your Task

{{- if index .UserAnswers "_user_refinement" }}
**REFINEMENT MODE**: Apply user feedback and regenerate the fix prompt.
DO NOT ask questions.
{{- else if .UserAnswers }}
Using the user's clarifications above, generate a complete fix prompt.
{{- else }}
Analyze if you have enough context to generate a confident fix.
{{- end }}

**If you can generate a confident fix**, output a fix prompt the applier
can execute against the registry entry for `{{.Subject.ID}}`. The fix
MUST:
- Change only the fields the failed check names — a rewritten
  description, or the steps it objected to.
- Keep the behaviour the scenario specifies. Rewording is the whole
  point; re-specifying is out of scope and silently loses coverage.
- Leave `service:`, `path:` and `user_stories[]` untouched. They are
  lineage and routing, not prose, and nothing in this checklist rules
  on them.
- Never edit a generated test file, a step definition, or another
  scenario's entry.

=== FILE_START: {{.ResultPath}} ===
# Fix Prompt for Scenario {{.SubjectID}}: {{.Subject.Description}}

## Target
The `{{.Subject.ID}}` entry of the scenario registry.

## Required Changes
### Change #N: [description]
**Field:** description | merged_steps.<given|when|then>
**Action:** Use the Edit tool with the exact strings below.
**old_string:**
```
<verbatim slice of the entry as it stands>
```
**new_string:**
```
<the replacement>
```
=== FILE_END: {{.ResultPath}} ===

**If you need clarification first**, output:

=== QUESTIONS_START ===
questions:
  - id: q1
    question: "<your question>"
    context: "<why you need this information>"
    options:
      - "<option 1>"
      - "<option 2>"
=== QUESTIONS_END ===

**Important:**
- Output EXACTLY ONE block (FILE_START/FILE_END or QUESTIONS_START/QUESTIONS_END)
- Never output both

{{ if .Structured }}
**ANSWER CONTRACT FOR THIS RUN — this supersedes the FILE_START/FILE_END
and QUESTIONS_START/QUESTIONS_END instructions above.** Your final message
must be a single JSON object with both keys present:

- `fix_prompt`: the fix instructions, or `""` when you need clarification.
- `questions`: an array of `{id, question, context, options}` objects, or
  `[]` when you are returning a fix prompt.

Exactly one of the two carries content. Do not emit any markers, and do not
wrap the object in markdown fences. Reasoning before the object is ignored.
{{ end }}

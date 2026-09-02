# Generate Fix Prompt for an Unbound Scenario Step

## Reference Documentation
{{- range $key, $path := .DocPaths }}
- Read(`{{ $path }}`) — {{ $key }}
{{- end }}

## Where the fix goes

Read the architectural spec above and find the
`architecture.testing.suites[]` entry whose `service:` equals the
subject's **Service**. That entry's `path:` is the suite root, and the
only tree the applier may write:

    <suite path>/steps/

Glob and Read every file already there before proposing anything. The
definitions in it share one state type and a set of helpers; a fix that
re-implements what a sibling already does is the drift this command
exists to prevent.

## Subject — Registry Scenario

**Scenario ID:** {{.Subject.ID}}
**Description:** {{.Subject.Description}}
**Service:** {{.Subject.Service}}
{{- if .Subject.Requirement }}
**Requirement:** {{.Subject.Requirement}}
{{- end }}

### Steps
{{.Subject.FormatSteps}}

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
can execute. It may Write a new file or Edit an existing one under
`<suite path>/steps/` and nowhere else. The fix MUST:
- Register one definition per unbound step:
  `suite.Step(`^<anchored regexp>$`, <func>)`, added to the suite's
  existing `Register` function rather than a new one.
- Put a capture group where the step's text varies — a quoted run, a
  number — so the definition serves every scenario phrasing it that
  way. A pattern that is a literal copy of one scenario's line is a
  definition that will never match a second scenario.
- Implement the function against the state type the sibling definitions
  already use: Given prepares, When acts, Then asserts and returns an
  error naming what it expected and what it got.
- Never weaken an existing definition so this step matches it.
- Never modify the scenario registry.

=== FILE_START: {{.ResultPath}} ===
# Fix Prompt for Scenario {{.SubjectID}}: {{.Subject.Description}}

## Target
`<suite path>/steps/<file>.go` — the suite resolved from the
architectural spec for service `{{.Subject.Service}}`.

## Required Changes
### Change #N: [description]
**File:** <repo-relative path>
**Action:** Use the Write tool (new file) or the Edit tool (extend
existing) with the exact content below.
**Content / new_string:**
```
<paste the exact registration and function here>
```
**old_string** (only for Edit on an existing file):
```
<verbatim slice the new code should replace or append after>
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

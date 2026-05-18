# Generate Fix Prompt for Missing Test

## Search Roots
- `tests/integration/`
- `tests/e2e/`
- `services/backend/`
- `services/frontend/`

Use Glob, Grep, and Read on these trees to understand the existing
test layout and pick the right file to extend (or create).

## Reference Documentation
{{- range $key, $path := .DocPaths }}
- Read(`{{ $path }}`) — {{ $key }}
{{- end }}

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

**If you can generate a confident fix**, output a fix prompt that the
applier can execute. The applier is allowed to Write new test files
and Edit existing ones, but ONLY under the four search roots above.
The fix MUST:
- Pick the right tree: `tests/integration/` for `INT-*` scenarios
  (Playwright Request API, no browser), `tests/e2e/` for `E2E-*`
  scenarios (Playwright Browser/Page API).
- Either extend an existing spec file most aligned with the scenario's
  service or create a new one following the naming pattern
  `<service>-*.spec.ts`.
- Add a single `test('{{.Subject.ID}}: <short description>', ...)`
  whose body translates the subject's Given / When / Then into
  executable assertions.
- Never modify `docs/requirements.yaml`.
- Never duplicate an existing test.

=== FILE_START: {{.ResultPath}} ===
# Fix Prompt for Scenario {{.SubjectID}}: {{.Subject.Description}}

## Target
Apply the change under one of:
- `tests/integration/<service>-*.spec.ts`
- `tests/e2e/<service>-*.spec.ts`
- `services/backend/...` (only if the natural test location is here)
- `services/frontend/...` (only if the natural test location is here)

## Required Changes
### Change #N: [description]
**File:** <repo-relative path>
**Action:** Use the Write tool (new file) or the Edit tool (extend
existing) with the exact content below.
**Content / new_string:**
```
<paste the exact test block here>
```
**old_string** (only for Edit on an existing file):
```
<verbatim slice the new test should append after>
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

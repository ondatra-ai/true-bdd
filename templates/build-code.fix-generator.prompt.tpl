# Generate Fix Prompt for Failing Test

## Edit Roots (HARD LIMITS)

The fix applier may only Write or Edit files under `services/*`.

The fix applier MUST NOT touch:
- Any file under `tests/`
- Any file matching `*_test.go`
- Any file under `services/*/__tests__/`
- `docs/requirements.yaml` or anything else under `docs/`

Do NOT propose changes that violate these limits.

## Investigation Tools

Use Read, Glob, and Grep to inspect:
- The failing test file (`{{.Subject.FilePath}}`) to understand the
  contract being asserted.
- The production source under `services/{{.Subject.Service}}/` (or the
  service the test exercises).
- Sibling tests passing today, to mirror conventions.

## Reference Documentation
{{- range $key, $path := .DocPaths }}
- Read(`{{ $path }}`) — {{ $key }}
{{- end }}

## Subject — Failing Test

**Test ID:** {{.Subject.ID}}
**Service:** {{.Subject.Service}}
**Layer:** {{.Subject.Layer}}
**Framework:** {{.Subject.Framework}}
**Test Name:** {{.Subject.TestName}}
**Source File:** {{.Subject.FilePath}}

### Last Failure Output
```
{{.Subject.FailureOutput}}
```

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
applier can execute. The applier is allowed to Write new files and Edit
existing ones, but ONLY under `services/*` (and never any test file
or anything under `docs/`). The fix MUST:
- Make the smallest production change that causes the test to pass
  without weakening, removing, or skipping any assertion.
- Reference exact file paths and use exact code snippets the applier
  can copy verbatim.
- Never modify the test itself.
- Never modify `docs/requirements.yaml` or anything else under `docs/`.

=== FILE_START: {{.ResultPath}} ===
# Fix Prompt for Failing Test {{.SubjectID}}

## Target
Apply the change(s) under `services/*` only. The test at
`{{.Subject.FilePath}}` is the contract; do NOT modify it.

## Required Changes
### Change #N: [description]
**File:** <repo-relative path under `services/*`>
**Action:** Use the Write tool (new file) or the Edit tool (existing
file) with the exact content below.
**Content / new_string:**
```
<paste the exact code block here>
```
**old_string** (only for Edit on an existing file):
```
<verbatim slice the new code should replace or anchor against>
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

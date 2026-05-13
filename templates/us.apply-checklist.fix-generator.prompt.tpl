# Generate Fix Prompt for Registry Merge

## Registry (read this file first)
- Read(`{{.Subject.RequirementsScratchPath}}`) — current registry state

## Reference Documentation
{{- range $key, $path := .DocPaths }}
- Read(`{{ $path }}`) — {{ $key }}
{{- end }}

## Subject — Acceptance Criterion

**Story ID:** {{.Subject.StoryID}}
**Story Path:** {{.Subject.StoryPath}}
**AC ID:** {{.Subject.ACID}}
**Lineage Scenario ID:** {{.Subject.LineageScenarioID}}
**Description:** {{.Subject.Description}}

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

**If you can generate a confident fix**, output:

=== FILE_START: {{.ResultPath}} ===
# Fix Prompt for AC {{.SubjectID}}: {{.Subject.Description}}

## Target
Apply the change to: `{{.Subject.RequirementsScratchPath}}`

## Required Changes
### Change #N: [description]
**Issue:** <what's wrong>
**Action:** Use the Edit tool on the target file with the exact
old_string and new_string below.
**old_string:**
```
<verbatim slice of the file>
```
**new_string:**
```
<replacement>
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

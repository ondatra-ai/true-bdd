# Generate Fix Prompt for User Story

## Reference Documentation

Read the following documents to understand context before generating fixes:
{{- range $key, $path := .DocPaths }}
- Read(`{{ $path }}`) - {{ $key }}
{{- end }}

## Original User Story

**Story ID:** {{.Subject.ID}}
**Title:** {{.Subject.Title}}

**As a** {{.Subject.AsA}}
**I want** {{.Subject.IWant}}
**So that** {{.Subject.SoThat}}

## Current Acceptance Criteria

Each criterion is shown with the steps it ALREADY has. An AC that
already carries Given/When/Then is not missing them, and rewriting it
would replace work that is already correct.

{{- range $i, $ac := .Subject.AcceptanceCriteria }}
{{ add $i 1 }}. **{{ $ac.ID }}:** {{ $ac.Description }}
{{- if $ac.Steps }}
   Existing steps:
```yaml
{{- range $step := $ac.Steps }}
{{- range $g := $step.Given }}
- given: "{{ $g.Statement }}"{{ if $g.Type }} ({{ $g.Type }}){{ end }}
{{- end }}
{{- range $w := $step.When }}
- when: "{{ $w.Statement }}"{{ if $w.Type }} ({{ $w.Type }}){{ end }}
{{- end }}
{{- range $t := $step.Then }}
- then: "{{ $t.Statement }}"{{ if $t.Type }} ({{ $t.Type }}){{ end }}
{{- end }}
{{- end }}
```
{{- else }}
   Existing steps: NONE — this criterion has no Given/When/Then yet.
{{- end }}
{{- end }}

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

Use these answers to generate a confident fix. Do not ask these questions again.
{{- end }}

{{- if index .UserAnswers "_user_refinement" }}

## ⚠️ REFINEMENT MODE - User Feedback (CRITICAL)

The user has reviewed your PREVIOUS fix prompt and is providing feedback to CORRECT IT:

> {{ index .UserAnswers "_user_refinement" }}

**CRITICAL INSTRUCTIONS FOR REFINEMENT:**
1. **DO NOT ask more questions** - The user is giving you a directive, not asking for options
2. **Address the specific issue** - Fix exactly what the user pointed out
3. **Keep everything else** - Preserve parts of your previous fix that weren't criticized
4. **Output a fix prompt** - You MUST output FILE_START/FILE_END, NEVER QUESTIONS_START/QUESTIONS_END

If the user's feedback is unclear, make your best interpretation and fix it. DO NOT ask for clarification.
{{- end }}

## Your Task

{{- if index .UserAnswers "_user_refinement" }}
**REFINEMENT MODE**: The user has provided feedback on your previous fix. Apply their feedback and regenerate the fix prompt. DO NOT ask questions.
{{- else if .UserAnswers }}
Using the user's clarifications above, generate a complete fix prompt.
{{- else }}
Analyze if you have enough context to generate a confident fix.
{{- end }}

**If you can generate a confident fix**, output:

=== FILE_START: {{.ResultPath}} ===
# Fix Prompt for Story {{.Subject.ID}}: {{.Subject.Title}}

## Instructions
Apply the following changes to the acceptance criteria for this story.

## Original Acceptance Criteria
{{- range $i, $ac := .Subject.AcceptanceCriteria }}
{{ add $i 1 }}. {{ $ac.ID }}: {{ $ac.Description }}{{ if $ac.Steps }} [has steps]{{ else }} [no steps]{{ end }}
{{- end }}

## Required Changes

Change ONLY what the failed check requires. A criterion the check does
not fault is reproduced exactly as it stands — same description, same
steps, word for word. Do not "improve" a passing criterion while you are
in the file: a fix that repairs one AC and quietly rewrites four others
loses work nobody asked you to touch, and the qualifiers it drops are
usually the ones that made the criterion testable.

### Change #N: [AC-ID]
**Before:** <original description>
**Issue:** <what's wrong with it>
**After (description):** <one-line rule-based statement with must/should>
**After (steps):**
```yaml
- given:
    - "<precondition>"
- when:
    - "<action>"
- then:
    - "<outcome>"
```

<Also add any NEW ACs needed for edge cases or missing coverage>

## Complete Fixed Acceptance Criteria

<List ALL ACs after applying changes, ready to copy-paste into the story
file. "All" means the changed ones AND the untouched ones — the block
replaces the whole list, so an omitted criterion is a deleted criterion.
Every AC shown with existing steps above must reappear with those steps
byte for byte unless the failed check is about that criterion.>

```yaml
- ac_id: AC-1
  description: "<one-line rule-based statement>"
  steps:
    - given:
        - "<precondition>"
    - when:
        - "<action>"
    - then:
        - "<outcome>"

- ac_id: AC-2
  description: "<one-line rule-based statement>"
  steps:
    - given:
        - "<precondition>"
    - when:
        - "<action>"
    - then:
        - "<outcome>"
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
      - "<option 3>"
  - id: q2
    question: "<another question if needed>"
    context: "<context>"
    options:
      - "<option>"
=== QUESTIONS_END ===

**Important:**
- Output EXACTLY ONE block (either FILE_START/FILE_END or QUESTIONS_START/QUESTIONS_END)
- Never output both
- Questions must have unique IDs (q1, q2, etc.)
- Each question should have 2-4 suggested options

**When to ASK vs GENERATE:**
{{- if index .UserAnswers "_user_refinement" }}
- **REFINEMENT MODE ACTIVE** - You MUST generate a fix prompt (FILE_START/FILE_END). DO NOT ask questions.
{{- else }}
- **ASK** when:
  - AC implies a user-facing feature with multiple interaction patterns
  - Two or more ACs appear to describe the same behavior (ask before merging)
  - Adding or removing ACs
- **GENERATE** when just converting format:
  - First person → third person (use EXACT role: "{{.Subject.AsA}}")
  - Vague words → specific outcomes
  - Adding missing Given/When/Then structure
{{- end }}

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

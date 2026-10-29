# Propose a step definition body

## Reference Documentation
{{ range $key, $path := .DocPaths }}
- Read(`{{ $path }}`) — {{ $key }}
{{ end }}

## Catalogue phrase

Suite: {{ .Subject.Suite }}
Service: {{ .Subject.Service }}
Phrase: {{ .Subject.Phrase.Template }}
Block: {{ .Subject.Phrase.Block }}
Meaning: {{ .Subject.Phrase.Means }}

Propose only this phrase's body against the declared Public interface and Phrase mapping.
The engine owns the registration, pattern and function signature. Reuse the suite's
state and helpers. Never implement the service's behaviour inside a definition.
Never edit the Specification or service code. Ask for missing context before proposing.

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

When ready, propose only the phrase's body and any shared state it needs.
Preserve registration, pattern and signature. Do not edit other phrases, generated
tests, the Specification or service code. Follow the Phrase mapping through the
declared Public interface; never perform the product's operation yourself.

=== FILE_START: {{.ResultPath}} ===
# Fix Prompt for Item {{.SubjectID}}: {{.Subject.Phrase.Template}}

## Target
{{.Subject.Suite}}

## Required Changes
<paste the body and shared-state changes here>
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

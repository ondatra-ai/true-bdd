# Plan a production repair for the failing set

Read the Specification, the harness and the production source to propose one repair for the whole set. The writer may edit only the relevant services' path and exact writable paths, plus host tmp. Never edit tests or step definitions. First investigate a solution inside those declarations. A preferred undeclared file is not an impossibility; report impossible only if no permitted solution exists, naming the required files and reasons. Tests outside this set are not the repair's subject.

## Declared edit roots
{{ range .WriteRoots }}
- `{{ . }}`
{{ end }}

## Reference Documentation
{{ range $key, $path := .DocPaths }}
- Read(`{{ $path }}`) — {{ $key }}
{{ end }}

## Subject — Failing Set

{{ range .Subject.Tests }}
**Test ID:** {{ .ID }}
**Service:** {{ if .Service }}{{ .Service }}{{ else }}none is known{{ end }}
**Suite:** {{ .Suite }}
**Framework:** {{ .Framework }}
**Test Name:** {{ .TestName }}
**Source File:** {{ .FilePath }}
{{ end }}

### Failure groups
{{ range .Subject.Groups }}
Tests: {{ join ", " .IDs }}
```
{{ .Output }}
```
{{ end }}

### Specification documents
{{ range $key, $path := .Subject.Documents }}
- Read(`{{ $path }}`) — {{ $key }}
{{ end }}

{{ range .Subject.Scenarios }}
### Registry Scenario
**Scenario ID:** {{ .ID }}
**Description:** {{ .Description }}
**Service:** {{ .Service }}
**Path:** {{ .Path }}

{{ .FormatSteps }}
{{ end }}

{{ with .Subject.Specification }}
{{ if .Concepts }}
## What the spec says these steps mean

### Concepts
{{ range .Concepts }}
- `{{ .Concept }}` ({{ .Block }} concept): {{ .Means }}
  Words: {{ if .Terms }}{{ join ", " .Terms }}{{ else }}none{{ end }}
{{ range .Bindings }}  Bound for {{ .Service }} {{ if .Intent }}by intent: {{ .Intent }}{{ else }}through {{ join ", " .References }}: {{ .Does }}{{ end }}
{{ else }}  The architecture binds it for no service of these steps.
{{ end }}{{ end }}

### Words
{{ range .Terms }}
#### {{ .Term }}
{{ range .Means }}- Means: {{ . }}
{{ else }}- The glossary does not define it.
{{ end }}
{{ range .Mappings }}- In the implementation: {{ .Technical }}
- Observed by: {{ .ObservedBy }}
{{ else }}- The architecture's word mapping does not map it.
{{ end }}
{{ end }}
{{ end }}
{{ end }}

## Failed Prompt
{{ .FailedCheck.SectionPath }}: {{ .FailedCheck.Question }}
**Actual:** {{ .FailedCheck.ActualAnswer }}
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

Propose a plan: what to build or change, in which files, and which tests each part serves. Explain the contract and the intended implementation so the writer can investigate and implement it. Do not write replacement files into the plan. Never weaken, skip or remove assertions.

=== FILE_START: {{ .ResultPath }} ===
# Repair plan for {{ .SubjectID }}
<parts, files, intended behaviour, and the set's tests each part serves>
=== FILE_END: {{ .ResultPath }} ===

If clarification is required, output one QUESTIONS_START/QUESTIONS_END block with questions containing id, question, context and options. Return either a plan or questions, never both. In refinement mode return the revised plan.

{{ if .Structured }}
**ANSWER CONTRACT FOR THIS RUN — this supersedes the FILE_START/FILE_END
and QUESTIONS_START/QUESTIONS_END instructions above.** Your final message
must be a single JSON object with all three keys present:

- `impossible`: an array of `{path, reason}` objects naming required undeclared files only when no permitted solution exists, otherwise `[]`.
- `fix_prompt`: the fix instructions, or `""` when you need clarification.
- `questions`: an array of `{id, question, context, options}` objects, or
  `[]` when you are returning a fix prompt.

Exactly one of the three carries content. For impossible work, fix_prompt is empty and questions is []. Do not emit any markers, and do not
wrap the object in markdown fences. Reasoning before the object is ignored.
{{ end }}

{{ if .FailedCheck.Judgement -}}
You propose a repair; change no file. The Product Owner owns concepts and
Story or Scenario wording; the System Architect owns concept bindings.
Read the reference documents:
{{ range $key, $path := .DocPaths }}
- Read(`{{ $path }}`) - {{ $key }}
{{ end }}
Item: {{.SubjectID}}
**Actual:** {{.FailedCheck.ActualAnswer}}
Description: {{.Subject.Description}}
Service: {{.Subject.Service}}
Steps:
{{.Subject.FormatSteps}}
Findings:
{{ range .FailedCheck.Context }}
- {{.}}
{{ end }}
{{ if eq .FailedCheck.Judgement "words" }}
This is the Product Owner's word repair. Propose ONLY a product glossary plus
concept terms change, or a reworded means. No Story, Scenario or architecture
change belongs to this fix; a Word mapping is a later System Architect menu.
{{ end }}
Required repair policy:
{{.FailedCheck.FixPrompt}}
{{ with .UserAnswers }}User answers: {{. | toYaml}}{{ end }}
Return the exact proposed changes in fix_prompt for review at the menu.
Do not apply them. Preserve unaffected steps and concepts. A new variation
widens an existing concept of that item and use; add one only for a new item or use.
Return JSON with fix_prompt and questions. Exactly one carries content:
a complete proposal with questions: [], or questions with fix_prompt: "".
Never return both a proposal and questions. If no schema is supplied, put the proposed fix between these markers:
=== FILE_START: {{.ResultPath}} ===
<exact proposed changes>
=== FILE_END: {{.ResultPath}} ===
{{ else -}}
{{ if eq .FailedCheck.Check "public-interface" -}}
## Reference documents
{{ range $key, $path := .DocPaths }}
Read(`{{ $path }}`) — {{ $key }}
{{ end }}{{ range .Subject.InterfaceDocuments }}
Read(`{{ .Path }}`) — Public interface document
{{ end }}
## Scenario
ID: {{.Subject.ID}}
Service: {{.Subject.Service}}
Description: {{.Subject.Description}}
{{.Subject.FormatSteps}}
### Failed Check: {{.FailedCheck.SectionPath}}
**Question:** {{.FailedCheck.Question}}
**Why it matters:** {{.FailedCheck.Rationale}}
**Actual:** {{.FailedCheck.ActualAnswer}}
## Findings
{{ range .FailedCheck.Context }}- {{.}}
{{ end }}
## Fix template
{{.FailedCheck.FixPrompt}}
## Clarification answers
{{ range $key, $answer := .UserAnswers }}{{ $key }}: {{ $answer }}
{{ end }}
If no clarification answers have arrived, ask the System Architect what this service
offers. Once answered, propose exact YAML for architecture.services[].public_interface,
using only what the answer and Specification declare. Edit the architecture document's
working copy, and an existing interface document's where the answers say it lacks what the
service offers; never a Scenario or the product document. For ui, list elements with page
or region, role and accessible name; for cli/http name an existing document. Do not choose the service's Public interface on the Architect's behalf.
{{ if .Structured }}
Return exactly one JSON object with fix_prompt (string) and questions (array of
{id, question, context, options}). Exactly one carries content. When clarification is
needed return fix_prompt: ""; when proposing return questions: [].
{{ else }}
Return QUESTIONS_START/QUESTIONS_END for questions, or FILE_START/FILE_END carrying
the fix prompt at {{.ResultPath}}. Never both.
{{ end }}
{{ else -}}
{{ if eq .FailedCheck.Check "" -}}
# Generate Fix Prompt for a Registry Scenario

## Reference Documentation
{{- range $key, $path := .DocPaths }}
- Read(`{{ $path }}`) — {{ $key }}
{{- end }}

## Where the fix goes

Where the fix template below says, and nowhere else. A model-judged
scen-check finding is about the words of this one Scenario, its
description or its steps, and is fixed in the Scenario's own entry of
the scenario registry. The generated test rendered from the Scenario is
not edited here: `true-bdd build tests --fix` re-renders it from the
registry once the entry reads correctly. A step definition is never
edited by a scen-check fix.

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
**Why it matters:** {{ .FailedCheck.Rationale }}
**Actual:** {{ .FailedCheck.ActualAnswer }}

**Findings** (the reviewer's own lines):
{{- range .FailedCheck.Context }}
- {{ . }}
{{- end }}

**Fix template:**
{{ .FailedCheck.FixPrompt }}

## Keeping the Scenario in its concepts' words

When the product document declares `concepts:`, every step that does
not open with `llm:` or `judge:` stays an instance of exactly one
concept of its own block, as that concept's closed `means` states it.
A rewritten step keeps to a variation the `means` admits, or rewords
into another concept of its block whose meaning fits, rather than
inventing a variation no concept admits. Read the concepts before you
reword a step.
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
Decide whether the Scenario and the documents already say what the
fixed text must say. When the fix template asks you to ask first and
the Scenario leaves the answer open, ask; otherwise generate the fix.
{{- end }}

**If you can generate a confident fix**, output a fix prompt the applier
can execute against the document the fix template names. The fix MUST:
- Change only what the findings name: the description or the steps
  they object to.
- Keep the behaviour the Scenario specifies. Rewording is the point;
  re-specifying it silently loses coverage.
- In the registry, leave the Scenario's id, `service:`, `path:` and
  `user_stories:` exactly as they are, and every other Scenario too.
- Give each change as the current text, copied from the subject above,
  and the exact new text, so the applier replaces rather than composes.
- Never edit a generated test, a step definition, or a document the fix
  template does not name.

=== FILE_START: {{.ResultPath}} ===
# Fix Prompt for Scenario {{.SubjectID}}: {{.Subject.Description}}

## Target
The `{{.Subject.ID}}` entry of the `scenarios_yaml` document (the
scenario registry).

## Required Changes
### Change #N: [description]
**Document:** scenarios_yaml
**Field:** <description | merged_steps.<given|when|then>>
**Current text:**
```
<the text as it stands>
```
**New text:**
```
<the replacement>
```
=== FILE_END: {{.ResultPath}} ===

**If you need clarification first**, output questions with suggested
answers: each question offers the options you would accept, the one you
recommend first.

=== QUESTIONS_START ===
questions:
  - id: q1
    question: "<your question>"
    context: "<why you need this information>"
    options:
      - "<the option you recommend>"
      - "<another option>"
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
{{ else -}}
**Actual:** {{.FailedCheck.ActualAnswer}}
## Reference documents
{{ range $key, $path := .DocPaths }}Read(`{{ $path }}`) — {{ $key }}
{{ end }}{{ range .Subject.InterfaceDocuments }}Read(`{{ .Path }}`) — Public interface
{{ end }}
## Concept document findings
Scenario: {{.Subject.ID}}
Service: {{.Subject.Service}}
Check: {{.FailedCheck.Check}}
{{ range .FailedCheck.Context }}- {{.}}
{{ end }}
Fix instruction: {{.FailedCheck.FixPrompt}}
{{ range $key, $answer := .UserAnswers }}{{ $key }}: {{ $answer }}
{{ end }}
Ask for the missing meaning or operation before proposing exact YAML to the owner.
Fix the product glossary, architecture.terms or architecture.concepts as the check names.
Bindings use the product's verbatim concept name and block, and their own service.
Every binding reference must be declared in that service's Public interface. For a missing
declaration, also propose its addition to the named Public interface document.
Never propose rewriting a Scenario to repair a concept, term or binding.
{{ if .Structured }}
Return exactly one JSON object with fix_prompt (string) and questions (array of
{id, question, context, options}). Exactly one carries content. When clarification is
needed return fix_prompt: ""; when proposing return questions: [].
{{ else }}
Return QUESTIONS_START/QUESTIONS_END for questions, or FILE_START/FILE_END carrying
the fix prompt at {{.ResultPath}}. Never both.
{{ end }}
{{ end -}}
{{ end -}}
{{ end -}}

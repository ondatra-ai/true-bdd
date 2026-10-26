{{ if eq .FailedCheck.Check "" -}}
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
{{ else -}}
# Generate Fix Prompt for a Story's Step Phrases

## Reference Documentation

Read the following documents before you propose an entry:
{{- range $key, $path := .DocPaths }}
- Read(`{{ $path }}`) - {{ $key }}
{{- end }}

## Where the fix goes

Into the product document's step-phrase catalogue, and nowhere else. A
step-phrase finding says a step of this Story is not written in the
language the product declares, so the step has no stated meaning and no
step definition follows from it. The answer is a catalogue entry,
written once, where every Story and Scenario using the phrase reads it.
The Story itself is never edited by this fix: its steps stay word for
word as the user wrote them.

## Story

**Story ID:** {{.Subject.ID}}
**Title:** {{.Subject.Title}}

**As a** {{.Subject.AsA}}
**I want** {{.Subject.IWant}}
**So that** {{.Subject.SoThat}}

### Acceptance Criteria

Each step is shown as `<block>: <text>`, its text exactly as a catalogue
template must match it, without its `and:` or `but:` keyword.
{{- range $i, $ac := .Subject.AcceptanceCriteria }}

{{ add $i 1 }}. **{{ $ac.ID }}:** {{ $ac.Description }}
```text
{{- range $step := $ac.Steps }}
{{- range $g := $step.Given }}
given: {{ $g.Statement }}
{{- end }}
{{- range $w := $step.When }}
when: {{ $w.Statement }}
{{- end }}
{{- range $t := $step.Then }}
then: {{ $t.Statement }}
{{- end }}
{{- end }}
```
{{- end }}

## Validation Failure

The following check failed and needs to be fixed:

### Failed Check: {{ .FailedCheck.SectionPath }}

**Check:** {{ .FailedCheck.Question }}
**Why it matters:** {{ .FailedCheck.Rationale }}
**Actual:** {{ .FailedCheck.ActualAnswer }}

**Findings** (one line per acceptance criterion, headed by its id):
{{- range .FailedCheck.Context }}
- {{ . }}
{{- end }}

**Fix template:**
{{ .FailedCheck.FixPrompt }}

## The catalogue

A catalogue entry has four fields:

- `template`: the shape of a step, in the grammar below. It is the
  entry's identity: no two entries share one.
- `block`: `given`, `when` or `then`, the one block a step of this shape
  may sit in.
- `means`: what a step of this shape means, in product words a reader
  of the Story understands; it may name the template's slots.
- `terms`: the glossary terms that meaning relies on, each spelled
  exactly as the glossary spells it; the list may be empty.

Every step of the Story must match exactly one template in the whole
catalogue, and that entry's block must be the step's block. A step
opening with `llm:` or `judge:` is run by a model and exempt.

Template grammar, token by token:

- literal text: itself, exactly, case and whitespace included.
- `"{name}"`: one or more characters other than `"`; the slot sits
  between literal double quotes.
- `{name:number}`: digits, or the word `no` for zero.
- `{name:role}`: exactly one `roles[].name` of the product document.
- `{name:pattern}`: the rest of the step, verbatim; only as the
  template's last token, after a space.
- `[a|b]`: exactly one of the literal alternatives.
- `[s]`: `s`, or nothing.
- `\{`, `\}`, `\[`, `\]`, `\\`: the literal character.

Slot names are lower-case letters and underscores, unique within a
template. A template matches a step's whole text, never a part of it.

A good entry phrases the step in the shape other Stories will reuse:
quoted runs and numbers become slots, the words that carry the meaning
stay literal. It must not also match a step another entry already
phrases, or that step becomes ambiguous.

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
Decide whether the Story and the documents already say what each
failing step means. When the fix template asks you to ask first, or the
meaning is not plain, ask; otherwise generate the fix.
{{- end }}

**If you can generate a confident fix**, output a fix prompt the applier
can execute against the product document. The fix MUST:
- Add or change only the entries the findings call for: for a step that
  matches no template, or only a template of another block, one new
  entry phrasing it in its own block; for a step that matches several,
  the change that leaves it matching one.
- Leave every other step of this Story, and every step the other
  entries phrase, matching exactly one template.
- Give the exact YAML of each entry, its template single-quoted, so the
  applier copies it rather than composing it.
- Never edit the Story or another document.

=== FILE_START: {{.ResultPath}} ===
# Fix Prompt for Story {{.Subject.ID}}: {{.Subject.Title}}

## Target
The `product` document: its `catalogue:` list.

## Required Changes
### Change #N: [description]
**Document:** product
**Action:** Append this entry to the end of the `catalogue:` list
(or: replace old_string with new_string, verbatim).
**YAML:**
```yaml
  - template: '<template>'
    block: <given|when|then>
    means: <what a step of this shape means>
    terms: [<glossary term>, ...]
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
{{ end -}}

# Generate Fix Prompt for a Registry Scenario

## Reference Documentation
{{- range $key, $path := .DocPaths }}
- Read(`{{ $path }}`) — {{ $key }}
{{- end }}

## Where the fix goes

Into the term documents, and nowhere else: the product document's
step-phrase catalogue and glossary. A scen-check finding says a phrase
or a word a Scenario relies on has no stated meaning. The answer is a
definition, written once, where every Scenario using the phrase reads
it. The Scenario, the generated test rendered from it and the step
definitions are never edited by a scen-check fix. The fix template below
says which document, and which kind of entry, the failed check wants.

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

**Check:** {{ .FailedCheck.Question }}
**Why it matters:** {{ .FailedCheck.Rationale }}
**Actual:** {{ .FailedCheck.ActualAnswer }}

**Findings** (one line per failing step or entry):
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
  of the Scenario understands; it may name the template's slots.
- `terms`: the glossary terms that meaning relies on, each spelled
  exactly as the glossary spells it; the list may be empty.

Every step of a Scenario a Story claims must match exactly one template
in the whole catalogue, and that entry's block must be the step's block.
A step opening with `llm:` or `judge:` is run by a model and exempt.

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

A good entry phrases the step in the shape other Scenarios will reuse:
quoted runs and numbers become slots, the words that carry the meaning
stay literal. It must not also match a step another entry already
phrases, or that step becomes ambiguous.
{{- if eq .FailedCheck.Check "declared-terms" }}

## The glossary

The product document's `glossary:` list defines the words catalogue
entries declare in their `terms`. A glossary entry has two fields:

- `term`: the word, spelled exactly as the catalogue entries declaring
  it spell it, case and spacing included. A term is looked up by that
  spelling and nothing else.
- `means`: what the word is in this product and why it matters to a
  reader of a Scenario, in product words. It says what the word means,
  never how the code implements it.

Every term a catalogue entry declares, through a step that matches
that entry, must be defined exactly once. A term defined twice has two
meanings, so a reader can rely on neither.

A good definition reads true in every Scenario whose phrases declare
the term, not only in this one: read the `means` of each catalogue
entry declaring it before you write it.
{{- end }}

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
{{- else if eq .FailedCheck.Check "declared-terms" }}
Decide whether the Scenario and the documents already say what each
term the findings name means. When the fix template asks you to ask
first, or the meaning is not plain, ask; otherwise generate the fix.
{{- else }}
Decide whether the Scenario and the documents already say what the
failing step means. When the fix template asks you to ask first, or the
meaning is not plain, ask; otherwise generate the fix.
{{- end }}

**If you can generate a confident fix**, output a fix prompt the applier
can execute against the term document. The fix MUST:
{{- if eq .FailedCheck.Check "declared-terms" }}
- Add or change only the glossary entries the findings call for: for a
  term the glossary does not define, one new entry defining it; for a
  term it defines more than once, a single entry in place of them all.
- Leave the catalogue, and every glossary entry the findings do not
  name, exactly as they are.
- Give the exact YAML of each entry, so the applier copies it rather
  than composing it.
{{- else }}
- Add or change only the entries the findings call for: for a step that
  matches no template, one new entry phrasing it.
- Leave every other step of this Scenario, and every step the other
  entries phrase, matching exactly one template.
- Give the exact YAML of each entry, its template single-quoted, so the
  applier copies it rather than composing it.
{{- end }}
- Never edit the Scenario, a generated test, a step definition or
  another document.

=== FILE_START: {{.ResultPath}} ===
# Fix Prompt for Scenario {{.SubjectID}}: {{.Subject.Description}}
{{- if eq .FailedCheck.Check "declared-terms" }}

## Target
The `product` document: its `glossary:` list.

## Required Changes
### Change #N: [description]
**Document:** product
**Action:** Append this entry to the end of the `glossary:` list
(or: replace old_string with new_string, verbatim).
**YAML:**
```yaml
  - term: <the term, spelled exactly as the catalogue declares it>
    means: <what the word is in this product, and why it matters>
```
{{- else }}

## Target
The `<key>` document: its `catalogue:` list (or its `glossary:` list).

## Required Changes
### Change #N: [description]
**Document:** <key>
**Action:** Append this entry to the end of the `catalogue:` list
(or: replace old_string with new_string, verbatim).
**YAML:**
```yaml
  - template: '<template>'
    block: <given|when|then>
    means: <what a step of this shape means>
    terms: [<glossary term>, ...]
```
{{- end }}
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

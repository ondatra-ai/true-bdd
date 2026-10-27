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
the scenario registry; or it is about what a catalogue phrase means,
and is fixed in that catalogue entry's `means`, in the product
document, where every Scenario using the phrase reads it. The generated
test rendered from the Scenario is not edited here: `true-bdd build
tests --fix` re-renders it from the registry once the entry reads
correctly. A step definition is never edited by a scen-check fix.

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

## Keeping the Scenario in the catalogue's words

When the product document declares a `catalogue:` and a Story claims
this Scenario (its lineage above is not empty), every step that does
not open with `llm:` or `judge:` must match exactly one catalogue
template, of its own block. A rewritten step stays in an existing
template's shape: fill the template's slots with new values, or pick
another entry whose meaning fits, rather than inventing wording no
template matches. Read the catalogue before you reword a step. Slots:
`"{name}"` is a quoted run, `{name:number}` digits or `no`,
`{name:role}` one of the product document's role names, and
`{name:pattern}` the rest of the step, verbatim.

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
- Change only what the findings name: the description, the steps they
  object to, or the `means` of the catalogue entries they name.
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
scenario registry), or the catalogue entries of the `product` document
the findings name.

## Required Changes
### Change #N: [description]
**Document:** <scenarios_yaml | product>
**Field:** <description | merged_steps.<given|when|then> | the means of catalogue entry '<template>'>
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
# Generate Fix Prompt for a Registry Scenario

## Reference Documentation
{{- range $key, $path := .DocPaths }}
- Read(`{{ $path }}`) — {{ $key }}
{{- end }}

## Where the fix goes

{{ if eq .FailedCheck.Check "term-mappings" -}}
Into the term documents, and nowhere else: here the architecture
document's word mapping, its `architecture.terms:` list. A term-mapping
finding says a product word a Scenario relies on has no stated
implementation: nothing says what it is in the code, or how a step
definition sets it up or observes it. The answer is a mapping, written
once, where every Scenario using the word reads it. The Scenario, the
generated test rendered from it, the step definitions and the product
document are never edited by this fix.
{{- else -}}
Into the term documents, and nowhere else: the product document's
step-phrase catalogue and glossary. A scen-check finding says a phrase
or a word a Scenario relies on has no stated meaning. The answer is a
definition, written once, where every Scenario using the phrase reads
it. The Scenario, the generated test rendered from it and the step
definitions are never edited by a scen-check fix. The fix template below
says which document, and which kind of entry, the failed check wants.
{{- end }}

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
{{- else if eq .FailedCheck.Check "term-mappings" }}

## The word mapping

The architecture document's `architecture.terms:` list maps each
product word the catalogue entries declare in their `terms` onto the
implementation. A mapping has three fields:

- `term`: the word, spelled exactly as the catalogue entries declaring
  it spell it, case and spacing included. A term is looked up by that
  spelling and nothing else.
- `technical`: what the word is in the implementation: the process,
  request, record, file or field that carries it, named in words.
- `observed_by`: how a step definition sets the word up or observes
  it, in words a reader can follow to write that definition: what it
  starts, sends or reads, and what it compares. It never names a code
  identifier, so the definition can be rebuilt from it.

Every term a catalogue entry declares, through a step that matches
that entry, must be mapped exactly once. A term mapped twice is built
two ways, so no step definition can follow both.

A good mapping holds for every Scenario whose phrases declare the
term, not only for this one: read the `means` of each catalogue entry
declaring it, the term's glossary entry, and the mappings already in
the architecture document before you write it, and say the same kind
of thing the existing mappings say.
{{- else if eq .FailedCheck.Check "step-definitions" }}

## The step definitions

The suite's step definitions, in the `steps/` directory the findings
name, bind each step's text to code. Catalogue templates and step
definitions must correspond one to one over the steps of every Scenario
a Story claims in that suite: every step matching one template binds to
one and the same definition, and each such definition is reached by
that template alone. A step no definition binds yet is not a finding.

A fix here changes the catalogue only: split a template whose steps
bind several definitions into one entry per definition, or merge the
templates that reach one definition into one entry. The step
definitions are never edited by a scen-check fix; when no catalogue
change can close a finding, say so in the fix prompt instead of
proposing one.
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
{{- else if eq .FailedCheck.Check "term-mappings" }}
Decide whether the Scenario and the documents already say how each
term the findings name is implemented and how a step definition
observes it. When the fix template asks you to ask first, or that is
not plain, ask; otherwise generate the fix.
{{- else if eq .FailedCheck.Check "step-definitions" }}
Decide whether the Scenario and the documents already say which phrases
mean the same thing and which differ. When the fix template asks you to
ask first, or that is not plain, ask; otherwise generate the fix.
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
{{- else if eq .FailedCheck.Check "term-mappings" }}
- Add or change only the mappings the findings call for: for a term
  the architecture does not map, one new entry mapping it; for a term
  it maps more than once, a single entry in place of them all.
- Leave every other part of the architecture document, and the product
  document, exactly as they are.
- Give the exact YAML of each entry, so the applier copies it rather
  than composing it.
{{- else if eq .FailedCheck.Check "step-definitions" }}
- Change only the catalogue entries the findings name: split or merge
  them so each template binds to one step definition and each
  definition is reached by one template.
- Leave every step that matches exactly one template today matching
  exactly one template, and every other entry as it is.
- Give the exact YAML of each entry, its template single-quoted, so the
  applier copies it rather than composing it.
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
{{- else if eq .FailedCheck.Check "term-mappings" }}

## Target
The `architecture_yaml` document: its `architecture.terms:` list.

## Required Changes
### Change #N: [description]
**Document:** architecture_yaml
**Action:** Append this entry to the end of the `architecture.terms:`
list, creating the list under `architecture:` when there is none (or:
replace old_string with new_string, verbatim).
**YAML:**
```yaml
    - term: <the term, spelled exactly as the catalogue declares it>
      technical: <what the word is in the implementation>
      observed_by: <how a step definition sets it up or observes it>
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
{{ end -}}

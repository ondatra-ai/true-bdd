## Subject — Acceptance Criterion

**Story ID:** {{.Subject.StoryID}}
**Story Path:** {{.Subject.StoryPath}}
**AC ID:** {{.Subject.ACID}}
**Lineage Scenario ID:** {{.Subject.LineageScenarioID}}
**Description:** {{.Subject.Description}}
**Scratch Registry Path:** {{.Subject.RequirementsScratchPath}}

### Steps
{{.Subject.FormatSteps}}

---

## Fix Prompt to Apply

{{.FixPrompt}}

---

## Instructions

1. Use the Read tool to load `{{.Subject.RequirementsScratchPath}}` and
   inspect its current state.
2. Apply the changes described in the "Fix Prompt to Apply" section
   above by invoking the Edit tool on
   `{{.Subject.RequirementsScratchPath}}`. Do NOT touch any other path.
3. After the edits succeed, output the confirmation block below.

=== FILE_START: {{.ResultPath}} ===
applied: true
target: {{.Subject.RequirementsScratchPath}}
summary: "<one-line summary of what changed>"
=== FILE_END: {{.ResultPath}} ===

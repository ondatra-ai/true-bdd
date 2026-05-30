
# Test Status Validation

## Purpose
Mirror back the verdict of one failing test after the Go-side runner has
re-executed it. The runner already knows whether the test passes — your
job is to translate the `LastRunPassed` flag into the `answer:` field.

## Instructions
1. Read the subject's `LastRunPassed` field below.
2. If `LastRunPassed: true`, answer `pass`.
3. If `LastRunPassed: false` (or unset), answer `fail`.
4. Do NOT invoke any test runner yourself. Do NOT use Bash to re-run the
   test. Do NOT search for the test on disk. The Go runner is the
   authoritative source of truth for this verdict.
5. If you answer `fail`, copy the F: template verbatim into the
   `fix_prompt:` field exactly as instructed below.
{{- if .Docs }}

## Reference Documentation
{{- range $key, $doc := .Docs }}
- Read(`{{ $doc.FilePath }}`) — {{ $key }}
{{- end }}
{{- end }}

## Subject — Failing Test

**Test ID:** {{.Subject.ID}}
**Service:** {{.Subject.Service}}
**Layer:** {{.Subject.Layer}}
**Framework:** {{.Subject.Framework}}
**Test Name:** {{.Subject.TestName}}
**Source File:** {{.Subject.FilePath}}
**LastRunPassed:** {{.Subject.LastRunPassed}}

### Last Failure Output
```
{{.Subject.FailureOutput}}
```

---

## Validation Question

{{.Question}}{{- if .Rationale }} SO THAT WE ENSURE {{.Rationale}}{{- end }}
{{- if .FixTemplate }}

## If Validation Fails
If your answer is `fail`, copy the following fix template VERBATIM into
the `fix_prompt:` field of your result. Do not paraphrase.

{{ .FixTemplate }}
{{- end }}

## Answer Format
Output your answer using this exact format:

=== FILE_START: {{.ResultPath}} ===
answer: <pass | fail>
context:
  - "LastRunPassed=<true|false> for {{.Subject.ID}}"
{{- if .FixTemplate }}
fix_prompt: |
  <if answer is fail, paste the fix template above verbatim>
  <if answer is pass, omit this field entirely>
{{- end }}
=== FILE_END: {{.ResultPath}} ===

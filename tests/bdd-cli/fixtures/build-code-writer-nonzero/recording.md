# Recording build-code-writer-nonzero

Record this Scenario alone with the architecture's declared record command, prepending this fixture's `record-bin/` to PATH and setting `TRUE_BDD_RECORD_CODEX` to the installed codex executable. The wrapper runs that real writer unchanged, then injects the template's transport outcome after it succeeds. Replay needs neither the wrapper nor the live CLI. The wrapper lives outside `input/` and never enters the host project.

An empty outcome truncates the answer path supplied by `-o` after the real codex turn succeeds; stdout events are not its answer.

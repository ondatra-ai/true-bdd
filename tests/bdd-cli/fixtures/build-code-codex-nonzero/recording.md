# Recording build-code-codex-nonzero

Record this Scenario alone with the architecture's declared record command, prepending this fixture's `record-bin/` to PATH and setting `TRUE_BDD_RECORD_CODEX` to the installed codex executable. The wrapper runs real codex unchanged, preserves its edits and final-message file, then injects exit 7 after success. Replay requires neither wrapper nor live CLI. No codex turn is recorded.

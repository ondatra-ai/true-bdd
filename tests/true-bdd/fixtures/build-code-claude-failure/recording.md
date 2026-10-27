# Recording the failed Claude writer

Run the architecture's declared record command with `-run '^TestBuildCodeClaude006Failure$'`, prepending this fixture's `record-bin/` to PATH and setting `TRUE_BDD_RECORD_CLAUDE` to the installed Claude executable. The wrapper runs real Claude with unchanged arguments and streams, preserves its edits and result, then injects exit 7 only for the fixture's writer request. Replay uses the recorded outcome without a real CLI.

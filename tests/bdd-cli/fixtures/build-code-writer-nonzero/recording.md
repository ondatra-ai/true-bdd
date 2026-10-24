# Recording build-code-writer-nonzero

Record this Scenario alone with the architecture's declared record command, prepending this fixture's `record-bin/` to PATH and setting `TRUE_BDD_RECORD_CRUSH` to the installed crush executable. The wrapper runs that real writer unchanged, then injects the template's transport outcome after it succeeds. Replay needs neither the wrapper nor the live CLI. The wrapper lives outside `input/` and never enters the host project.

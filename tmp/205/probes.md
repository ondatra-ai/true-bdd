# Issue 205 live probes

All probes run in the foreground. They use codex-cli 0.162.1 on macOS, dummy authentication and harmless markers. No historical source or transcript content is read. The real CLI's `sandbox` operation exercises the generated profile without a model turn.

## Final commands and results

```sh
TRUE_BDD_CODEX_SANDBOX_PROBE=/opt/homebrew/bin/codex go test -count=1 -v ./services/bdd-cli/adapters/ai -run '^TestCodex(EveryCommandApplyConfinement|ApplySingleFileAndFileGlobLive|WriterCreatesMissingEditRootsBeforeSpawning|WriterSequentialTurnsProtectEarlierHomesAndVCS)$'
```

The final combined suite passed in 91.920 seconds. It covers all five command appliers, exact-file/glob writes and both retained #201 probes. Earlier all-applier passes took 59.527 and 61.151 seconds. This invokes the actual bootstrap appliers, not five labels on the same router request. `us refine`, `us apply` and `scen check` grant scratch; `build tests` grants configured `tests/*/steps/**`, `tests/new/**`, one existing single file and scratch; `build code` grants its declared service directory and scratch. Each turn denies `~/work`, codex sessions, Claude projects, its generated authentication, sibling scratch, ondatra-ai source/download copies, VCS-cache contents and writes outside its roots. A Node TCP connection returns EPERM. Each runs a selected Go test, node 26.10.0, npm 11.19.1, an actual npm test script, and Docker client 29.8.2. Full commands and observed errors are in `tmp/205/live-probes-final.log` (earlier all-applier passes: `live-command-probes-final.log` and `live-command-probes.log`). An isolated actual-npm-script pass used `-run '^TestCodexEveryCommandApplyConfinement/us_refine$'` and took 14.168 seconds (`live-npm-suite-probe.log`).

```sh
TRUE_BDD_CODEX_SANDBOX_PROBE=/opt/homebrew/bin/codex go test -count=1 -v ./services/bdd-cli/adapters/ai -run '^TestCodexApplySingleFileAndFileGlobLive$'
```

Passed, 5.399 seconds. Existing and absent single files, a file in a safely created missing parent, and an existing `tests/**/*.go` match are writable. Neighboring files, a new wildcard filename outside the frozen match set, and children of directories substituted for either single-file root are denied. `tmp/205/live-file-probes.log` preserves the commands and results.

```sh
TRUE_BDD_CODEX_SANDBOX_PROBE=/opt/homebrew/bin/codex go test -count=1 -v ./services/bdd-cli/adapters/ai -run '^TestCodex(EveryCommandApplyConfinement|WriterCreatesMissingEditRootsBeforeSpawning|WriterSequentialTurnsProtectEarlierHomesAndVCS)$'
```

Passed, 78.488 seconds, during the initial profile matrix. The latter two retained #201 probes grant a missing service root and selected Go replay, deny host-root writes and access to a previous generated home, and exercise case variants, parent traversal and symlink aliases of excluded VCS, home and scratch paths. Both turns remove authentication. The first test was subsequently strengthened to use the actual command appliers, as above. Raw evidence: `tmp/205/live-probes.log`.

## Calibration failures and fixes

The initial `TestCodexEveryCommandApplyConfinement` invocation failed because curl reports connection failure without exposing EPERM; it was replaced by a Node socket probe that asserts the OS error. Repeated isolated commands used the same environment prefix and `-run '^TestCodexEveryCommandApplyConfinement/us_refine$'`; their failed results, in order, exposed unavailable runtime PATH names, missing libnode/linked-library grants, missing Homebrew installation-link metadata and outside OpenSSL configuration. The implementation now enumerates Mach-O dependencies, grants their runtime directories/link metadata, supplies scratch runtime-bin links, and sets `OPENSSL_CONF=/dev/null`. The actual-applier pass followed a separate failure where a wildcard directory anchor incorrectly included a file; anchors now require directories.

The isolated calibration's successive logs were overwritten while debugging; the final outcome is in `tmp/205/probe-debug.log`. Additional preserved experiments:

- `TRUE_BDD_SAVE_PROFILE="$PWD/tmp/205/debug-home/config.toml" TRUE_BDD_CODEX_SANDBOX_PROBE=/opt/homebrew/bin/codex go test -count=1 ./services/bdd-cli/adapters/ai -run '^TestCodexEveryCommandApplyConfinement/us_refine$'` reproduced the library denial and saved configuration only, never authentication (`probe-debug-save.log`). The temporary save hook was removed.
- `CODEX_HOME="$PWD/tmp/205/debug-home" /opt/homebrew/bin/codex sandbox --log-denials -P writer /opt/homebrew/bin/node --version` refused the unresolved executable link (`denials.log`, later overwritten by the next experiment).
- The same command with `/opt/homebrew/Cellar/node/26.10.0_2/bin/node` reached dyld but refused llhttp. `--log-denials` supplied no additional OS records; no missing record was counted as a successful read (`denials.log`).
- `CODEX_HOME="$PWD/tmp/205/debug-home" /opt/homebrew/bin/codex sandbox -P writer /bin/sh -c 'ls -ld /opt/homebrew/opt/llhttp /opt/homebrew/Cellar/llhttp/9.4.3; ls -l /opt/homebrew/Cellar/llhttp/9.4.3/lib/libllhttp.9.4.3.dylib; /usr/bin/stat /opt/homebrew/opt/llhttp/lib/libllhttp.9.4.dylib; /usr/bin/head -c 1 /opt/homebrew/opt/llhttp/lib/libllhttp.9.4.dylib'` showed the canonical library readable and its installation link denied (`runtime-path.log`).
- After adding the opt link directory to that disposable profile, invoking the canonical node path, both with inherited cwd and with `-C /`, aborted during startup (`node-direct.log`). Those experiments used a saved configuration whose original temporary host had been removed. The in-host probe then exposed OpenSSL's outside config, which was disabled, and passed all runtimes. No broader fallback was introduced.

## Recording probes

Each command is the declared record command with one anchored selector:

```sh
go test -tags bdd -json -count=1 -timeout=180m ./tests/bdd-cli/ -mode=services:record,tests:record -run '^TestUsApply002DuplicatesCollapse$'
go test -tags bdd -json -count=1 -timeout=180m ./tests/bdd-cli/ -mode=services:record,tests:record -run '^TestCodexApply001UsRefineWritesScratch$'
go test -tags bdd -json -count=1 -timeout=180m ./tests/bdd-cli/ -mode=services:record,tests:record -run '^TestCodexApply002BuildTestsWritesConfiguredRoot$'
go test -tags bdd -json -count=1 -timeout=180m ./tests/bdd-cli/ -mode=services:record,tests:record -run '^TestBuildTestsSteps001UnboundScenarioGetsTestAndSteps$'
```

All four final recordings passed, in that order. Duplicate-collapse's first attempt failed on snapshot traversal of runtime-bin symlinks; generated state is now skipped before traversal. The first new build-tests attempt used an incomplete fixture config and failed at startup; its input now carries the complete config plus the explicit root. Both rejected shelves remained staged and were never promoted or edited. No usage limit occurred. The manifest records the final status and CLI minutes; the four `tmp/205/record-*.log` files preserve the final runs.

## Configured literal-file regression

The real build-tests applier initially treated `test_write_globs: [./tests/missing/single.txt]` as a directory root. The isolated command `TRUE_BDD_CODEX_SANDBOX_PROBE=/opt/homebrew/bin/codex go test -count=1 -v ./services/bdd-cli/adapters/ai -run '^TestCodexEveryCommandApplyConfinement/build_tests$'` failed with “Is a directory” (`red-configured-file.log`). Separating configured-pattern mode from service-directory mode made that exact command pass in 13.016 seconds (`green-configured-file.log`); the final combined pass includes it. The ordinary test also asserts that preparation never creates a directory at that literal filename.

## npm companion commands

Declared Jest and Playwright replay forms use npx, so the controlled runtime PATH also supplies npm’s npx companion with the same package read grant and offline cache/config. The installed-runtime unit check failed before that grant (`red-npx.log`). `TRUE_BDD_CODEX_SANDBOX_PROBE=/opt/homebrew/bin/codex go test -count=1 -v ./services/bdd-cli/adapters/ai -run '^TestCodexEveryCommandApplyConfinement$'` then passed every command in 62.946 seconds, including `npx --no-install probe-runtime` against a seeded local executable, with no download (`live-npx-probes.log`).

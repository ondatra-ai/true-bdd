# The build code writer may replay tests inside its turn

**Superseded by [ADR-0023](0023-remove-crush-from-product.md).** Historical rationale follows; current writer recovery and confinement follow ADR-0021, ADR-0022 and ADR-0023.

Only `build code --fix`'s crush apply turn receives the declared replay command without machine output, its framework selector and the suite's absolute working directory. ADR-0017's read root remains the project; this amends its boundary only for executing that test command. All engine turns keep general shell permission false. ADR-0022 extends this grant to the Claude build-code writer through its own hook; ADR-0021 grants the codex build-code writer replay through its own permission profile.

## Considered Options

- A full shell, as in #181's prototype: faster writer feedback but unrestricted opportunistic shell use.
- Parse shell syntax in the guard: its splitter is not crush's mvdan shell and cannot establish equivalent execution.
- Keep all tests outside the writer: pays for repeated fix and apply turns.

## Consequences

The canonical form splits the declared replay command with the suite splitter and removes the framework's machine-readable flag only before its argument boundary: `-args` for go-test, a bare `--` for jest and playwright. The boundary and every argument after it pass through unchanged, then use the same canonical quoting rule as the other arguments.

The guard matches exactly the canonical command, or that command plus one selector and a nonempty single-quoted value containing no quote, newline or NUL. Canonical argv uses single quotes around anything beyond A-Za-z0-9_./:=,@%+-. An unrepresentable argument withholds the grant and logs one warning. The command directory resolves against the read root with symbolic links resolved and must equal the suite directory. Crush alone gets bash and job tools; job_output and job_kill are permitted only with a test command, covering tests that crush backgrounds after a minute. Tests run the writer's code with machine permissions: this is a shell-use restriction, not containment of deliberately written code. Network tools keep their current allowance. No shipped prompt template or failing-set lifecycle changes here.

Decided by the Owner in #181's iteration 2 thread and #193. No grilling entry records this grant decision; the parent drill is recorded in `docs/qna/20261022-201524-rebuild-from-spec-drill.yaml`, Q5.

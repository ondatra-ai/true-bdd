# Step coverage is read from source, by the engine

`build tests` walks only the scenarios with a step no definition binds. It used
to learn that by spawning a test — `architecture.testing.commands.coverage`,
which every suite answered with JSON in `$TRUEBDD_COVERAGE_REPORT_DIR` (ADR
0011). The engine now resolves it itself, by parsing `<suite root>/steps/`, and
`coverage:` is gone: from the loader, from the schema, and from this
repository's own architecture document.

This supersedes ADR 0011, whose whole subject was the transport being deleted.

## The doctrine this reverses, and why it was over-strong

`stepcoverage`'s package doc argued the ask had to go to the suite:

> The question is a regexp match against the patterns a suite registered, and
> the suite is the only thing that holds those patterns after they are built —
> some are assembled at registration rather than written as literals, so reading
> the source can only ever approximate the answer.

The assembly is real. It is also **constant-expression** assembly: across all
466 `suite.Step(` call sites in `tests/bdd-cli/steps` and `tests/bdd-web/steps`,
every first argument is a string literal, or literals joined by `+` with one of
19 identifiers, and every one of those is a package-level `const` in the same
package. No `fmt.Sprintf`, no `strings.Join`, no loops, no cross-package
references. A constant folder reproduces that exactly, not approximately.

What actually made source-reading a guess was never the assembly — it was the
habit of **skipping** a pattern the reader could not fold. The extractor already
in this repository (`tests/bdd-cli/steps/registry_steps.go`) handles only
`*ast.BasicLit` and says so: "A non-literal or non-compiling argument is
skipped, not an error." Pointed at `tests/bdd-web/steps` that drops the 131
registrations composed from `selectorPattern` and reports every step they bind
as unbound — a false gap, which looks exactly like real work to a fix turn.

So the precondition for reading source is not that patterns are simple. It is
that **nothing is ever skipped**: anything that does not fold is a refusal
naming the file, the line and the expression, and a pattern that does not
compile fails the whole answer rather than shrinking the table. That turns an
empirical property of today's tree into a checked contract. The failure mode
becomes a stopped run instead of a silent under-report, which is the one thing
the subprocess was there to prevent.

The proof that the mirror is faithful is cheap and was run: with both suites
green, the compiled guard and the engine's scan agree on all 299 scenarios.

## Why not link the suites instead

Because product code cannot see the test tree. `.golangci.yaml`'s `root-services`
depguard list denies `services/**` any import of
`github.com/ondatra-ai/true-bdd/tests`, and `pkg/testkit/bddgo` — which does hold
the resolver — imports `testing`, which has no business in the shipped binary.
Parsing and spawning were the only two candidates; this ADR picks the first.

## The approximation that remains, stated rather than hidden

The scanner counts **every** `Step` call in the steps package, not only those a
call graph says `Register` reaches. Reachability needs types a parser does not
have, and `tests/bdd-web/steps` fans out from `Register` across 104 files
through some eighty helpers. Today the two rules agree — no `register…` helper
is orphaned — and nothing enforces that they keep agreeing.

The direction of the error is what makes this acceptable: counting an
unreachable registration can only turn a gap into a binding, or into an
ambiguity refusal. It can never make a scenario vanish from the answer. A dead
`suite.Step` is dead code; a missed scenario is a false green.

## What a host has to do

Delete `coverage:` from its architecture document. There is no compatibility
shim and none is needed: yamale is strict about unknown keys, so a host that
keeps the key fails schema lint the moment the schema entry goes. That refusal
is the migration notice, exactly as the environment variable's rename was ADR
0011's — `gopkg.in/yaml.v3` decodes non-strictly and would have ignored the key
in silence.

## Consequences

- The answer costs no subprocess and no compile: about 100ms against the ~1.5s
  the `go test` spawn took, before the cold-build case.
- A host whose step definitions are not Go loses narrowing and walks every
  scenario — which is exactly what an absent `coverage:` already did.
- CI loses its cheap red/green on unbound steps: `TestStepCoverage` is gone from
  both suites and from the gate line, so an unbound step now surfaces when
  `build tests` runs. `TestScenarioCoverage` and `TestFixtureTreesArePaired`
  stay hand-written and stay in the gates.
- Three new startup refusals, each pinned by a registry scenario: a pattern that
  does not fold (E2E-299), a step matched by two definitions (E2E-300), and the
  narrowing itself against a host declaring no command at all (E2E-298). ADR
  0011 closed by noting no scenario covered the ask; that gap is closed here.
- The scan runs **before** codegen, so a refusal leaves the tree untouched —
  both refusal fixtures record an empty golden.
- `Statement` gained a `Mode`, so the engine's registry loader now classifies
  `llm:`/`judge:` prefixes as bddgo does and refuses a prefix in the wrong block
  at load. `Statement.Text` keeps the prefix verbatim: the generated test quotes
  `Text`, and bddgo strips it at run time.

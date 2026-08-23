// Command materializer prepares a harness E2E fixture tree inside a
// target directory and prints a machine-readable description of the
// result to stdout as JSON. It is invoked by the Playwright harness
// suite (tests/legacy/bdd-web-playwright/helpers/materializer.ts) and shares its
// primitives — engine-layer overlay, tree copy, checklist prompt
// filtering — with the BDD fixture runner (tests/libraries/runner) instead
// of re-implementing them in TypeScript.
//
// # CLI contract
//
// Materialize a fixture:
//
//	go run ./tests/libraries/materializer \\
//	    -fixture <fixture-dir> -target <target-dir> [-repo <repo-root>]
//
// Recompute the baseline hash of an already-materialized tree:
//
//	go run ./tests/libraries/materializer -list-baseline -target <target-dir>
//
// Flags:
//
//   - -fixture: fixture directory (has fixture.yaml). Required unless -list-baseline is set.
//   - -target: dir to materialize into, created if missing, must be empty if it exists. Required.
//   - -repo: repo root for the engine layer (true-bdd/, templates/); walks up from cwd to a .git dir.
//   - -list-baseline: skip materialization; hash -target, print {"target","baseline"} JSON.
//
// On success the process exits 0 and prints a single JSON object:
//
//	{"fixture": "<fixture dir basename>", "target": "/abs/target/dir",
//	 "base": "engine"|"none", "baseline": {"<slash/relative/path>": "<sha256 hex>", ...},
//	 "teardown": ["<shell command>", ...]}
//
// "baseline" is the tree-hash map taken AFTER base overlay, remove,
// input overlay, checklist filtering, and prep — the mutation-oracle
// input (plan §4.2/§4.5). The root-level `tmp/**` subtree (the
// engine's declared runtime path) is excluded. "teardown" echoes the
// validated teardown commands verbatim; the materializer never runs
// them — the TypeScript side runs them in test-scoped teardown.
//
// Any validation or preparation failure exits non-zero with a clear
// one-line error on stderr and no JSON on stdout. Prep command output
// is forwarded to stderr so stdout stays pure JSON.
//
// # Manifest (fixture.yaml)
//
//	base: engine                    # required: engine | none
//	input: input                    # optional: dir overlaid onto target
//	prep: [npm install]             # optional: bash -c, cwd=target, before baseline hash
//	teardown: [docker compose down] # optional: validated + echoed back only, never run
//	checklist_prompts: {us-refine: ["rule-based format"]}  # filter live checklists
//	remove: [true-bdd/checklists/us-apply.yaml]            # deleted after base, before input
//
// Layering order: (1) base — `engine` copies the live `true-bdd/` and
// `templates/` trees from the repo root, `none` starts from an empty
// dir; (2) `remove` paths are deleted (each must exist — a typo fails
// loudly); (3) `input` is overlaid, files in input win; (4)
// `checklist_prompts` rewrites each `true-bdd/checklists/<stem>.yaml`
// in the target to contain only the prompts matched by the snippets;
// (5) `prep` commands run; (6) the baseline tree hash is taken.
//
// Validation (non-zero exit with a clear error):
//
//   - unknown manifest keys are rejected (strict schema);
//   - base must be exactly "engine" or "none";
//   - a declared input directory must exist;
//   - remove requires base: engine; paths must be clean, relative, non-escaping, and must exist at removal time;
//   - checklist_prompts requires base: engine, and the key must not be declared empty;
//   - each stem must resolve to a live checklist file in the target (wrong stem is an error);
//   - the fixture must not also ship an input override for the same checklist file;
//   - each snippet must match exactly one live NON-SKIPPED prompt (empty, unmatched, ambiguous, duplicate all fail);
//   - semantics shared with tests/libraries/runner via runner.FilterChecklistFile.
package main

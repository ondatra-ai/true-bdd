// Command materializer prepares a harness E2E fixture tree inside a
// target directory and prints a machine-readable description of the
// result to stdout as JSON. It is invoked by the Playwright harness
// suite (harness/tests/e2e/helpers/materializer.ts) and shares its
// primitives — engine-layer overlay, tree copy, checklist prompt
// filtering — with the BDD fixture runner (tests/bdd/runner) instead
// of re-implementing them in TypeScript.
//
// # CLI contract
//
// Materialize a fixture:
//
//	go run ./tests/harness/materializer \
//	    -fixture <fixture-dir> -target <target-dir> [-repo <repo-root>]
//
// Recompute the baseline hash of an already-materialized tree:
//
//	go run ./tests/harness/materializer -list-baseline -target <target-dir>
//
// Flags:
//
//   - -fixture: path of the fixture directory (contains fixture.yaml).
//     Required unless -list-baseline is set.
//   - -target: directory to materialize into. Created if missing; if it
//     already exists it must be empty. Required.
//   - -repo: repository root supplying the engine layer (`true-bdd/` +
//     `templates/`) for `base: engine` fixtures. Defaults to walking up
//     from cwd until a `.git` directory is found.
//   - -list-baseline: skip materialization; hash the existing -target
//     tree and print {"target", "baseline"} JSON. Used by tests that
//     want the Go-side oracle recomputed after a run.
//
// On success the process exits 0 and prints a single JSON object:
//
//	{
//	  "fixture":  "<fixture dir basename>",
//	  "target":   "/abs/target/dir",
//	  "base":     "engine" | "none",
//	  "baseline": { "<slash/relative/path>": "<sha256 hex>", ... },
//	  "teardown": [ "<shell command>", ... ]
//	}
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
//	base: engine            # required: engine | none
//	input: input            # optional: dir (relative to fixture) overlaid onto target
//	prep:                   # optional: shell commands, `bash -c`, cwd=target,
//	  - npm install         #   run BEFORE the baseline hash
//	teardown:               # optional: validated + echoed back only
//	  - docker compose down
//	checklist_prompts:      # optional: filter live checklists to selected prompts
//	  us-refine:
//	    - "rule-based format"
//	remove:                 # optional: paths deleted after the base overlay,
//	  - true-bdd/checklists/us-apply.yaml   # before the input overlay
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
//   - remove requires base: engine; paths must be clean, relative, and
//     non-escaping, and must exist at removal time;
//   - checklist_prompts requires base: engine; the key must not be
//     declared empty; each stem must resolve to a live checklist file
//     in the materialized tree (wrong stem ⇒ error); the fixture must
//     not ALSO ship an input override for the same checklist file;
//     each snippet must match exactly one live NON-SKIPPED prompt
//     (empty, unmatched, ambiguous, and duplicate-resolution snippets
//     all fail) — semantics shared verbatim with tests/bdd/runner via
//     runner.FilterChecklistFile.
package main

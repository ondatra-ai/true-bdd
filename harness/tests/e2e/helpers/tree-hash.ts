/**
 * Filesystem mutation oracle (plan §4.5) — the TypeScript twin of the
 * Go tree hash (tests/harness/materializer/tree_hash.go). Recomputes
 * the same sha256 map at assertion time and diffs it against the
 * materializer's baseline with expected-change matchers.
 *
 * Algorithm parity with Go: keys are slash-separated paths relative
 * to root; values are lowercase sha256 hex of file contents; the
 * ROOT-LEVEL `tmp/` subtree (the engine's declared runtime path) is
 * excluded — a nested "tmp/" dir elsewhere in the tree is NOT.
 */

import crypto from "node:crypto";
import fs from "node:fs";
import path from "node:path";

/** Matches Go's runtimeDir exclusion: root-level tmp/** only. */
const RUNTIME_DIR = "tmp";

export async function hashTree(root: string): Promise<Record<string, string>> {
  const out: Record<string, string> = {};
  await walk(root, root, out);

  return out;
}

async function walk(
  root: string,
  dir: string,
  out: Record<string, string>,
): Promise<void> {
  const entries = await fs.promises.readdir(dir, { withFileTypes: true });

  for (const entry of entries) {
    const full = path.join(dir, entry.name);
    const rel = path.relative(root, full).split(path.sep).join("/");

    if (entry.isDirectory()) {
      if (rel === RUNTIME_DIR) continue;

      await walk(root, full, out);
      continue;
    }

    // Non-directories (incl. symlinks) are hashed by content, exactly
    // like Go's os.ReadFile after WalkDir.
    const data = await fs.promises.readFile(full);
    out[rel] = crypto.createHash("sha256").update(data).digest("hex");
  }
}

export interface TreeDiff {
  created: string[];
  modified: string[];
  deleted: string[];
}

export function diffTrees(
  baseline: Record<string, string>,
  current: Record<string, string>,
): TreeDiff {
  const created: string[] = [];
  const modified: string[] = [];
  const deleted: string[] = [];

  for (const [rel, hash] of Object.entries(current)) {
    const before = baseline[rel];
    if (before === undefined) {
      created.push(rel);
    } else if (before !== hash) {
      modified.push(rel);
    }
  }

  for (const rel of Object.keys(baseline)) {
    if (!(rel in current)) {
      deleted.push(rel);
    }
  }

  created.sort();
  modified.sort();
  deleted.sort();

  return { created, modified, deleted };
}

export function isEmptyDiff(diff: TreeDiff): boolean {
  return diff.created.length === 0 && diff.modified.length === 0 && diff.deleted.length === 0;
}

function describeDiff(diff: TreeDiff): string {
  const lines = [
    ...diff.created.map((p) => `  created:  ${p}`),
    ...diff.modified.map((p) => `  modified: ${p}`),
    ...diff.deleted.map((p) => `  deleted:  ${p}`),
  ];

  return lines.length === 0 ? "  (no changes)" : lines.join("\n");
}

/**
 * Converts a glob to an anchored RegExp. Supports `**` (any chars
 * incl. `/`), `*` (any chars except `/`), and `?` (one char except
 * `/`). Everything else is literal.
 */
export function globToRegExp(glob: string): RegExp {
  let pattern = "";
  let index = 0;

  while (index < glob.length) {
    const char = glob[index];

    if (char === "*") {
      if (glob[index + 1] === "*") {
        // Deliberately simple semantics: "**" is ".*" (so "a/**/b"
        // requires the slashes literally present around the wildcard).
        pattern += ".*";
        index += 2;
        continue;
      }

      pattern += "[^/]*";
      index += 1;
      continue;
    }

    if (char === "?") {
      pattern += "[^/]";
      index += 1;
      continue;
    }

    pattern += char.replace(/[.+^${}()|[\]\\]/g, "\\$&");
    index += 1;
  }

  return new RegExp(`^${pattern}$`);
}

/**
 * Asserts the tree at `root` is byte-identical to the baseline
 * (canonical files only — tmp/** is outside the hash). Oracle for
 * A0/A3/A7/A8/A9.
 */
export async function expectNoCanonicalChange(
  baseline: Record<string, string>,
  root: string,
): Promise<void> {
  const diff = diffTrees(baseline, await hashTree(root));

  if (!isEmptyDiff(diff)) {
    throw new Error(`expected NO canonical change, but the tree differs:\n${describeDiff(diff)}`);
  }
}

/**
 * Asserts every created/modified/deleted path matches one of the
 * allowed globs (exact paths are globs without wildcards). Returns
 * the diff for further assertions. Oracle for A2/A4/A5/A6.
 */
export async function expectOnlyPathsChanged(
  baseline: Record<string, string>,
  root: string,
  allowedGlobs: string[],
): Promise<TreeDiff> {
  const diff = diffTrees(baseline, await hashTree(root));
  const patterns = allowedGlobs.map(globToRegExp);

  const offenders = [...diff.created, ...diff.modified, ...diff.deleted].filter(
    (rel) => !patterns.some((re) => re.test(rel)),
  );

  if (offenders.length > 0) {
    throw new Error(
      `paths changed outside the allowed set [${allowedGlobs.join(", ")}]:\n` +
        offenders.map((p) => `  ${p}`).join("\n") +
        `\nfull diff:\n${describeDiff(diff)}`,
    );
  }

  return diff;
}

/**
 * Asserts the run created EXACTLY one new file matching the glob and
 * changed nothing else. Returns the created path. Oracle for A1.
 */
export async function expectExactlyOneNewFileMatching(
  baseline: Record<string, string>,
  root: string,
  glob: string,
): Promise<string> {
  const diff = diffTrees(baseline, await hashTree(root));
  const pattern = globToRegExp(glob);
  const matching = diff.created.filter((rel) => pattern.test(rel));

  const clean =
    matching.length === 1 &&
    diff.created.length === 1 &&
    diff.modified.length === 0 &&
    diff.deleted.length === 0;

  if (!clean) {
    throw new Error(
      `expected exactly one new file matching ${glob} and nothing else; got:\n${describeDiff(diff)}`,
    );
  }

  return matching[0];
}

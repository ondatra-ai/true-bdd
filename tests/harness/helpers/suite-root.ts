/**
 * Suite-root helper (plan §4.1).
 *
 * Global setup creates ONE unique suite root per invocation via
 * mkdtemp — never a fixed path — under the engine repo's gitignored
 * `tmp/` directory, and builds the Go binaries ONCE into it:
 *
 *   <suiteRoot>/bin/true-bdd       — `go build -o ... ./src`
 *   <suiteRoot>/bin/materializer   — `go build -o ... ./tests/materializer`
 *
 * The paths are handed to test workers through environment variables
 * (set in globalSetup, inherited by workers). Tests and helpers call
 * `suiteContext()` to read them.
 */

import { execFile } from "node:child_process";
import { promisify } from "node:util";
import fs from "node:fs";
import path from "node:path";

const execFileAsync = promisify(execFile);

const ENV_SUITE_ROOT = "TRUE_BDD_E2E_SUITE_ROOT";
const ENV_REPO_ROOT = "TRUE_BDD_E2E_REPO_ROOT";
const ENV_BIN = "TRUE_BDD_E2E_BIN";
const ENV_MATERIALIZER_BIN = "TRUE_BDD_E2E_MATERIALIZER_BIN";

export interface SuiteContext {
  /** Unique per-invocation scratch root (under `<repo>/tmp/`). */
  suiteRoot: string;
  /** Engine repository root (contains .git, go.mod, true-bdd/). */
  repoRoot: string;
  /** The harness app directory (`<repo>/harness`). */
  harnessDir: string;
  /** Absolute path of the freshly built `true-bdd` binary. */
  binPath: string;
  /** Absolute path of the freshly built fixture materializer binary. */
  materializerBin: string;
}

/** Walks up from this file until a `.git` directory is found. */
export function findRepoRoot(startDir: string = __dirname): string {
  let dir = path.resolve(startDir);

  for (;;) {
    if (fs.existsSync(path.join(dir, ".git"))) {
      return dir;
    }

    const parent = path.dirname(dir);
    if (parent === dir) {
      throw new Error(`repo root (with .git) not found above ${startDir}`);
    }

    dir = parent;
  }
}

/**
 * Creates the suite root and builds both Go binaries into it.
 * Called exactly once, from global-setup. Exposes the context to the
 * worker processes via environment variables.
 */
export async function createSuiteContext(): Promise<SuiteContext> {
  const repoRoot = findRepoRoot();
  const harnessDir = path.join(repoRoot, "harness");

  // Repo convention: temporary artefacts live under the repo's
  // gitignored ./tmp/, not the system temp dir. mkdtemp keeps the
  // root unique per invocation (plan §4.1: "never a fixed path").
  const tmpBase = path.join(repoRoot, "tmp");
  fs.mkdirSync(tmpBase, { recursive: true });
  const suiteRoot = fs.mkdtempSync(path.join(tmpBase, "harness-e2e-"));

  const binDir = path.join(suiteRoot, "bin");
  fs.mkdirSync(binDir, { recursive: true });

  const binPath = path.join(binDir, "true-bdd");
  const materializerBin = path.join(binDir, "materializer");

  await execFileAsync("go", ["build", "-o", binPath, "./src"], {
    cwd: repoRoot,
  });
  await execFileAsync(
    "go",
    ["build", "-o", materializerBin, "./tests/materializer"],
    { cwd: repoRoot },
  );

  process.env[ENV_SUITE_ROOT] = suiteRoot;
  process.env[ENV_REPO_ROOT] = repoRoot;
  process.env[ENV_BIN] = binPath;
  process.env[ENV_MATERIALIZER_BIN] = materializerBin;

  return { suiteRoot, repoRoot, harnessDir, binPath, materializerBin };
}

/**
 * Reads the suite context prepared by global-setup. Throws when called
 * outside a `playwright test` invocation (e.g. from vitest).
 */
export function suiteContext(): SuiteContext {
  const suiteRoot = process.env[ENV_SUITE_ROOT];
  const repoRoot = process.env[ENV_REPO_ROOT];
  const binPath = process.env[ENV_BIN];
  const materializerBin = process.env[ENV_MATERIALIZER_BIN];

  if (!suiteRoot || !repoRoot || !binPath || !materializerBin) {
    throw new Error(
      "suite context env not set — did global-setup run? (helpers require `playwright test`)",
    );
  }

  return {
    suiteRoot,
    repoRoot,
    harnessDir: path.join(repoRoot, "harness"),
    binPath,
    materializerBin,
  };
}

/** Creates a unique subdirectory under the suite root. */
export function mkdirUnderSuiteRoot(prefix: string): string {
  const { suiteRoot } = suiteContext();

  return fs.mkdtempSync(path.join(suiteRoot, `${prefix}-`));
}

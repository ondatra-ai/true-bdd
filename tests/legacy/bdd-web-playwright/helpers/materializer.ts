/**
 * TypeScript wrapper around the Go fixture materializer
 * (tests/libraries/materializer — see its doc.go for the CLI contract,
 * plan §4.2). The materializer is shared with the BDD runner's Go
 * primitives; nothing here re-implements overlay or checklist-filter
 * semantics.
 */

import { execFile } from "node:child_process";
import { promisify } from "node:util";
import fs from "node:fs";
import path from "node:path";

import { mkdirUnderSuiteRoot, suiteContext } from "./suite-root";

const execFileAsync = promisify(execFile);

const TEARDOWN_TIMEOUT_MS = 2 * 60 * 1000;

export interface MaterializedFixture {
  /** Fixture directory basename. */
  fixture: string;
  /** Absolute materialized host-project folder (the remote's cwd). */
  target: string;
  base: "engine" | "none";
  /**
   * Baseline tree hash (slash-relative path → sha256 hex), taken
   * after base overlay / remove / input overlay / checklist filter /
   * prep. Root-level tmp/** excluded. The mutation-oracle input.
   */
  baseline: Record<string, string>;
  /** Fixture-declared teardown commands — run via runFixtureTeardown. */
  teardown: string[];
}

/** Resolves a fixture name to `tests/bdd-web/fixtures/<name>`. */
export function fixtureDir(name: string): string {
  return path.join(suiteContext().repoRoot, "tests", "bdd-web", "fixtures", name);
}

/**
 * Materializes a fixture into targetDir (default: a fresh unique dir
 * under the suite root) by shelling to the suite-built materializer
 * binary. Throws with the materializer's stderr on any validation or
 * preparation failure.
 *
 * @param nameOrDir fixture name (resolved against tests/bdd-web/fixtures)
 *                  or an absolute fixture directory path.
 */
export async function materializeFixture(
  nameOrDir: string,
  targetDir?: string,
): Promise<MaterializedFixture> {
  const { materializerBin, repoRoot } = suiteContext();
  const fixture = path.isAbsolute(nameOrDir) ? nameOrDir : fixtureDir(nameOrDir);
  const target = targetDir ?? path.join(mkdirUnderSuiteRoot("fixture"), "host");

  let stdout: string;
  try {
    ({ stdout } = await execFileAsync(
      materializerBin,
      ["-fixture", fixture, "-target", target, "-repo", repoRoot],
      { maxBuffer: 64 * 1024 * 1024 },
    ));
  } catch (error) {
    const err = error as { stderr?: string; message?: string };
    throw new Error(
      `materializer failed for ${fixture}: ${err.stderr?.trim() || err.message}`,
    );
  }

  return JSON.parse(stdout) as MaterializedFixture;
}

/**
 * Recomputes the tree hash of an already-materialized target via the
 * GO side (`-list-baseline`) — the cross-implementation check against
 * the TS oracle in tree-hash.ts.
 */
export async function listBaseline(targetDir: string): Promise<Record<string, string>> {
  const { materializerBin } = suiteContext();

  const { stdout } = await execFileAsync(
    materializerBin,
    ["-list-baseline", "-target", targetDir],
    { maxBuffer: 64 * 1024 * 1024 },
  );

  const doc = JSON.parse(stdout) as { baseline: Record<string, string> };

  return doc.baseline;
}

/**
 * Runs the fixture's declared teardown commands (`bash -c`, cwd =
 * target) — call from test-scoped teardown AFTER process cleanup.
 * Best-effort: failures are collected into the returned list, never
 * thrown, so teardown can't mask the test verdict (mirrors the BDD
 * runner's teardown discipline).
 */
export async function runFixtureTeardown(
  fixture: Pick<MaterializedFixture, "target" | "teardown">,
): Promise<string[]> {
  const failures: string[] = [];

  for (const command of fixture.teardown) {
    try {
      await execFileAsync("bash", ["-c", command], {
        cwd: fixture.target,
        timeout: TEARDOWN_TIMEOUT_MS,
      });
    } catch (error) {
      const err = error as { message?: string };
      failures.push(`teardown failed (${command}): ${err.message}`);
    }
  }

  return failures;
}

/** True when the fixture directory exists (guards typoed names early). */
export function fixtureExists(name: string): boolean {
  return fs.existsSync(path.join(fixtureDir(name), "fixture.yaml"));
}

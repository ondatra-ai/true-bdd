/**
 * Reproduction manifest (plan §4.1): on failure, everything needed to
 * re-run the scenario by hand lands in the test's output dir —
 * fixture, repo commit, binary hash, model, port, DB path, session/run
 * ids, PGIDs, log paths, and the final filesystem diff.
 */

import { execFileSync } from "node:child_process";
import crypto from "node:crypto";
import fs from "node:fs";

import type { TestInfo } from "@playwright/test";

import { suiteContext } from "./suite-root";
import { diffTrees, hashTree, type TreeDiff } from "./tree-hash";

export interface ReproManifestInput {
  /** Fixture name (and any other ids the test tracked). */
  fixture?: string;
  model?: string;
  port?: number;
  dbPath?: string;
  sessionId?: string;
  runId?: string;
  /** Process-group ids of server/remote/children at failure time. */
  pgids?: number[];
  /** server/remote log file paths. */
  logPaths?: string[];
  /** Materializer baseline — enables the final tree diff. */
  baseline?: Record<string, string>;
  /** Materialized target root — enables the final tree diff. */
  treeRoot?: string;
  /** Anything else worth pinning (client tokens, seqs, ...). */
  extra?: Record<string, unknown>;
}

export interface ReproManifest {
  test: string;
  status: string;
  fixture?: string;
  repoCommit: string;
  binaryPath: string;
  binarySha256: string;
  model?: string;
  port?: number;
  dbPath?: string;
  sessionId?: string;
  runId?: string;
  pgids?: number[];
  logPaths?: string[];
  finalTreeDiff?: TreeDiff;
  extra?: Record<string, unknown>;
  suiteRoot: string;
  writtenAt: string;
}

function repoCommit(repoRoot: string): string {
  try {
    return execFileSync("git", ["-C", repoRoot, "rev-parse", "HEAD"], {
      encoding: "utf8",
    }).trim();
  } catch {
    return "unknown";
  }
}

function fileSha256(filePath: string): string {
  try {
    return crypto.createHash("sha256").update(fs.readFileSync(filePath)).digest("hex");
  } catch {
    return "unknown";
  }
}

/**
 * Writes `repro-manifest.json` into the test's output dir and attaches
 * it to the report. Returns the file path.
 */
export async function writeReproManifest(
  testInfo: TestInfo,
  input: ReproManifestInput,
): Promise<string> {
  const { repoRoot, binPath, suiteRoot } = suiteContext();

  let finalTreeDiff: TreeDiff | undefined;
  if (input.baseline !== undefined && input.treeRoot !== undefined) {
    try {
      finalTreeDiff = diffTrees(input.baseline, await hashTree(input.treeRoot));
    } catch {
      // The tree may be gone (teardown raced); the manifest still helps.
    }
  }

  const manifest: ReproManifest = {
    test: testInfo.titlePath.join(" > "),
    status: testInfo.status ?? "unknown",
    fixture: input.fixture,
    repoCommit: repoCommit(repoRoot),
    binaryPath: binPath,
    binarySha256: fileSha256(binPath),
    model: input.model,
    port: input.port,
    dbPath: input.dbPath,
    sessionId: input.sessionId,
    runId: input.runId,
    pgids: input.pgids,
    logPaths: input.logPaths,
    finalTreeDiff,
    extra: input.extra,
    suiteRoot,
    writtenAt: new Date().toISOString(),
  };

  const outPath = testInfo.outputPath("repro-manifest.json");
  fs.mkdirSync(testInfo.outputDir, { recursive: true });
  fs.writeFileSync(outPath, `${JSON.stringify(manifest, null, 2)}\n`);
  await testInfo.attach("repro-manifest", { path: outPath, contentType: "application/json" });

  return outPath;
}

/**
 * Convenience for test-scoped teardown: writes the manifest only when
 * the test did not end as expected. Returns the path when written.
 */
export async function writeReproManifestOnFailure(
  testInfo: TestInfo,
  input: ReproManifestInput,
): Promise<string | undefined> {
  if (testInfo.status === testInfo.expectedStatus) {
    return undefined;
  }

  return writeReproManifest(testInfo, input);
}

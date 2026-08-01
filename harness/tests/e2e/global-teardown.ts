/**
 * Global teardown (plan §4.1): the suite-level EMERGENCY process
 * cleanup. Test-scoped teardown is the primary path; this catches
 * process groups leaked by crashed or timed-out tests (servers,
 * remotes, claude descendants) via the suite process registry.
 *
 * The suite root itself is preserved — like the BDD runner's tmpdirs,
 * artifacts are never auto-cleaned; wipe `<repo>/tmp/harness-e2e-*`
 * manually to reclaim disk.
 */

import { emergencyKillAll } from "./helpers/process-registry";
import { takeRedisDown } from "./helpers/redis";
import { suiteContext } from "./helpers/suite-root";

export default async function globalTeardown(): Promise<void> {
  let suiteRoot = "(unknown)";
  try {
    suiteRoot = suiteContext().suiteRoot;
  } catch {
    // Global setup failed before exporting the context — nothing to clean.
    return;
  }

  const stale = await emergencyKillAll();
  if (stale.length > 0) {
    console.warn(
      `[harness-e2e] emergency cleanup killed leaked process group(s): ${stale.join(", ")} — ` +
        "a test's scoped teardown failed to reap them",
    );
  }

  // Stop the Redis compose stack ONCE (plan: per-test `compose down` would kill
  // the Redis other sequential tests reuse). Best-effort: a failed stop never
  // masks the suite verdict.
  await takeRedisDown();

  console.log(`[harness-e2e] artifacts preserved at: ${suiteRoot}`);
}

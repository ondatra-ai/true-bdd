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
import { sweepHarnessContainers } from "./helpers/server-controller";
import { suiteContext } from "./helpers/suite-root";

export default async function globalTeardown(): Promise<void> {
  // Mirror of the global-setup goldens guard: nothing was brought up.
  if (process.env.UPDATE_GOLDENS === "1") {
    return;
  }

  let suiteRoot: string | undefined;
  try {
    suiteRoot = suiteContext().suiteRoot;
  } catch {
    // Global setup failed before exporting the context — the process registry
    // may be absent, so skip emergency cleanup. Redis is still stopped below: a
    // partial setup may have already brought the compose stack up.
    suiteRoot = undefined;
  }

  try {
    if (suiteRoot !== undefined) {
      const stale = await emergencyKillAll();
      if (stale.length > 0) {
        console.warn(
          `[harness-e2e] emergency cleanup killed leaked process group(s): ${stale.join(", ")} — ` +
            "a test's scoped teardown failed to reap them",
        );
      }

      // Per-instance harness containers/networks leaked by a crashed test
      // (ServerController.stop() is the primary path; this is the safety net).
      const swept = await sweepHarnessContainers();
      if (swept > 0) {
        console.warn(
          `[harness-e2e] swept ${swept} leaked harness container(s) — a test's scoped teardown failed to down them`,
        );
      }
    }
  } finally {
    // Stop the Redis compose stack ONCE, on EVERY exit path — even when
    // emergencyKillAll throws or setup failed before exporting the context
    // (plan: per-test `compose down` would kill the Redis other sequential
    // tests reuse). Best-effort: a failed stop never masks the suite verdict.
    await takeRedisDown();
  }

  if (suiteRoot !== undefined) {
    console.log(`[harness-e2e] artifacts preserved at: ${suiteRoot}`);
  }
}

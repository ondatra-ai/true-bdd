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
            "a test's scoped teardown failed to reap them (host `node server.js` server or a remote)",
        );
      }
    }
  } finally {
    // Redis is the lone singleton container; keep it WARM across local
    // invocations so back-to-back runs start fast — per-test REDIS_KEY_PREFIXes
    // + flushPrefix keep it hygienic despite the shared instance. Only tear it
    // down under CI (hermetic there) or when explicitly asked. Best-effort: a
    // failed stop never masks the suite verdict.
    if (process.env.CI || process.env.TRUE_BDD_E2E_DOWN_REDIS === "1") {
      await takeRedisDown();
    }
  }

  if (suiteRoot !== undefined) {
    console.log(`[harness-e2e] artifacts preserved at: ${suiteRoot}`);
  }
}

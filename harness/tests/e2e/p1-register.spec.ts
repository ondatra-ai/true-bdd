/**
 * P1 (plan §4.3) — register, bare folder: a remote started THROUGH a
 * symlinked cwd in a `base: none` fixture registers a session whose
 * displayed folder equals realpath(cwd) (never the symlink), and the
 * inventory honestly reports the missing true-bdd.yaml.
 */

import fs from "node:fs";
import path from "node:path";

import { expect, test } from "@playwright/test";

import { ProtocolEnv } from "./helpers/protocol-env";
import { TID, gotoSession, gotoSessions, inventoryDoc, sessionRow } from "./helpers/ui";

let env: ProtocolEnv | undefined;

test.afterEach(async () => {
  // Extend (never overwrite) the shared budget so scoped teardown has
  // room even after a test-body timeout (Playwright's documented idiom).
  const info = test.info();
  info.setTimeout(info.timeout + 60_000);
  const current = env;
  env = undefined;

  if (current !== undefined) {
    await current.teardown(test.info());
  }
});

test("P1: bare-folder registration through a symlinked cwd shows the realpath", async ({ page }) => {
  env = await ProtocolEnv.start("p1-register");
  const e = env;

  const fixture = await e.materialize("bare-host");
  const realFolder = fs.realpathSync(fixture.target);

  // Start the remote THROUGH a symlink: cwd and $PWD both point at the
  // link, so a lazy implementation would display the link path. The
  // contract (plan §3.1) is the canonical realpath.
  const linkPath = path.join(e.dir, "host-via-symlink");
  fs.symlinkSync(fixture.target, linkPath, "dir");
  expect(realFolder).not.toBe(linkPath);

  const remote = await e.startRemote(linkPath, { env: { PWD: linkPath } });

  const session = await e.api.waitForSession((candidate) => candidate.pid === remote.pid);
  e.note({ sessionId: session.id });

  expect(session.folder).toBe(realFolder);
  expect(session.folder).not.toBe(linkPath);
  expect(session.reachability).toBe("connected");

  // The bare folder's first inventory snapshot must be promoted before
  // the chips can render honestly.
  await e.api.waitForGeneration(session.id, 0);

  // Sessions list renders the canonical folder, not the symlink.
  await gotoSessions(page, e.server.baseURL);
  const row = sessionRow(page, session.id);
  await expect(row).toBeVisible({ timeout: 15_000 });
  await expect(row.getByTestId(TID.sessionFolder)).toHaveText(realFolder);
  await expect(row.getByTestId(TID.sessionReachability)).toHaveText("connected");

  // Bare folder: the inventory reports the missing engine config
  // honestly — no eager bootstrap (plan §3.1).
  await gotoSession(page, e.server.baseURL, session.id);
  await expect(inventoryDoc(page, "config")).toHaveAttribute("data-status", "missing", {
    timeout: 15_000,
  });
});

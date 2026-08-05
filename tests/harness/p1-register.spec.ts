/**
 * P1 (plan §4.3, reworked for `home-sessions-list` H1) — register, bare folder:
 * a remote started THROUGH a symlinked cwd in a `base: none` fixture registers a
 * session whose displayed folder equals realpath(cwd) (never the symlink),
 * proven at BOTH the API (`GET /api/sessions`) and the live sessions list (`/`).
 *
 * The `/sessions/<sid>` detail-inventory half was retired here: session-detail
 * is out of scope for `home-sessions-list`, and the list-level sessions home is
 * the surface this spec now shares with `w18-*`. Symlink→realpath is proven at
 * the list; the general row anatomy lives in w18.2.
 */

import fs from "node:fs";
import path from "node:path";

import { expect, test } from "@playwright/test";

import { ProtocolEnv } from "./helpers/protocol-env";
import { TID, gotoSessions, sessionRow } from "./helpers/ui";

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
  e.note({ sessionId: session.session_id });

  expect(session.folder).toBe(realFolder);
  expect(session.folder).not.toBe(linkPath);

  // Sessions list renders the canonical folder, not the symlink (list level).
  await gotoSessions(page, e.server.baseURL);
  const row = sessionRow(page, session.session_id);
  await expect(row).toBeVisible({ timeout: 15_000 });
  await expect(row.getByTestId(TID.sessionFolder)).toHaveText(realFolder);
});

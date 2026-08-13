/**
 * w10 — workspace-overview design parity (task `workspace-overview-design-parity`,
 * R1–R5). DETERMINISTIC specs that drive the `w-workspace-happy` fixture through
 * `WorkspaceEnv` and assert the production `/sessions/<sid>/home` canvas renders
 * the design baseline's content surfaces (the prototype's /workspace-overview):
 * title + metadata, the build-actions row (wired to dispatch/read, disabled on an
 * own active run), the inventory-health list with status chips, the degraded-state
 * banners (absent on a clean read, present when SessionDetail reports one), and the
 * two-level breadcrumb trail. All data comes from the EXISTING browser API surface
 * (`GET /api/sessions` folder, `SessionDetail` status + inventory, the run-dispatch
 * route) — no new engine endpoint. Degraded/own-active states are driven
 * deterministically by injecting the existing SessionDetail shape via a route
 * override (no new fixture), never fabricated behavior.
 *
 * RED intent: the current `/home` is a STUB (`home-landing` with a bare heading),
 * so every case fails as an ASSERTION against an absent overview surface — not a
 * crash, an unresolved route, or a missing session. The persistent shell (rail,
 * sidebar, breadcrumb) already renders, and `WorkspaceEnv.start` proves the
 * session is live before any assertion runs.
 */

import { expect, test } from "@playwright/test";

import {
  INVENTORY_STATUSES,
  WTID,
  gotoWorkspace,
  overviewBanner,
  overviewInventoryRow,
  wsRoutes,
} from "./helpers/ui";
import { WorkspaceEnv } from "./helpers/workspace-env";

/**
 * A synthetic `SessionDetail` (the documented status + inventory shape) JSON
 * string, used ONLY by the route-injection cases to drive a DETERMINISTIC state
 * (own active run / degraded inventory) into the overview without a real run or a
 * new fixture. Built from scratch rather than fetched: the session-detail relay
 * route the overview reads is part of what this task adds, so a spec must never
 * GATE on it — the injection intercepts the browser's fetch whether or not the
 * server route exists yet, and the RED failure stays an assertion on the missing
 * overview surface.
 */
function syntheticDetail(sid: string, overrides: Record<string, unknown>): string {
  return JSON.stringify({
    session_id: sid,
    owner_id: "own-owner",
    active_run: null,
    runs: [],
    active_owners: [],
    inventory: { snapshot_truncated: false, stories_omitted: 0, limit_too_small: false },
    ...overrides,
  });
}

/** Matches the session-detail read the overview issues (any query, e.g. ?view=status). */
const detailPredFor = (sid: string) => (url: URL): boolean => url.pathname === `/api/sessions/${sid}`;

let env: WorkspaceEnv | undefined;

test.afterEach(async () => {
  const info = test.info();
  info.setTimeout(info.timeout + 60_000);
  const current = env;
  env = undefined;
  if (current !== undefined) {
    await current.teardown(test.info());
  }
});

test("w10.1 the overview title carries the session folder + id in its metadata line (R1)", async ({ page }) => {
  env = await WorkspaceEnv.start("w10-overview-meta");
  const e = env;

  // Drive the expected folder from the SAME existing API the page reads.
  const sessions = await e.api.listSessions();
  const summary = sessions.find((s) => s.session_id === e.sid);
  expect(summary, "the live session is missing from GET /api/sessions").toBeDefined();
  const folder = summary!.folder;
  expect(folder, "session folder is empty").not.toBe("");

  await gotoWorkspace(page, e.baseURL, wsRoutes.home(e.sid));

  await expect(page.getByTestId(WTID.overviewTitle)).toContainText("Workspace overview");
  const meta = page.getByTestId(WTID.overviewMeta);
  await expect(meta).toBeVisible();
  await expect(meta, "metadata line must show the session folder path").toContainText(folder);
  await expect(meta, "metadata line must show the session id").toContainText(e.sid);
});

test("w10.2 the build-actions row offers Build Tests / Build Code / Refresh wired to dispatch (R2)", async ({
  page,
}) => {
  env = await WorkspaceEnv.start("w10-overview-actions");
  const e = env;

  await gotoWorkspace(page, e.baseURL, wsRoutes.home(e.sid));

  const buildTests = page.getByTestId(WTID.overviewActionBuildTests);
  const buildCode = page.getByTestId(WTID.overviewActionBuildCode);
  const refresh = page.getByTestId(WTID.overviewActionRefresh);
  for (const [name, btn] of [
    ["build-tests", buildTests],
    ["build-code", buildCode],
    ["refresh", refresh],
  ] as const) {
    await expect(btn, `${name} action visible`).toBeVisible();
    await expect(btn, `${name} action enabled (no own active run)`).toBeEnabled();
  }

  // Boot's OWN session_detail read must fully LAND before we test Refresh, so the
  // GET we then await is unambiguously Refresh-caused. Without this, a late-arriving
  // boot read could satisfy the waiter even if the Refresh handler were removed
  // (reviewer Codex r1 #1: waitForRequest matches any future GET). The inventory
  // chips flip from the "…" skeleton (data-status="unknown") to a real live status
  // only once the boot read resolves — wait for that transition.
  const archChip = overviewInventoryRow(page, "architecture").getByTestId(WTID.overviewInventoryChip);
  await expect(archChip, "boot's session_detail read must resolve before testing Refresh").toHaveAttribute(
    "data-status",
    "present",
    { timeout: 30_000 },
  );

  // Refresh wiring (Codex r1 #1): Refresh is a READ, not a mutation — clicking it
  // triggers a live session_detail read (GET /api/sessions/<sid>), never a
  // dispatch. Proves the control is wired to the read surface, not inert.
  const detailPath = `/api/sessions/${e.sid}`;
  const refreshRead = page.waitForRequest(
    (r) => r.method() === "GET" && new URL(r.url()).pathname === detailPath,
  );
  await refresh.click();
  await refreshRead;

  // Wiring: a build action POSTs the matching `command` to the existing
  // run-dispatch route. Intercept it so NO real run (and no Claude) is spawned
  // on the CLI — the assertion needs only the request the button issues.
  const runsUrl = new RegExp(`/api/sessions/${e.sid}/runs$`);
  await page.route(`**/api/sessions/${e.sid}/runs`, (route) =>
    route.fulfill({
      status: 201,
      contentType: "application/json",
      body: JSON.stringify({ run_id: "e2e-intercepted" }),
    }),
  );

  const testsReq = page.waitForRequest((r) => r.method() === "POST" && runsUrl.test(r.url()));
  await buildTests.click();
  expect((await testsReq).postDataJSON().command, "Build Tests must dispatch command build-tests").toBe("build-tests");

  // Reset the canvas (the fulfilled 201 may have navigated to the run view).
  await gotoWorkspace(page, e.baseURL, wsRoutes.home(e.sid));
  const codeReq = page.waitForRequest((r) => r.method() === "POST" && runsUrl.test(r.url()));
  await page.getByTestId(WTID.overviewActionBuildCode).click();
  expect((await codeReq).postDataJSON().command, "Build Code must dispatch command build-code").toBe("build-code");
});

test("w10.3 an own active run disables the build actions; Refresh stays available (R2)", async ({ page }) => {
  env = await WorkspaceEnv.start("w10-overview-own-active");
  const e = env;

  // The own-active-run rule (README → disabled ONLY for the session's OWN active
  // run; p5 proves it on the protocol detail page) is driven by
  // SessionDetail.active_run. Inject a synthetic detail carrying an OWN active run
  // (active_run.owner_id === detail.owner_id) via a route override — deterministic,
  // no real run/Claude (Codex r1 #2).
  const ownActiveRun = {
    run_id: "own-active-run",
    owner_id: "own-owner",
    command: "build-tests",
    story_id: null,
    fix: false,
    state: "running",
    outcome: null,
    error_detail: null,
    answerable: true,
    created_at: Date.now(),
    updated_at: Date.now(),
  };
  await page.route(detailPredFor(e.sid), (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: syntheticDetail(e.sid, { active_run: ownActiveRun, runs: [ownActiveRun] }),
    }),
  );

  await gotoWorkspace(page, e.baseURL, wsRoutes.home(e.sid));

  await expect(page.getByTestId(WTID.overviewActionBuildTests), "own active run disables Build Tests").toBeDisabled();
  await expect(page.getByTestId(WTID.overviewActionBuildCode), "own active run disables Build Code").toBeDisabled();
  // Refresh is a READ, never gated by the own-active-run rule.
  await expect(page.getByTestId(WTID.overviewActionRefresh), "Refresh stays available").toBeEnabled();
});

test("w10.4 the inventory-health list renders a chip per SessionDetail.inventory entry (R3)", async ({ page }) => {
  env = await WorkspaceEnv.start("w10-overview-inventory");
  const e = env;

  // The overview reads the live inventory itself on mount (SessionDetail.inventory,
  // a fresh CLI scan up to a 30s deadline). Assert against the RENDERED list with a
  // generous timeout — never gate on the detail helper (that relay route is part of
  // what this task adds; see syntheticDetail).
  await gotoWorkspace(page, e.baseURL, wsRoutes.home(e.sid));

  const READ = { timeout: 30_000 } as const;
  const list = page.getByTestId(WTID.overviewInventory);
  await expect(list).toBeVisible(READ);

  const statusRe = new RegExp(`^(${INVENTORY_STATUSES.join("|")})$`);
  // File, DIRECTORY, and CHECKLIST entries must ALL render — not only the four doc
  // keys — each with a status chip in the vocabulary (Codex r1 #3).
  for (const key of ["config", "prd", "architecture", "registry", "stories-dir", "checklist-us-apply"]) {
    const row = overviewInventoryRow(page, key);
    await expect(row, `inventory-health row for "${key}"`).toBeVisible(READ);
    const chip = row.getByTestId(WTID.overviewInventoryChip);
    await expect(chip, `status chip for "${key}"`).toBeVisible();
    await expect(chip).toHaveAttribute("data-status", statusRe);
  }

  // Fixture-guaranteed statuses + paths (anti-placeholder: real values on real
  // rows, not a bare chip). architecture.yaml and the stories dir both exist →
  // present, and each row shows its own path.
  const archRow = overviewInventoryRow(page, "architecture");
  await expect(archRow).toContainText("docs/architecture/architecture.yaml");
  await expect(archRow.getByTestId(WTID.overviewInventoryChip)).toHaveAttribute("data-status", "present");
  const storiesRow = overviewInventoryRow(page, "stories-dir");
  await expect(storiesRow).toContainText("docs/prd/stories");
  await expect(storiesRow.getByTestId(WTID.overviewInventoryChip)).toHaveAttribute("data-status", "present");
});

test("w10.5 degraded-state banners are absent on a clean read (R4 absent state)", async ({ page }) => {
  env = await WorkspaceEnv.start("w10-overview-banners");
  const e = env;

  await gotoWorkspace(page, e.baseURL, wsRoutes.home(e.sid));

  // The overview canvas must render (RED on the stub, which has neither surface)…
  await expect(page.getByTestId(WTID.overviewTitle)).toBeVisible();
  await expect(page.getByTestId(WTID.overviewInventory)).toBeVisible({ timeout: 30_000 });

  // …and this happy fixture reports no degraded state → NOTHING renders.
  await expect(overviewBanner(page), "no degraded-state banner on a clean read").toHaveCount(0);
});

test("w10.6 degraded states from SessionDetail render as bordered banners (R4 present state)", async ({ page }) => {
  env = await WorkspaceEnv.start("w10-overview-degraded");
  const e = env;

  // Present-state oracle (closes Codex r1 #4's real gap deterministically, without
  // a new fixture and without a judge check that would false-fail the clean prod):
  // inject a synthetic degraded SessionDetail via a route override and assert the
  // matching bordered banner renders.
  const detailPred = detailPredFor(e.sid);
  const BANNER = { timeout: 30_000 } as const;

  // (a) A truncated inventory read → the inventory-truncated banner.
  await page.route(detailPred, (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: syntheticDetail(e.sid, {
        inventory: { snapshot_truncated: true, stories_omitted: 3, limit_too_small: false },
      }),
    }),
  );
  await gotoWorkspace(page, e.baseURL, wsRoutes.home(e.sid));
  await expect(
    overviewBanner(page, "inventory_truncated"),
    "snapshot_truncated → the inventory-truncated banner",
  ).toBeVisible(BANNER);
  await page.unroute(detailPred);

  // (b) A same-project SIBLING owner active → the folder-conflict banner (a
  //     warning, not a block — build controls stay enabled).
  await page.route(detailPred, (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: syntheticDetail(e.sid, {
        active_owners: [{ owner_id: "sibling-owner", session_id: null, run_id: "sibling-run", command: "build-code" }],
      }),
    }),
  );
  await gotoWorkspace(page, e.baseURL, wsRoutes.home(e.sid));
  await expect(
    overviewBanner(page, "folder_conflict"),
    "a same-project sibling owner → the folder-conflict banner",
  ).toBeVisible(BANNER);
  await page.unroute(detailPred);

  // (c) A configured/canonical architecture-path divergence → the path-mismatch
  //     banner. This is the mockup's THIRD degraded state (workspace-overview.html
  //     `mockup-path-mismatch`); pinning it here keeps it from silently vanishing on
  //     a fresh regenerate-from-e2e (otherwise asserted only by a gitignored unit test).
  await page.route(detailPred, (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: syntheticDetail(e.sid, {
        inventory: {
          snapshot_truncated: false,
          stories_omitted: 0,
          architecture_path_mismatch: true,
          configured_architecture_path: "services/mcp/architecture.yaml",
          canonical_architecture_path: "docs/architecture/architecture.yaml",
        },
      }),
    }),
  );
  await gotoWorkspace(page, e.baseURL, wsRoutes.home(e.sid));
  await expect(
    overviewBanner(page, "path_mismatch"),
    "a configured/canonical architecture-path divergence → the path-mismatch banner",
  ).toBeVisible(BANNER);
});

test("w10.7 the overview breadcrumb is a two-level Sessions / Workspace overview trail (R5)", async ({ page }) => {
  env = await WorkspaceEnv.start("w10-overview-breadcrumb");
  const e = env;

  await gotoWorkspace(page, e.baseURL, wsRoutes.home(e.sid));

  const crumb = page.getByTestId(WTID.contentBreadcrumb);
  await expect(crumb).toBeVisible();

  // Level 1: a "Sessions" LINK back to the sessions list ("/").
  const sessionsLink = crumb.getByRole("link", { name: "Sessions" });
  await expect(sessionsLink, "breadcrumb must lead with a Sessions link").toBeVisible();
  await expect(sessionsLink).toHaveAttribute("href", "/");

  // Level 2: the current "Workspace overview" crumb (aria-current, not a link).
  const current = crumb.locator('[aria-current="page"]');
  await expect(current).toHaveText("Workspace overview");
});

test("w10.8 a successful build dispatch re-reads session_detail so the own-active-run rule updates without a manual Refresh (R2)", async ({
  page,
}) => {
  env = await WorkspaceEnv.start("w10-overview-dispatch-reread");
  const e = env;

  // The dispatch re-read (a 200/201 dispatch starts an OWN run → the page re-reads
  // session_detail so the build actions disable IMMEDIATELY, no manual Refresh) is
  // an end-to-end behavior otherwise pinned by NO spec and living only in the
  // regenerated page — assert it here so it survives a regenerate-from-e2e. The
  // injected GET detail reports an own active_run ONLY after a dispatch has fired.
  const ownActiveRun = {
    run_id: "own-active-run",
    owner_id: "own-owner",
    command: "build-tests",
    story_id: null,
    fix: false,
    state: "running",
    outcome: null,
    error_detail: null,
    answerable: true,
    created_at: Date.now(),
    updated_at: Date.now(),
  };
  let dispatched = false;
  await page.route(detailPredFor(e.sid), (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: dispatched
        ? syntheticDetail(e.sid, { active_run: ownActiveRun, runs: [ownActiveRun] })
        : syntheticDetail(e.sid, {}),
    }),
  );
  await page.route(`**/api/sessions/${e.sid}/runs`, (route) => {
    dispatched = true;

    return route.fulfill({
      status: 201,
      contentType: "application/json",
      body: JSON.stringify({ run_id: "e2e-reread" }),
    });
  });

  // Count every session_detail GET (registered BEFORE navigation so boot's own read
  // is captured). This lets us strictly pin the RE-READ itself, not just its effect:
  // an implementation that optimistically disabled the buttons after the 201 WITHOUT
  // issuing a fresh GET would satisfy the disabled assertion alone (reviewer Codex r2 #4).
  const detailPath = `/api/sessions/${e.sid}`;
  let detailGets = 0;
  page.on("request", (r) => {
    if (r.method() === "GET" && new URL(r.url()).pathname === detailPath) detailGets += 1;
  });

  await gotoWorkspace(page, e.baseURL, wsRoutes.home(e.sid));

  const buildTests = page.getByTestId(WTID.overviewActionBuildTests);
  await expect(buildTests, "no active run yet → Build Tests enabled").toBeEnabled();
  // Boot's own read must be counted first, so a later GET is unambiguously the
  // dispatch re-read (the page issues exactly one detail read on mount).
  await expect.poll(() => detailGets, { timeout: 30_000 }).toBeGreaterThanOrEqual(1);
  const beforeClick = detailGets;

  await buildTests.click();

  // After the 201 dispatch, the page must (a) issue a NEW session_detail GET — the
  // re-read — and (b) apply the now-own-active-run state by disabling both build
  // actions, WITHOUT a manual Refresh. Dropping the re-read on regeneration fails (a).
  await expect
    .poll(() => detailGets, {
      message: "a successful dispatch must trigger a fresh session_detail re-read",
      timeout: 30_000,
    })
    .toBeGreaterThan(beforeClick);
  await expect(buildTests, "…and the re-read's own active run disables Build Tests").toBeDisabled();
  await expect(page.getByTestId(WTID.overviewActionBuildCode), "…and Build Code disables too").toBeDisabled();
});

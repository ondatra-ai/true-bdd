/**
 * w18 — the harness root (`/`) is a LIVE sessions list (task `home-sessions-list`,
 * P1–P10). DETERMINISTIC workspace-project specs (3-min timeout, no Claude): they
 * drive real CLI remotes through `ProtocolEnv`/`WorkspaceEnv`, read the existing
 * `GET /api/sessions` registry, and use browser-level route injection (the w10
 * idiom) for the failure/timing cases. No new engine endpoint is exercised — the
 * live list is derived entirely from `GET /api/sessions`.
 *
 * RED intent: `harness/src/app/page.tsx` is a behavior-free placeholder (a bare
 * heading + "Sessions list is not implemented yet") — every case below fails as
 * an ASSERTION against an ABSENT surface on the live root (`session-row`,
 * `sessions-list`, `sessions-empty`, the gradient frame, the Open control), never
 * a crash, an unresolved route, or a missing session. The `/` route, the
 * `GET /api/sessions` route, and `/sessions/<sid>/home` all already return 200,
 * and each env's `waitForSession` proves the session is live before any assertion.
 *
 * P10 timing (w18.9) uses the plan's documented REAL-TIME fallback rather than
 * `page.clock`: the sessions home is a task-blind, regenerate-from-e2e artifact
 * whose poll+debounce+React re-render cannot be guaranteed to advance under
 * Playwright's fake clock (Playwright fakes setTimeout/Interval/rAF but not the
 * MessageChannel React 18's scheduler uses), so a clock-driven contract risks
 * turning a compliant impl into a hang. The real-time contract keeps a
 * comfortable margin (notice ABSENT through ~22s of sustained failure, PRESENT by
 * ~45s, real timers driving recovery) so a compliant ~30s impl is never
 * false-failed while a first-503 impl still fails.
 */

import { expect, test, type Page } from "@playwright/test";

import { DESKTOP_VIEWPORT } from "./helpers/design-conformance";
import { ProtocolEnv } from "./helpers/protocol-env";
import {
  TID,
  WTID,
  cssVarRgb,
  gotoSessions,
  sessionOpen,
  sessionRow,
  sessionsList,
  wsRoutes,
} from "./helpers/ui";

const SESSIONS_PATH = "/api/sessions";
/** Only the LIST read (`/api/sessions`), never a session-scoped sub-path. */
const isSessionsList = (url: string): boolean => new URL(url).pathname === SESSIONS_PATH;

let env: ProtocolEnv | undefined;

test.afterEach(async () => {
  // Extend (never overwrite) the shared budget so scoped teardown has room even
  // after a test-body timeout (Playwright's documented idiom).
  const info = test.info();
  info.setTimeout(info.timeout + 60_000);
  const current = env;
  env = undefined;
  if (current !== undefined) {
    await current.teardown(test.info());
  }
});

/** Starts a fresh env + N remotes (each in its own materialized bare folder),
 * returning the live SessionSummary for each remote (by pid). */
async function startWithRemotes(slug: string, count: number) {
  env = await ProtocolEnv.start(slug);
  const e = env;
  const sessions = [];
  for (let i = 0; i < count; i += 1) {
    const fixture = await e.materialize("bare-host");
    const remote = await e.startRemote(fixture.target);
    const session = await e.api.waitForSession((candidate) => candidate.pid === remote.pid);
    sessions.push({ remote, session });
  }
  e.note({ sessionId: sessions.map((s) => s.session.session_id).join(",") });

  return { e, sessions };
}

/** Counts main-document navigations from the moment of the call (a
 * `location.reload()` refresh trips this; a live fetch poll does not). */
function countDocumentNavigations(page: Page): () => number {
  let navigations = 0;
  page.on("request", (request) => {
    if (request.isNavigationRequest() && request.resourceType() === "document") {
      navigations += 1;
    }
  });

  return () => navigations;
}

/** The fully-resolved computed value of the `--gradient-spiral-soft` token
 * (inner `var()` colour stops resolved exactly as the browser paints them). */
function resolvedSpiralGradient(page: Page): Promise<string> {
  return page.evaluate(() => {
    const probe = document.createElement("div");
    probe.style.backgroundImage = "var(--gradient-spiral-soft)";
    document.body.appendChild(probe);
    const value = getComputedStyle(probe).backgroundImage;
    probe.remove();

    return value;
  });
}

/**
 * How many VISIBLE, TOP-anchored, viewport-wide elements paint exactly the
 * resolved spiral gradient — i.e. a real top BAR, not a stray decorative swatch,
 * a row, or a full-page background. The geometry gate (flush to the page top +
 * spanning the viewport) pins "top bar" WITHOUT inventing a new testid.
 */
function gradientTopBarCount(page: Page, expected: string): Promise<number> {
  return page.evaluate((exp) => {
    const viewportWidth = window.innerWidth;

    return Array.from(document.querySelectorAll("*")).filter((el) => {
      const rect = el.getBoundingClientRect();
      if (rect.width === 0 || rect.height === 0) {
        return false;
      }
      // A top BAR begins AT the page top (both directions — not scrolled far
      // above), spans the viewport width, and is a shallow BAND (not a full-page
      // background or a tall hero). Bounds derived generously from the prototype
      // `.top-bar` so a faithful implementation is never false-failed.
      const topAnchored = rect.top >= -24 && rect.top <= 24;
      const spansViewport = rect.width >= viewportWidth * 0.85;
      const bandHeight = rect.height <= 240;
      if (!topAnchored || !spansViewport || !bandHeight) {
        return false;
      }

      return getComputedStyle(el).backgroundImage === exp;
    }).length;
  }, expected);
}

test("w18.1 P1: one row per connected session, scoped to the sessions-list container", async ({ page }) => {
  const { e, sessions } = await startWithRemotes("w18-1-one-row-per-session", 2);
  const [a, b] = sessions;

  await gotoSessions(page, e.server.baseURL);

  const list = sessionsList(page);
  await expect(list, "the sessions-list container must render").toBeVisible({ timeout: 30_000 });
  await expect(
    list.getByTestId(TID.sessionRow),
    "exactly one row per connected session, scoped to the documented container",
  ).toHaveCount(2, { timeout: 30_000 });
  await expect(sessionRow(page, a.session.session_id)).toBeVisible();
  await expect(sessionRow(page, b.session.session_id)).toBeVisible();
});

test("w18.2 P2: row identity — folder realpath / session id / version", async ({ page }) => {
  const { e, sessions } = await startWithRemotes("w18-2-row-identity", 1);
  const summary = sessions[0].session;
  expect(summary.version, "a connected session reports a non-empty version").not.toBe("");

  await gotoSessions(page, e.server.baseURL);

  const row = sessionRow(page, summary.session_id);
  await expect(row).toBeVisible({ timeout: 30_000 });
  await expect(row.getByTestId(TID.sessionFolder), "folder cell EQUALS the realpath").toHaveText(summary.folder);
  await expect(row.getByTestId(TID.sessionMeta), "meta line CONTAINS the session id").toContainText(summary.session_id);
  await expect(row.getByTestId(TID.sessionVersion), "version cell EQUALS the remote version").toHaveText(summary.version);
});

test("w18.3 P3: each row's Open control links to that session's workspace", async ({ page }) => {
  const { e, sessions } = await startWithRemotes("w18-3-open-workspace", 2);

  await gotoSessions(page, e.server.baseURL);
  await expect(sessionsList(page)).toBeVisible({ timeout: 30_000 });

  // Every row's Open control is an <a> whose href resolves EXACTLY to that row's
  // workspace — a shared href, a <button>, or a handler unrelated to the row fails.
  for (const { session } of sessions) {
    const row = sessionRow(page, session.session_id);
    await expect(row).toBeVisible();
    const open = sessionOpen(row);
    await expect(open, "each row exposes an Open-workspace control").toBeVisible();

    const tag = (await open.evaluate((el) => el.tagName)).toLowerCase();
    expect(tag, "the Open control must be an anchor (<a>), not an imperative button").toBe("a");

    const resolved = await open.evaluate((el) => (el as HTMLAnchorElement).href);
    expect(resolved, "the Open href resolves to this row's workspace home").toBe(
      new URL(wsRoutes.home(session.session_id), e.server.baseURL).href,
    );
  }

  // Click ONE row's control and land in THAT session's workspace shell.
  const target = sessions[0].session.session_id;
  await sessionOpen(sessionRow(page, target)).click();
  await expect(page.getByTestId(WTID.rail)).toBeVisible({ timeout: 30_000 });
  await expect(page.getByTestId(WTID.sidebar)).toBeVisible();
  await expect(page.getByTestId(WTID.contentBreadcrumb)).toBeVisible();
  expect(page.url(), "the Open control navigated to the target session's workspace").toContain(
    wsRoutes.home(target),
  );
});

test("w18.4 P4: a newly connected CLI appears live, without a reload", async ({ page }) => {
  const { e, sessions } = await startWithRemotes("w18-4-live-add", 1);
  const first = sessions[0].session;

  await gotoSessions(page, e.server.baseURL);
  await expect(sessionRow(page, first.session_id), "the initial session's row").toBeVisible({ timeout: 30_000 });

  // From here on, no full-document navigation may occur — the list must refresh
  // via a live poll, not a `location.reload()`.
  const navigations = countDocumentNavigations(page);

  const fixture = await e.materialize("bare-host");
  const remote = await e.startRemote(fixture.target);
  const second = await e.api.waitForSession((candidate) => candidate.pid === remote.pid);

  await expect(sessionRow(page, second.session_id), "the new session's row appears on the same page").toBeVisible({
    timeout: 60_000,
  });
  expect(navigations(), "the list updated via a live poll, not a full-document reload").toBe(0);
});

test("w18.5 P4/P5: a stopped CLI vanishes live, structurally, with no marker", async ({ page }) => {
  const { e, sessions } = await startWithRemotes("w18-5-live-drop", 2);
  const [a, b] = sessions;

  await gotoSessions(page, e.server.baseURL);
  await expect(sessionRow(page, a.session.session_id)).toBeVisible({ timeout: 30_000 });
  await expect(sessionRow(page, b.session.session_id)).toBeVisible();

  // Record ANY appearance of a forbidden P5 marker THROUGHOUT the disconnect —
  // a final count-0 alone would miss a marker that flashed and was removed. The
  // check is STRUCTURAL (testids), never a body-text scan: the prototype's own
  // copy legitimately contains "reachability"/"reconnection".
  const forbidden = [TID.sessionDisconnected, TID.sessionUnreachable, TID.sessionReconnect];
  await page.evaluate((markers) => {
    const seen = new Set<string>();
    // The reserved-testid check is opt-in: a wrong impl could render a generic
    // `<button>Reconnect</button>` or a "Disconnected" status BANNER carrying
    // neither a reserved testid nor the dead session's id, and slip through. Also
    // scan INTERACTIVE / STATUS elements by their accessible text for the
    // disconnect/unreachable/reconnect VOCABULARY — scoped to controls/status
    // roles (not free prose) so the prototype's legitimate "reachability"/
    // "reconnection" copy in paragraphs never false-fails.
    const AFFORDANCE = 'button, a, [role="button"], [role="link"], [role="status"], [role="alert"]';
    const VOCAB = /\b(reconnect|disconnect|unreachable)/i;
    const scan = (): void => {
      for (const marker of markers) {
        if (document.querySelector(`[data-testid="${marker}"]`) !== null) {
          seen.add(marker);
        }
      }
      for (const el of Array.from(document.querySelectorAll(AFFORDANCE))) {
        if (VOCAB.test(el.textContent ?? "") || VOCAB.test(el.getAttribute("aria-label") ?? "")) {
          seen.add("labelled-affordance");
        }
      }
    };
    scan();
    const observer = new MutationObserver(scan);
    // `characterData: true` too — a forbidden control could flash its VOCAB via a
    // text-node change alone (no node insert / attribute change) and revert; without
    // it the observer would never re-scan and the flash would be missed.
    observer.observe(document.documentElement, {
      childList: true,
      subtree: true,
      attributes: true,
      characterData: true,
    });
    (window as unknown as { __forbiddenSeen: Set<string> }).__forbiddenSeen = seen;
  }, forbidden);

  const navigations = countDocumentNavigations(page);

  // Freeze remote B: its poll lease lapses (~10s) and the relay removes it.
  b.remote.signal("SIGSTOP");

  // Every element carrying B's session id — the row AND any per-B chip/status/
  // reconnect control — reaches count 0 (STRUCTURAL absence, page-wide).
  await expect(
    page.locator(`[data-session-id="${b.session.session_id}"]`),
    "the stopped session disappears from the list structurally",
  ).toHaveCount(0, { timeout: 60_000 });
  await expect(sessionRow(page, a.session.session_id), "the surviving session stays").toBeVisible();
  expect(navigations(), "the drop happened via a live poll, not a reload").toBe(0);

  const seen = await page.evaluate(
    () => [...(window as unknown as { __forbiddenSeen: Set<string> }).__forbiddenSeen],
  );
  expect(seen, "no dead-session / reachability / reconnect marker may appear during a disconnect").toEqual([]);
});

test("w18.6 P6/P7: honest empty state in the design language", async ({ page }) => {
  env = await ProtocolEnv.start("w18-6-empty-state");
  const e = env;

  await gotoSessions(page, e.server.baseURL);

  const empty = page.getByTestId(TID.sessionsEmpty);
  await expect(empty, "an honest empty state renders when no session is connected").toBeVisible({ timeout: 30_000 });
  await expect(empty, "the empty state states no sessions are connected").toContainText(/no sessions connected/i);
  await expect(empty, "the empty state hints how to connect a session").toContainText(/true-bdd remote/i);

  await expect(page.getByTestId(TID.sessionRow), "no rows in the empty state").toHaveCount(0);
  await expect(sessionsList(page), "the empty state REPLACES the list container").toHaveCount(0);

  // The empty frame keeps the prototype design language: gradient top bar +
  // token wordmark still render, and the message resolves a token-driven muted colour.
  const gradient = await resolvedSpiralGradient(page);
  expect(gradient, "--gradient-spiral-soft must resolve to a gradient (tokens.css is loaded)").toMatch(/gradient/i);
  expect(
    await gradientTopBarCount(page, gradient),
    "the empty frame keeps the gradient top bar",
  ).toBeGreaterThanOrEqual(1);
  const wordmark = page.getByText("TrueBDD", { exact: true }).filter({ visible: true });
  await expect(wordmark, "the wordmark still renders on the empty frame").toBeVisible();
  // The empty branch must consume --text-inverse too (a hardcoded/ wrong colour
  // in an empty-state fork would pass a mere visibility check) — live-mutate the
  // token and confirm the wordmark recomputes to it (same probe as w18.10).
  const inverseSentinel = "rgb(9, 11, 13)";
  await page.evaluate((value) => document.documentElement.style.setProperty("--text-inverse", value), inverseSentinel);
  await expect(wordmark, "the empty-frame wordmark consumes --text-inverse, not a literal").toHaveCSS(
    "color",
    inverseSentinel,
  );
  await page.evaluate(() => document.documentElement.style.removeProperty("--text-inverse"));

  await expect(empty, "the empty-state message uses the token-driven muted colour").toHaveCSS(
    "color",
    await cssVarRgb(page, "--text-muted"),
  );
});

test("w18.7 P8: stable oldest-first order across live refreshes", async ({ page }) => {
  const { e, sessions } = await startWithRemotes("w18-7-stable-order", 2);
  const [a, b] = sessions.map((s) => s.session);

  // Unambiguous order guard: real inter-remote startup latency makes a
  // millisecond `connected_at` tie effectively impossible; if it ever ties, the
  // sort below is ambiguous, so fail as a SETUP problem rather than pass wrongly.
  expect(b.connected_at, "the two sessions must have distinct connected_at (order guard)").not.toBe(a.connected_at);
  const expectedOrder = [a, b].sort((x, y) => x.connected_at - y.connected_at).map((s) => s.session_id);

  await gotoSessions(page, e.server.baseURL);
  await expect(sessionsList(page)).toBeVisible({ timeout: 30_000 });
  await expect(page.getByTestId(TID.sessionRow)).toHaveCount(2, { timeout: 30_000 });

  const readOrder = (): Promise<(string | null)[]> =>
    page.getByTestId(TID.sessionRow).evaluateAll((els) => els.map((el) => el.getAttribute("data-session-id")));

  expect(await readOrder(), "rows render oldest-first by connected_at").toEqual(expectedOrder);

  // Prove order is stable across TWO distinct live refreshes. The two waiters are
  // armed and awaited SEQUENTIALLY (never concurrently): two identical predicates
  // armed at once could BOTH resolve from a single response, proving only one
  // refresh. Awaiting the first, then arming the second, guarantees the second
  // captures a genuinely later poll. Each match is a completed GET 200.
  const sessionsRead = () =>
    page.waitForResponse((r) => r.request().method() === "GET" && isSessionsList(r.url()) && r.status() === 200);

  // A completed GET-200 response resolves BEFORE React commits the re-render, and
  // a fixed frame count is NOT a commit barrier. So after awaiting each distinct
  // poll, assert the order with an AUTO-RETRYING `expect.poll` that re-reads the
  // LIVE (committed) DOM — a reshuffle to a persistent wrong order fails, while a
  // late-committing correct render is tolerated. Real remotes started A-then-B
  // keep the plan's method; only the assertion is made race-free.
  await sessionsRead();
  await expect
    .poll(readOrder, { message: "order is stable after the first live refresh", timeout: 10_000 })
    .toEqual(expectedOrder);

  await sessionsRead();
  await expect
    .poll(readOrder, { message: "order is stable after a second, distinct live refresh", timeout: 10_000 })
    .toEqual(expectedOrder);
});

test("w18.8 P9: no empty-state flash before the first read resolves", async ({ page }) => {
  env = await ProtocolEnv.start("w18-8-no-empty-flash");
  const e = env;

  // A handshake proves the first read is genuinely IN FLIGHT (not merely a
  // not-yet-mounted client that would trivially show 0 empty states): the
  // handler resolves `intercepted` on entry, then BLOCKS on `release` before
  // fulfilling 200 {sessions: []}.
  let markIntercepted: () => void = () => {};
  const intercepted = new Promise<void>((resolve) => (markIntercepted = resolve));
  let release: () => void = () => {};
  const released = new Promise<void>((resolve) => (release = resolve));

  await page.route("**/api/sessions", async (route) => {
    if (!isSessionsList(route.request().url())) {
      return route.continue();
    }
    markIntercepted();
    await released;

    return route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ sessions: [] }) });
  });

  await gotoSessions(page, e.server.baseURL);

  // Bound the handshake with an ASSERTION (never a hang): the sessions home MUST
  // issue a GET /api/sessions read on mount. A placeholder that issues none fails
  // HERE as an assertion, not a timeout.
  const inFlight = await Promise.race([
    intercepted.then(() => true),
    page.waitForTimeout(30_000).then(() => false),
  ]);
  expect(inFlight, "the sessions home must issue a GET /api/sessions read on mount").toBe(true);

  // While the first read is provably pending, the empty state must NOT flash.
  await expect(page.getByTestId(TID.sessionsEmpty), "no empty-state flash while the first read is pending").toHaveCount(
    0,
  );

  release();
  await expect(page.getByTestId(TID.sessionsEmpty), "the empty state appears once the zero read resolves").toBeVisible({
    timeout: 30_000,
  });
});

test("w18.9 P10: unavailable only after sustained failure, then auto-return", async ({ page }) => {
  const { e, sessions } = await startWithRemotes("w18-9-unavailable-threshold", 1);
  const sid = sessions[0].session.session_id;

  // A single route owns a `mode`: pass → the real read; fail → a deterministic
  // 503 JSON error (the canonical P10 stimulus) that resolves `firstFailure` on
  // the FIRST injected failure so the threshold is measured from a known point.
  let mode: "pass" | "fail" = "pass";
  let markFirstFailure: () => void = () => {};
  const firstFailure = new Promise<void>((resolve) => (markFirstFailure = resolve));
  let firedFirstFailure = false;

  await page.route("**/api/sessions", async (route) => {
    if (!isSessionsList(route.request().url()) || mode === "pass") {
      return route.continue();
    }
    if (!firedFirstFailure) {
      firedFirstFailure = true;
      markFirstFailure();
    }

    return route.fulfill({
      status: 503,
      contentType: "application/json",
      body: JSON.stringify({ error: "sessions_unavailable" }),
    });
  });

  await gotoSessions(page, e.server.baseURL);
  // (1) A real read succeeded → the live row (proves the page reads before failing).
  await expect(sessionRow(page, sid), "a real read populates the live row before failures begin").toBeVisible({
    timeout: 30_000,
  });

  // (2) Begin sustained failure; measure the threshold from the first failing poll.
  mode = "fail";
  const started = await Promise.race([firstFailure.then(() => true), page.waitForTimeout(30_000).then(() => false)]);
  expect(started, "the sessions home must keep polling GET /api/sessions (no failing poll observed)").toBe(true);
  const failStart = Date.now();

  const notice = page.getByTestId(TID.sessionsUnavailable);

  // (3a) ABSENT through ~22s of sustained failure — a first-503 notice fails here.
  while (Date.now() - failStart < 22_000) {
    expect(await notice.count(), "the unavailable notice must not appear before the sustained-failure threshold").toBe(
      0,
    );
    await page.waitForTimeout(1_000);
  }

  // (3b) PRESENT after the ~30s threshold (present by ~45s), stale row gone.
  await expect(notice, "the unavailable notice appears after sustained failure").toBeVisible({ timeout: 28_000 });
  // The notice must EXPLAIN unavailability (P10 "explicit unavailable notice"), not
  // be a blank box — the copy is production behavior otherwise pinned by no test and
  // would silently vanish on a fresh regenerate-from-tests.
  await expect(notice, "the unavailable notice explains that sessions are unavailable").toContainText(
    /unavailable|unable to load/i,
  );
  await expect(sessionRow(page, sid), "the stale row is not carried past the staleness verdict").toHaveCount(0);

  // (4) Recovery — waiter armed BEFORE the trigger (w10 idiom); real timers drive
  // the next poll. Both the row RETURNS and the notice CLEARS, with no manual reload.
  const recovered = page.waitForResponse(
    (r) => r.request().method() === "GET" && isSessionsList(r.url()) && r.status() === 200,
  );
  mode = "pass";
  await recovered;
  await expect(sessionRow(page, sid), "the row returns once reads recover").toBeVisible({ timeout: 30_000 });
  await expect(notice, "the unavailable notice clears on recovery — no manual reload").toHaveCount(0);
});

test("w18.10 P7: deterministic sessions-design parity (primary gate)", async ({ page }) => {
  const { e, sessions } = await startWithRemotes("w18-10-design-parity", 1);
  const summary = sessions[0].session;

  await page.setViewportSize({ ...DESKTOP_VIEWPORT });
  await gotoSessions(page, e.server.baseURL);
  const row = sessionRow(page, summary.session_id);
  await expect(row, "a live row renders the sessions frame").toBeVisible({ timeout: 30_000 });

  // Gradient top bar — a top-anchored, viewport-wide band paints the
  // --gradient-spiral-soft field.
  const gradient = await resolvedSpiralGradient(page);
  expect(gradient, "--gradient-spiral-soft must resolve to a gradient (tokens.css is loaded)").toMatch(/gradient/i);
  expect(
    await gradientTopBarCount(page, gradient),
    "a gradient top bar paints --gradient-spiral-soft",
  ).toBeGreaterThanOrEqual(1);

  // TrueBDD wordmark CONSUMES --text-inverse (live-mutation token probe — proves a
  // token, not a hardcoded literal). Scoped to the VISIBLE wordmark so a
  // responsive hidden copy cannot trip strict mode.
  const wordmark = page.getByText("TrueBDD", { exact: true }).filter({ visible: true });
  await expect(wordmark, "the TrueBDD wordmark").toBeVisible();
  const sentinel = "rgb(3, 5, 7)";
  await page.evaluate((value) => document.documentElement.style.setProperty("--text-inverse", value), sentinel);
  await expect(wordmark, "the wordmark must consume --text-inverse, not a literal colour").toHaveCSS("color", sentinel);
  await page.evaluate(() => document.documentElement.style.removeProperty("--text-inverse"));

  // Tagline present (visible copy — tolerant of a responsive hidden duplicate).
  await expect(page.getByText(/spec-anchored cli/i).filter({ visible: true }), "the top-bar tagline").toBeVisible();

  // Exactly ONE visible h1 whose text is exactly "Sessions".
  const h1 = page.locator("h1");
  await expect(h1, "exactly one h1 on the sessions home").toHaveCount(1);
  await expect(h1, 'the page heading is exactly "Sessions"').toHaveText("Sessions");
  await expect(
    page.getByRole("heading", { level: 1, name: "Sessions" }),
    "the h1 accessible name is Sessions",
  ).toBeVisible();

  // Row anatomy: title / meta / version chip.
  await expect(row.getByTestId(TID.sessionFolder)).toBeVisible();
  await expect(row.getByTestId(TID.sessionMeta)).toBeVisible();
  await expect(row.getByTestId(TID.sessionVersion)).toBeVisible();

  // Explicit PRE-WORKSPACE frame (P7 "no icon rail, no sidebar"): the workspace
  // shell must NOT leak onto `/`.
  for (const tid of [WTID.rail, WTID.sidebar, WTID.appShell, WTID.workspaceMain]) {
    await expect(page.getByTestId(tid), `${tid} must not appear on the sessions home`).toHaveCount(0);
  }

  // Zero Test-connection controls (P7 drop). The literal survives ONLY as this
  // negative assertion — the control is retired from the contract.
  await expect(page.getByTestId("test-connection"), "Test-connection is retired from the sessions home").toHaveCount(0);
  // The testid-only check is opt-in: a wrong regeneration could reintroduce the
  // control without the retired testid (the prototype carried a "Test connection"
  // button until 2026-08-06, when it was removed to match this drop). Pin the
  // drop by ACCESSIBLE NAME so a labelled-but-untagged control fails too.
  for (const role of ["button", "link"] as const) {
    await expect(
      page.getByRole(role, { name: /test[- ]?connection/i }),
      `no ${role} named "Test connection" may render (P7 drop, by accessible name)`,
    ).toHaveCount(0);
  }
});

test("w18.11 P10: a HUNG read (not just an error response) still surfaces the notice and recovers", async ({
  page,
}) => {
  // w18.9 injects immediately-completed 503s, which need no client-side timeout to
  // be classified as failures. A read that HANGS (never responds) does — without a
  // per-request timeout the poll loop stalls forever, the unavailable notice never
  // appears, and recovery never happens. That resilience is production behavior no
  // other e2e case pins, so a fresh regenerate-from-tests could silently drop it.
  // The generous 80s bound covers the client timeout + poll-interval accrual to the
  // ~30s threshold for continuously-hung reads, so a compliant impl is never
  // false-failed while a no-timeout impl hangs and fails here.
  test.setTimeout(240_000);
  const { e, sessions } = await startWithRemotes("w18-11-hung-read", 1);
  const sid = sessions[0].session.session_id;

  let mode: "pass" | "hang" = "pass";
  await page.route("**/api/sessions", async (route) => {
    if (!isSessionsList(route.request().url()) || mode === "pass") {
      return route.continue();
    }

    // Never fulfill: the request hangs until the client's OWN per-request timeout
    // aborts it (classified as a poll failure). No timeout ⇒ this never returns and
    // the notice below never appears ⇒ the test fails, pinning the resilience.
    return new Promise<void>(() => {});
  });

  await gotoSessions(page, e.server.baseURL);
  await expect(sessionRow(page, sid), "a real read populates the live row before reads hang").toBeVisible({
    timeout: 30_000,
  });

  mode = "hang";
  const notice = page.getByTestId(TID.sessionsUnavailable);
  await expect(notice, "a hung read must still surface the unavailable notice").toBeVisible({ timeout: 80_000 });
  await expect(sessionRow(page, sid), "the stale row is not carried past the staleness verdict").toHaveCount(0);

  // Recovery — waiter armed BEFORE the flip (w10 idiom); real timers drive the
  // next poll once the current hung request is aborted by the client timeout.
  const recovered = page.waitForResponse(
    (r) => r.request().method() === "GET" && isSessionsList(r.url()) && r.status() === 200,
  );
  mode = "pass";
  await recovered;
  await expect(sessionRow(page, sid), "the row returns once reads recover").toBeVisible({ timeout: 30_000 });
  await expect(notice, "the unavailable notice clears on recovery — no manual reload").toHaveCount(0);
});

test("w18.12 P6/P9: a malformed 200 is a failure, never the honest empty state", async ({ page }) => {
  // The honest empty state is reserved for a GENUINE successful zero read. A
  // malformed 200 (`{}`, a non-array `sessions`, a malformed entry) must be treated
  // as a poll FAILURE — a naive `body.sessions ?? []` regeneration would instead
  // render "No sessions connected." on garbage. That guard is production behavior
  // otherwise pinned only by gitignored unit tests, so pin the user-visible
  // consequence here.
  env = await ProtocolEnv.start("w18-12-malformed-not-empty");
  const e = env;

  let mode: "malformed" | "empty" = "malformed";
  await page.route("**/api/sessions", async (route) => {
    if (!isSessionsList(route.request().url())) {
      return route.continue();
    }
    const body = mode === "malformed" ? JSON.stringify({ garbage: true }) : JSON.stringify({ sessions: [] });

    return route.fulfill({ status: 200, contentType: "application/json", body });
  });

  await gotoSessions(page, e.server.baseURL);

  const empty = page.getByTestId(TID.sessionsEmpty);
  // Across several malformed polls the empty state must NEVER appear (a naive impl
  // shows it within the first poll).
  const start = Date.now();
  while (Date.now() - start < 8_000) {
    expect(await empty.count(), "a malformed 200 must never render the honest empty state").toBe(0);
    await page.waitForTimeout(1_000);
  }

  // A genuine successful zero read DOES show the honest empty state.
  const okRead = page.waitForResponse(
    (r) => r.request().method() === "GET" && isSessionsList(r.url()) && r.status() === 200,
  );
  mode = "empty";
  await okRead;
  await expect(empty, "a genuine zero read shows the honest empty state").toBeVisible({ timeout: 30_000 });
});

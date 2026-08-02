/**
 * m1 — page coverage, shared sidebar frame, and S&F token consumption.
 *
 * Static local HTML mockups under `harness/design/mockups/`, opened over file://
 * (no server/build). Every case is phrased to FAIL if the behavior/content were
 * absent (missing page → goto navigation error; missing region/link/token wiring
 * → assertion failure). Signature strings are drawn verbatim from the named
 * fixtures so a placeholder/empty page cannot pass.
 *
 * Provenance of the quoted content:
 *   - Architecture (service/vocabulary): fixtures/a1-create-no-fix/.../architecture.yaml
 *   - Product (prd/epic/story 60.2):     fixtures/p3-inventory-spread/... + a1 prd.yaml
 *   - Requirements (E2E-60x, INT-901):   fixtures/p3-inventory-spread/scenarios.yaml + a5
 */

import { expect, test, type Locator, type Page } from "@playwright/test";

import {
  FONT_CHECK_PAGES,
  MOCKUP_TID,
  PAGES,
  TOKEN_PROBE_ATTR,
  WORKSPACE_PAGES,
  mirrorTokenNames,
  openMockup,
  sidebarSectionId,
  tokensEntrypointHref,
} from "./helpers/mockups";

// Custom-property names actually declared in the mirror's CSS — the m1.6c probe
// must consume one of THESE, not an arbitrary inline token (round 2, finding 4).
const MIRROR_TOKENS = mirrorTokenNames();

/**
 * Assert `ids` map ONE-TO-ONE onto distinct `rows` — each id occurs in exactly
 * one row, and no single "catch-all" row lists more than one id (round 2,
 * finding 1). Defeats a placeholder that lists every id in one blob.
 */
async function assertOneRowPerId(rows: Locator, ids: string[]): Promise<void> {
  for (const id of ids) {
    await expect(rows.filter({ hasText: id }), `exactly one row for ${id}`).toHaveCount(1);
  }
  const maxIdsPerRow = await rows.evaluateAll(
    (els, wanted) =>
      Math.max(0, ...els.map((e) => wanted.filter((id) => (e.textContent ?? "").includes(id)).length)),
    ids,
  );
  expect(maxIdsPerRow, "no catch-all row lists multiple ids").toBeLessThanOrEqual(1);
}

/** Signature-content assertions per page (m1.1). Fixture-exact strings. */
async function assertSignature(page: Page, name: string): Promise<void> {
  switch (name) {
    case "sessions": {
      const rows = page.getByTestId(MOCKUP_TID.sessionRow);
      await expect(rows.first()).toBeVisible();
      // The row surfaces a canonical folder path, a CLI version and a control.
      await expect(rows.first()).toContainText("/");
      await expect(rows.first(), "row carries a CLI version string").toContainText(/\d+\.\d+/);
      await expect(page.getByRole("button", { name: /test connection/i }).first()).toBeVisible();
      break;
    }
    case "workspace-overview": {
      await expect(page.getByRole("heading", { name: /workspace overview/i })).toBeVisible();
      await expect(page.locator(`[data-testid="${MOCKUP_TID.inventoryChip}"]`).first()).toBeVisible();
      await expect(page.getByRole("button", { name: /build tests/i })).toBeVisible();
      await expect(page.getByRole("button", { name: /build code/i })).toBeVisible();
      await expect(page.getByRole("button", { name: /refresh/i }).first()).toBeVisible();
      break;
    }
    case "prd-overview": {
      // Title + summary + personas from a1-create-no-fix/prd.yaml (has BOTH personas).
      await expect(page.getByText(/MCP Google Docs Editor/i).first()).toBeVisible();
      await expect(page.getByText(/deliberately minimal/i).first()).toBeVisible();
      await expect(page.getByText("Claude User").first()).toBeVisible();
      await expect(page.getByText("Developer/Maintainer").first()).toBeVisible();
      break;
    }
    case "epic": {
      await expect(page.getByText("Inventory Spread Epic").first()).toBeVisible();
      // A grouped story table with one DISTINCT row per story (60.1/60.2/60.3).
      const rows = page.getByTestId(MOCKUP_TID.storyRow);
      await expect(rows.first()).toBeVisible();
      await assertOneRowPerId(rows, ["60.1", "60.2", "60.3"]);
      break;
    }
    case "story-detail": {
      await expect(page.getByText("60.2").first()).toBeVisible();
      // as-a / i-want / so-that — story 60.2 verbatim.
      await expect(page.getByText("Claude User").first()).toBeVisible();
      await expect(
        page.getByText(/give me a short summary of a shared Google Doc/i).first(),
      ).toBeVisible();
      await expect(
        page.getByText(/decide whether the document needs my attention/i).first(),
      ).toBeVisible();
      break;
    }
    case "scenarios": {
      // Each scenario id has its OWN DISTINCT row, and each row surfaces its service.
      const rows = page.getByTestId(MOCKUP_TID.scenarioRow);
      await expect(rows.first()).toBeVisible();
      const ids = ["E2E-601", "E2E-602", "E2E-603", "INT-901"];
      await assertOneRowPerId(rows, ids);
      for (const id of ids) {
        await expect(
          rows.filter({ hasText: id }).first(),
          `scenario row ${id} surfaces its service`,
        ).toContainText("mcp-service");
      }
      break;
    }
    case "scenario-detail": {
      await expect(page.getByText("E2E-601").first()).toBeVisible();
      await expect(page.getByText("mcp-service").first()).toBeVisible();
      // E2E-601 merged given/when/then (scenarios.yaml, verbatim).
      await expect(page.getByText(/at least 500 words of body text/i).first()).toBeVisible();
      await expect(page.getByText(/asks Claude to summarize the document/i).first()).toBeVisible();
      await expect(page.getByText(/150 words or fewer in the conversation/i).first()).toBeVisible();
      // Link to the originating story (60.2).
      await expect(page.locator('a[href*="story-detail"]').first()).toHaveCount(1);
      break;
    }
    case "service": {
      await expect(page.getByText("mcp-service").first()).toBeVisible();
      await expect(page.getByText("net/http").first()).toBeVisible();
      await expect(page.getByText(/jest/i).first()).toBeVisible();
      // The exact test-config path from architecture.yaml.
      await expect(page.getByText(/tests\/jest\.config\.ts/i).first()).toBeVisible();
      await expect(page.getByText("McpClient").first()).toBeVisible();
      break;
    }
    case "vocabulary": {
      await expect(page.getByText("ask Claude").first()).toBeVisible();
      for (const q of ["properly", "correctly", "seamless"]) {
        await expect(page.getByText(q).first()).toBeVisible();
      }
      await expect(page.getByText("handle").first()).toBeVisible();
      break;
    }
    case "runs": {
      const rows = page.getByTestId(MOCKUP_TID.runRow);
      await expect(rows.first()).toBeVisible();
      // A run row carries a command AND an outcome.
      await expect(rows.first(), "run row shows a command").toContainText(
        /build|create|refine|apply|version/i,
      );
      await expect(rows.first(), "run row shows an outcome").toContainText(
        /ok|converged|not_fixed|user_exit|max_attempts|interrupted|abandoned|error/i,
      );
      break;
    }
    case "run-detail": {
      // A run state, a non-success outcome badge (deep check in m3.8), an output tail.
      await expect(page.getByTestId(MOCKUP_TID.runState)).toBeVisible();
      await expect(page.getByTestId(MOCKUP_TID.runOutcome)).toBeVisible();
      await expect(page.locator("body")).toContainText(/output|log|tail/i);
      break;
    }
    case "prompt-choice":
    case "prompt-clarify":
    case "prompt-freetext": {
      // An OPEN dialog overlay (deep structure in m4).
      await expect(page.getByRole("dialog")).toBeVisible();
      break;
    }
    // Degraded pages: existence + workspace frame here; deep content in m3.
    case "story-ambiguous":
    case "story-invalid":
    case "unavailable": {
      await expect(page.getByTestId(MOCKUP_TID.sidebar)).toBeVisible();
      const bodyText = (await page.locator("body").innerText()).trim();
      expect(bodyText.length).toBeGreaterThan(0);
      break;
    }
    default:
      throw new Error(`no signature assertion defined for page "${name}"`);
  }
}

test.describe("m1 mockup pages", () => {
  // ── m1.1 — every page in the inventory exists and renders signature content ──
  for (const name of PAGES) {
    test(`m1.1 ${name}: page exists and renders its identifying content`, async ({ page }) => {
      await openMockup(page, name);
      await assertSignature(page, name);
    });
  }

  // ── m1.2 — shared sidebar with the required structure and top-level ordering ──
  for (const name of WORKSPACE_PAGES) {
    test(`m1.2 ${name}: sidebar carries the ordered top-level structure`, async ({ page }) => {
      await openMockup(page, name);
      const sidebar = page.getByTestId(MOCKUP_TID.sidebar);
      await expect(sidebar).toHaveCount(1);
      await expect(sidebar).toBeVisible();

      // The two flat links bracket the four section landmarks, in this order.
      const entries = [
        { label: "workspace overview", locator: sidebar.locator('a[href$="workspace-overview.html"]').first() },
        { label: "Architecture", locator: sidebar.getByTestId(sidebarSectionId("architecture")) },
        { label: "Product", locator: sidebar.getByTestId(sidebarSectionId("product")) },
        { label: "Requirements", locator: sidebar.getByTestId(sidebarSectionId("requirements")) },
        { label: "Runs", locator: sidebar.getByTestId(sidebarSectionId("runs")) },
        { label: "return to sessions", locator: sidebar.locator('a[href$="sessions.html"]').first() },
      ];

      // Resolve each entry and assert their vertical order strictly increases
      // (the brief's "top-level ordering").
      const ys: number[] = [];
      for (const { label, locator } of entries) {
        await expect(locator, `sidebar entry "${label}" present`).toBeVisible();
        const box = await locator.boundingBox();
        expect(box, `sidebar entry "${label}" has a bounding box`).not.toBeNull();
        ys.push(box!.y);
      }
      for (let i = 1; i < ys.length; i++) {
        expect(
          ys[i],
          `"${entries[i].label}" must be below "${entries[i - 1].label}"`,
        ).toBeGreaterThan(ys[i - 1]);
      }
    });
  }

  // ── m1.3 — Architecture section: a service node + a vocabulary node ──
  test("m1.3 Architecture section follows architecture.yaml (service + vocabulary)", async ({ page }) => {
    await openMockup(page, "workspace-overview");
    const arch = page.getByTestId(sidebarSectionId("architecture"));
    await expect(arch).toHaveCount(1);
    await expect(arch.locator('a[href$="service.html"]')).toHaveCount(1);
    await expect(arch).toContainText("mcp-service");
    await expect(arch.locator('a[href$="vocabulary.html"]')).toHaveCount(1);
    await expect(arch).toContainText(/vocabulary/i);
  });

  // ── m1.4 — Product section: PRD-overview root, epic → stories tree ──
  test("m1.4 Product section is epics→stories with a PRD-overview root", async ({ page }) => {
    await openMockup(page, "workspace-overview");
    const product = page.getByTestId(sidebarSectionId("product"));
    await expect(product).toHaveCount(1);
    await expect(product.locator('a[href$="prd-overview.html"]')).toHaveCount(1);
    await expect(product.locator('a[href$="epic.html"]')).toHaveCount(1);
    await expect(product).toContainText("Inventory Spread Epic");
    // The epic expands to its three stories (existence regardless of collapse).
    for (const id of ["60.1", "60.2", "60.3"]) {
      await expect(product.getByText(id)).not.toHaveCount(0);
    }
    await expect(product.locator('a[href*="story-detail"]').first()).toHaveCount(1);
  });

  // ── m1.5 — Requirements/Scenarios: FLAT scenario-id list (no epic/story tree) ──
  test("m1.5 Requirements section is a FLAT scenario-id list", async ({ page }) => {
    await openMockup(page, "workspace-overview");
    const req = page.getByTestId(sidebarSectionId("requirements"));
    await expect(req).toHaveCount(1);

    // No epic tree level intervenes.
    await expect(req.getByText("Inventory Spread Epic")).toHaveCount(0);

    // Each scenario id is its own DISTINCT row and each row surfaces its service.
    const ids = ["E2E-601", "E2E-602", "E2E-603", "INT-901"];
    const reqRows = req.getByTestId(MOCKUP_TID.scenarioRow);
    await assertOneRowPerId(reqRows, ids);
    for (const id of ids) {
      await expect(
        reqRows.filter({ hasText: id }).first(),
        `Requirements row ${id} surfaces its service`,
      ).toContainText("mcp-service");
    }

    // Flatness (structural, robust to whether the section header is collapsible):
    // every scenario row is a SIBLING in one flat list — a tree would group them
    // under per-epic sub-lists with different parents.
    const flat = await req
      .getByTestId(MOCKUP_TID.scenarioRow)
      .evaluateAll((els) => els.length > 0 && els.every((e) => e.parentElement === els[0].parentElement));
    expect(flat, "all scenario rows are siblings in one flat list (no epic sub-grouping)").toBe(true);
  });

  // ── m1.6a — every page links the EXACT mirror entrypoint ──
  for (const name of PAGES) {
    test(`m1.6a ${name}: links exactly …/harness/design/system/tokens.css`, async ({ page }) => {
      await openMockup(page, name);
      const hrefs = await page.evaluate(() =>
        Array.from(document.querySelectorAll('link[rel="stylesheet"]')).map(
          (l) => (l as HTMLLinkElement).href,
        ),
      );
      expect(
        hrefs,
        `no <link rel=stylesheet> resolving to the mirror entrypoint; got: ${hrefs.join(", ")}`,
      ).toContain(tokensEntrypointHref);
    });
  }

  // ── m1.6b — applied Poppins type on every page + a real offline font decode ──
  for (const name of PAGES) {
    test(`m1.6b ${name}: body type resolves to Poppins`, async ({ page }) => {
      await openMockup(page, name);
      const family = await page.evaluate(() => getComputedStyle(document.body).fontFamily);
      expect(family).toMatch(/Poppins/i);
    });
  }
  for (const name of FONT_CHECK_PAGES) {
    test(`m1.6b ${name}: the Poppins font binary actually decodes offline`, async ({ page }) => {
      await openMockup(page, name);
      const result = await page.evaluate(async () => {
        await document.fonts.load('16px "Poppins"', "Ag");
        const poppins = Array.from(document.fonts).filter((f) => /Poppins/i.test(f.family));
        const anyLoaded = poppins.some((f) => f.status === "loaded");
        await document.fonts.ready;

        return { count: poppins.length, anyLoaded };
      });
      expect(result.count, "a @font-face Poppins entry is registered").toBeGreaterThan(0);
      expect(result.anyLoaded, "at least one Poppins FontFace reaches status=loaded").toBe(true);
    });
  }

  // ── m1.6c — a token value is actually applied (self-referential, serialize-safe) ──
  for (const name of PAGES) {
    test(`m1.6c ${name}: a documented S&F token is applied to a real element`, async ({ page }) => {
      await openMockup(page, name);
      const probe = await page.evaluate((attr) => {
        const el = document.querySelector(`[${attr}]`);
        if (el === null) {
          return { ok: false as const, reason: `no [${attr}] element on the page` };
        }
        const token = (el.getAttribute(attr) ?? "").trim();
        const root = document.documentElement;
        const declared = getComputedStyle(root).getPropertyValue(token).trim();
        // Normalize the token to a canonical rgb via a throwaway probe element so
        // hex-vs-rgb / shorthand serialization can't cause a false mismatch.
        const span = document.createElement("span");
        span.style.color = `var(${token})`;
        document.body.appendChild(span);
        const normalized = getComputedStyle(span).color;
        span.remove();
        const applied = getComputedStyle(el).color;

        // Live-dependency proof (round 3, finding 2): mutate the token at :root and
        // observe the element TRACK it. A hardcoded literal equal to the token's
        // value would NOT change — defeating that false pass.
        const before = applied;
        const sentinel = "rgb(1, 2, 3)";
        const priorInline = root.style.getPropertyValue(token);
        root.style.setProperty(token, sentinel);
        const after = getComputedStyle(el).color;
        if (priorInline) {
          root.style.setProperty(token, priorInline);
        } else {
          root.style.removeProperty(token);
        }

        return { ok: true as const, token, declared, normalized, applied, before, after, sentinel };
      }, TOKEN_PROBE_ATTR);

      expect(probe.ok, probe.ok ? "" : probe.reason).toBe(true);
      if (!probe.ok) {
        return;
      }
      expect(probe.token, "the probe names a custom property").toMatch(/^--/);
      // The probed token must be a REAL mirror token (not an inline decoy), so the
      // page provably consumes the S&F system, not a hardcoded literal.
      expect(
        MIRROR_TOKENS.has(probe.token),
        `probed token ${probe.token} is declared in the mirror CSS`,
      ).toBe(true);
      expect(probe.declared, `token ${probe.token} is defined and non-empty`).not.toBe("");
      // Baseline: the element's computed color equals the token, normalized.
      expect(probe.applied).toBe(probe.normalized);
      // Live dependency: mutating the token re-colours the element to the sentinel.
      expect(probe.after, "element colour tracks the token (not a hardcoded literal)").toBe(
        probe.sentinel,
      );
      expect(probe.after, "the element's colour actually changed with the token").not.toBe(
        probe.before,
      );
    });
  }

  // ── m1.6d — fully self-contained + offline (no http(s) request at all) ──
  for (const name of PAGES) {
    test(`m1.6d ${name}: every request is file:/data: and there are no errors`, async ({ page }) => {
      const { requests, pageErrors, requestFailures } = await openMockup(page, name);
      const remote = requests.filter((u) => !/^(file:|data:)/i.test(u));
      expect(remote, `remote (non-file:/data:) requests: ${remote.join(", ")}`).toEqual([]);
      expect(pageErrors, `page errors: ${pageErrors.join(" | ")}`).toEqual([]);
      expect(requestFailures, `failed requests: ${requestFailures.join(", ")}`).toEqual([]);
    });
  }

  // ── m1.7 — the story detail page carries the FULL document the brief requires ──
  test("m1.7 story-detail carries status, steps, actions and a fix toggle", async ({ page }) => {
    await openMockup(page, "story-detail");
    await expect(page.getByText("60.2").first()).toBeVisible();
    // A status pill.
    await expect(page.getByTestId(MOCKUP_TID.statusPill).first()).toBeVisible();
    // At least one AC CONTAINER groups its Given / When / Then step text
    // (verbatim, story 60.2) — the steps must live under one AC block, not
    // merely appear anywhere on the page (final review, finding 6).
    const acBlock = page
      .locator(".ac-block")
      .filter({ hasText: /at least 500 words of body text/i })
      .first();
    await expect(acBlock).toBeVisible();
    await expect(acBlock).toContainText(/asks Claude to summarize the document/i);
    await expect(acBlock).toContainText(/150 words or fewer in the conversation/i);
    // The three story-page action controls.
    await expect(page.getByRole("button", { name: /create/i }).first()).toBeVisible();
    await expect(page.getByRole("button", { name: /refine/i }).first()).toBeVisible();
    await expect(page.getByRole("button", { name: /apply/i }).first()).toBeVisible();
    // A fix toggle whose ACCESSIBLE NAME says "fix" — an unrelated checkbox
    // must not satisfy this (final review, finding 6).
    const fix = page
      .getByRole("switch", { name: /fix/i })
      .or(page.getByRole("checkbox", { name: /fix/i }));
    await expect(fix.first()).toBeVisible();
  });

  // ── m1.9 — the epic page is a real grouped TABLE, not free-floating rows ──
  test("m1.9 epic.html renders the grouped story table with lifecycle columns", async ({ page }) => {
    await openMockup(page, "epic");
    const table = page.locator("table").filter({ hasText: "60.2" }).first();
    await expect(table).toBeVisible();
    // The lifecycle/flag column headers the brief's grouped table requires.
    for (const header of [/story/i, /status/i, /created/i, /applied/i, /refined/i, /flags/i]) {
      await expect(table.locator("th").filter({ hasText: header }).first()).toBeVisible();
    }
    // The header row has exactly the six column cells, and the body has
    // exactly the three story TABLE ROWS — each carrying its own status pill
    // and lifecycle chip (final review round 2: pin the full table shape, not
    // just the 60.2 row).
    await expect(table.locator("thead th")).toHaveCount(6);
    const rows = table.locator('tbody tr[data-testid="story-row"]');
    await expect(rows).toHaveCount(3);
    for (let i = 0; i < 3; i++) {
      await expect(rows.nth(i).getByTestId(MOCKUP_TID.statusPill).first()).toBeVisible();
      await expect(rows.nth(i).getByTestId(MOCKUP_TID.lifecycleChip).first()).toBeVisible();
    }
  });

  // ── m1.8 — the layout frame holds at the 1440px desktop reference ──
  // Data-driven over EVERY page (final review, finding 10): the document must
  // not scroll horizontally, and the content canvas must not clip content
  // sideways (overflow-x:hidden hiding wider content would show up as the
  // canvas scrollWidth exceeding its clientWidth).
  for (const name of PAGES) {
    test(`m1.8 ${name}: frame regions hold and nothing overflows at 1440px`, async ({ page }) => {
      await openMockup(page, name);
      if (name !== "sessions") {
        await expect(page.getByTestId(MOCKUP_TID.sidebar)).toBeVisible();
        await expect(page.getByTestId(MOCKUP_TID.breadcrumb)).toBeVisible();
        await expect(page.getByTestId(MOCKUP_TID.canvas)).toBeVisible();
      }
      const overflow = await page.evaluate(() => {
        const canvas = document.querySelector('[data-testid="mockup-canvas"]');
        return {
          docScrollWidth: document.documentElement.scrollWidth,
          docClientWidth: document.documentElement.clientWidth,
          canvasScrollWidth: canvas ? canvas.scrollWidth : 0,
          canvasClientWidth: canvas ? canvas.clientWidth : 0,
        };
      });
      // Allow a 1px rounding tolerance on each oracle.
      expect(
        overflow.docScrollWidth,
        "the document does not scroll horizontally at 1440px",
      ).toBeLessThanOrEqual(overflow.docClientWidth + 1);
      expect(
        overflow.canvasScrollWidth,
        "the content canvas does not clip content sideways at 1440px",
      ).toBeLessThanOrEqual(overflow.canvasClientWidth + 1);
    });
  }
});

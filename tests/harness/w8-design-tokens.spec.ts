/**
 * w8 — design TOKEN conformance (task `design-conformance-tests`, R1).
 *
 * DETERMINISTIC, never skips. Renders a real production workspace page and
 * asserts every VISIBLE element draws its colours and typography exclusively
 * from the S&F design-system tokens (paths.yaml → design_system,
 * `harness/design/system/tokens.css`) — ad-hoc colours or non-Poppins type
 * (outside the single scoped `file-view` monospace exception) fail the spec with
 * the offending element + value named.
 *
 * RED intent: any element rendering a colour/font outside the token palette is a
 * genuine design deviation (assertion failure, not a crash). Where the current
 * app is already token-disciplined a check is simply born green; once green the
 * check is the permanent token gate.
 */

import { expect, test } from "@playwright/test";

import {
  collectColorViolations,
  collectFontViolations,
  poppinsActuallyRenders,
} from "./helpers/design-conformance";
import { WTID, fileView, gotoWorkspace, wsRoutes } from "./helpers/ui";
import { WorkspaceEnv } from "./helpers/workspace-env";

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

/** Brings up the workspace and lands on the architecture file page, fully rendered. */
async function openArchitecture(page: import("@playwright/test").Page, slug: string): Promise<void> {
  env = await WorkspaceEnv.start(slug);
  const e = env;
  await gotoWorkspace(page, e.baseURL, wsRoutes.architecture(e.sid));
  await expect(page.getByTestId(WTID.rail)).toBeVisible();
  await expect(page.getByTestId(WTID.sidebar)).toBeVisible();
  await expect(fileView(page)).toBeVisible();
}

test("w8.1 every visible element's colours come only from the design-system token palette (R1)", async ({
  page,
}) => {
  await openArchitecture(page, "w8-token-colors");

  const violations = await collectColorViolations(page);
  const report = violations.map((v) => `${v.selector} { ${v.property}: ${v.value} }`).join("\n");

  expect(violations, `off-token colours on the workspace page:\n${report}`).toEqual([]);
});

test("w8.2 visible type is the Poppins design face (file-view monospace excepted) and the face truly renders (R1)", async ({
  page,
}) => {
  await openArchitecture(page, "w8-token-type");

  const violations = await collectFontViolations(page);
  const report = violations.map((v) => `${v.selector} { font-family(primary): ${v.value} }`).join("\n");
  expect(violations, `off-token typography on the workspace page:\n${report}`).toEqual([]);

  // Declared-vs-rendered: a wrong @font-face wiring resolves the family string
  // (so the check above passes) yet falls back to a system face at paint time.
  expect(await poppinsActuallyRenders(page), "the Poppins design face is not actually loaded/rendering").toBe(true);
});

test("w8.3 the chat-open form controls also stay inside the token palette (R1 — closes the closed-chat coverage gap)", async ({
  page,
}) => {
  await openArchitecture(page, "w8-token-chat");

  // The closed-chat architecture snapshot (w8.1/w8.2) never renders the docked
  // chat's input/send/new controls — the exact form controls the global UA
  // reset pulls back onto the token palette. Open the chat so they are VISIBLE
  // and get swept, turning that reset from an unproven change into a real gate.
  await page.getByTestId(WTID.chatDockToggle).click();
  await expect(page.getByTestId(WTID.chatDockPanel)).toBeVisible();
  // The sweeps silently skip invisible/absent elements, so prove the specific
  // form controls the reset targets are actually rendered + visible — otherwise
  // this test would pass VACUOUSLY if the chat controls never appeared.
  await expect(page.getByTestId(WTID.chatDockNew)).toBeVisible();
  await expect(page.getByTestId(WTID.chatDockInput)).toBeVisible();
  await expect(page.getByTestId(WTID.chatDockSend)).toBeVisible();

  const colorViolations = await collectColorViolations(page);
  const colorReport = colorViolations.map((v) => `${v.selector} { ${v.property}: ${v.value} }`).join("\n");
  expect(colorViolations, `off-token colours with the chat open:\n${colorReport}`).toEqual([]);

  const fontViolations = await collectFontViolations(page);
  const fontReport = fontViolations.map((v) => `${v.selector} { font-family(primary): ${v.value} }`).join("\n");
  expect(fontViolations, `off-token typography with the chat open:\n${fontReport}`).toEqual([]);
});

test("w8.4 the canvas padding + breadcrumb hairline render at the exact design spacing/border tokens (R1, deterministic)", async ({
  page,
}) => {
  await openArchitecture(page, "w8-token-frame");

  // Deterministic complement to w9's (nondeterministic) vision judge: assert the
  // two frame dimensions THIS design task established render at the EXACT design
  // tokens — anchored to the live token values (design/system/tokens.css →
  // globals), never a magic number. SPEC.md §1: the canvas has 40px (`--space-5`)
  // inner padding and the breadcrumb is separated by a `--border-width` hairline.
  const frame = await page.evaluate(() => {
    const root = getComputedStyle(document.documentElement);
    const pane = document.querySelector('[data-testid="content-pane"]');
    const crumb = document.querySelector('[data-testid="content-breadcrumb"]');
    const paneStyle = pane === null ? null : getComputedStyle(pane);
    const crumbStyle = crumb === null ? null : getComputedStyle(crumb);

    // Resolve --border-hairline to its computed rgb(...) via a probe (custom
    // properties read back their unresolved `var(--gray-100)` reference, so
    // apply it to a real border and let the engine resolve it).
    const probe = document.createElement("span");
    probe.style.borderBottomStyle = "solid";
    probe.style.borderBottomWidth = "3px";
    probe.style.borderBottomColor = "var(--border-hairline)";
    document.body.appendChild(probe);
    const hairlineColor = getComputedStyle(probe).borderBottomColor;
    probe.remove();

    return {
      space5: root.getPropertyValue("--space-5").trim(),
      borderWidth: root.getPropertyValue("--border-width").trim(),
      hairlineColor,
      panePresent: pane !== null,
      crumbPresent: crumb !== null,
      padding: paneStyle === null
        ? []
        : [paneStyle.paddingTop, paneStyle.paddingRight, paneStyle.paddingBottom, paneStyle.paddingLeft],
      crumbBorderBottomStyle: crumbStyle?.borderBottomStyle ?? "",
      crumbBorderBottomWidth: crumbStyle?.borderBottomWidth ?? "",
      crumbBorderBottomColor: crumbStyle?.borderBottomColor ?? "",
    };
  });

  expect(frame.panePresent, "content-pane is not present").toBe(true);
  expect(frame.crumbPresent, "the persistent breadcrumb bar is not present").toBe(true);
  // The design tokens must resolve to concrete values (guards against a missing
  // token silently making both sides of a comparison empty/equal).
  expect(frame.space5, "design token --space-5 did not resolve").toMatch(/^\d+px$/);
  expect(frame.borderWidth, "design token --border-width did not resolve").toMatch(/^\d+px$/);
  expect(frame.hairlineColor, "design token --border-hairline did not resolve to a colour").toMatch(/^rgba?\(/);
  expect(
    frame.padding,
    `canvas (content-pane) inner padding must equal --space-5 (${frame.space5}) on all four sides`,
  ).toEqual([frame.space5, frame.space5, frame.space5, frame.space5]);
  expect(frame.crumbBorderBottomStyle, "the breadcrumb must have a SOLID hairline bottom border").toBe("solid");
  expect(
    frame.crumbBorderBottomWidth,
    `the breadcrumb hairline bottom border must equal --border-width (${frame.borderWidth})`,
  ).toBe(frame.borderWidth);
  // Not merely SOME token colour (w8.1 already gates that) — the SPEC's specific
  // hairline token: `border-bottom: var(--border-width) solid var(--border-hairline)`.
  expect(
    frame.crumbBorderBottomColor,
    `the breadcrumb hairline colour must equal --border-hairline (${frame.hairlineColor})`,
  ).toBe(frame.hairlineColor);
});

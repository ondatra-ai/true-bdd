/**
 * w3 — live architecture outline, per-service provenance, exact-line jumps
 * (P13–P16). Derived views are asserted as DESCENDANTS of the correct service's
 * details region (present there, absent from the other) — not merely present
 * somewhere on the page.
 */

import { parse } from "yaml";
import { expect, test } from "@playwright/test";

import {
  WTID,
  archServiceDetails,
  archServiceRow,
  gotoWorkspace,
  lineIndex,
  railItem,
  readEditor,
  sidebarGroup,
  sidebarSection,
  writeEditor,
  wsRoutes,
} from "./helpers/ui";
import { WorkspaceEnv, runToken } from "./helpers/workspace-env";

const ARCH = "docs/architecture/architecture.yaml";

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

test("w3.1 outline: one row per service, flat Terms, Docker compose path; provenance scoped to the right service (P13/P15)", async ({
  page,
}) => {
  env = await WorkspaceEnv.start("w3-outline");
  const e = env;

  await gotoWorkspace(page, e.baseURL, wsRoutes.architecture(e.sid));
  const sidebar = sidebarSection(page, "architecture");

  const services = sidebarGroup(sidebar, "Services");
  await expect(services.getByTestId(WTID.archServiceRow)).toHaveCount(2); // one per service, no sub-tree
  await expect(archServiceRow(services, "mcp-service")).toBeVisible();
  await expect(archServiceRow(services, "postgres")).toBeVisible();

  await expect(sidebarGroup(sidebar, "Terms").getByTestId(WTID.archTermRow).first()).toBeVisible();
  await expect(sidebarGroup(sidebar, "Docker").getByTestId(WTID.archDockerRow)).toHaveText(/docker-compose\.yml/);

  // Tech stack + Docker provenance are DESCENDANTS of the correct service's
  // details region, absent from the other's.
  const mcp = archServiceDetails(page, "mcp-service");
  await expect(mcp.locator(`[data-testid="${WTID.serviceTech}"][data-tech="go"]`)).toBeVisible();
  await expect(mcp.locator(`[data-testid="${WTID.serviceTech}"][data-tech="net/http"]`)).toBeVisible();
  await expect(mcp.getByTestId(WTID.serviceDockerProvenance)).toContainText("services/mcp/Dockerfile");

  const pg = archServiceDetails(page, "postgres");
  await expect(pg.getByTestId(WTID.serviceDockerProvenance)).toContainText("docker-compose.yml#services.postgres");
  await expect(pg.locator(`[data-testid="${WTID.serviceTech}"][data-tech="go"]`)).toHaveCount(0);
});

test("w3.2 typing a redis service adds a Services row live; deleting it removes the row (P13/P9)", async ({ page }) => {
  env = await WorkspaceEnv.start("w3-live-outline");
  const e = env;

  await gotoWorkspace(page, e.baseURL, wsRoutes.architecture(e.sid));
  const services = sidebarGroup(sidebarSection(page, "architecture"), "Services");
  await expect(services.getByTestId(WTID.archServiceRow)).toHaveCount(2);

  const editor = page.getByTestId(WTID.fileViewEditor);
  const original = await readEditor(editor);
  const redisBlock = ["  redis:", "    type: redis", "    image: redis:7-alpine", "    port: 6379"].join("\n");
  await writeEditor(editor, original.replace(/\nterms:/, `\n${redisBlock}\nterms:`));

  await expect(archServiceRow(services, "redis")).toBeVisible();
  await expect(services.getByTestId(WTID.archServiceRow)).toHaveCount(3);

  // Live removal, not just addition.
  await writeEditor(editor, original);
  await expect(archServiceRow(services, "redis")).toHaveCount(0);
  await expect(services.getByTestId(WTID.archServiceRow)).toHaveCount(2);
});

test("w3.3 outline entry jumps to the exact line (mid-file, near-end clamp, cross-page) (P14)", async ({ page }) => {
  env = await WorkspaceEnv.start("w3-jumps");
  const e = env;

  await gotoWorkspace(page, e.baseURL, wsRoutes.architecture(e.sid));
  const sidebar = sidebarSection(page, "architecture");
  const postgresLine = lineIndex(e.readDocOnDisk(ARCH), /^ {2}postgres:/);

  // Mid-file target (postgres, not near the file end) → the flash lands on its line.
  await archServiceRow(sidebar, "postgres").click();
  const flash = page.getByTestId(WTID.fileViewFlash);
  await expect(flash).toBeVisible();
  await expect(flash).toHaveAttribute("data-line", String(postgresLine));

  // Near-end target (Docker compose row) → CLAMPED: scrollTop == max.
  await sidebarGroup(sidebar, "Docker").getByTestId(WTID.archDockerRow).click();
  const clamped = await page
    .getByTestId(WTID.fileViewEditor)
    .evaluate((el) => Math.abs(el.scrollTop - (el.scrollHeight - el.clientHeight)) <= 2);
  expect(clamped).toBe(true);

  // Cross-page: from a story page, click an architecture outline entry in the
  // flyout → navigate first, THEN scroll to the line (scroll={false}).
  await gotoWorkspace(page, e.baseURL, wsRoutes.story(e.sid, "60.2"));
  await railItem(page, "architecture").hover();
  await archServiceRow(page.getByTestId(WTID.railFlyout), "postgres").click();
  await expect(page).toHaveURL(new RegExp(`/sessions/${e.sid}/architecture$`));
  await expect(page.getByTestId(WTID.fileViewFlash)).toHaveAttribute("data-line", String(postgresLine));
});

test("w3.4 endpoints on the custom service, connection info on the supporting one; endpoints editable + persisted (P16)", async ({
  page,
}) => {
  env = await WorkspaceEnv.start("w3-endpoints");
  const e = env;

  await gotoWorkspace(page, e.baseURL, wsRoutes.architecture(e.sid));

  const mcp = archServiceDetails(page, "mcp-service");
  await expect(
    mcp.locator(`[data-testid="${WTID.serviceEndpoint}"][data-method="POST"][data-path="/mcp"]`),
  ).toBeVisible();
  await expect(mcp.getByTestId(WTID.serviceConnection)).toHaveCount(0);

  const pg = archServiceDetails(page, "postgres");
  await expect(pg.locator(`[data-testid="${WTID.serviceConnection}"][data-key="port"]`)).toContainText("5432");
  await expect(pg.getByTestId(WTID.serviceEndpoint)).toHaveCount(0);

  // Declare a NEW endpoint (unique path) for mcp-service → appears in the derived
  // view AND persists on disk (parsed under the mcp-service node).
  const uniquePath = `/probe-${runToken("ep").replace(/[^a-z0-9-]/gi, "")}`;
  const editor = page.getByTestId(WTID.fileViewEditor);
  const current = await readEditor(editor);
  const withEndpoint = current.replace(
    /(\n {4}endpoints:\n)/,
    `$1      - method: GET\n        path: ${uniquePath}\n        summary: probe endpoint\n`,
  );
  await writeEditor(editor, withEndpoint);

  await expect(mcp.locator(`[data-testid="${WTID.serviceEndpoint}"][data-path="${uniquePath}"]`)).toBeVisible();
  const bytes = await e.waitForDocOnDisk(ARCH, (b) => b.includes(uniquePath));
  const doc = parse(bytes) as { services: { "mcp-service": { endpoints: Array<{ path: string }> } } };
  expect(doc.services["mcp-service"].endpoints.map((ep) => ep.path)).toContain(uniquePath);
});

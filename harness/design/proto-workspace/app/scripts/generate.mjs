// Throwaway generator: extracts the main-content blob (breadcrumb + <main> +
// any trailing dialog scrim) out of each static mockup HTML page and writes
// it as a content/<slug>.html fragment, plus a matching Next.js route that
// injects it via dangerouslySetInnerHTML. Run once by hand; not part of the
// Next.js build/runtime.
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const APP_ROOT = path.resolve(__dirname, "..");
const MOCKUPS_DIR = path.resolve(APP_ROOT, "../../harness/design/mockups");

const SCRIM_PAGES = new Set(["prompt-choice", "prompt-clarify", "prompt-freetext"]);

const files = fs
  .readdirSync(MOCKUPS_DIR)
  .filter((f) => f.endsWith(".html"))
  .map((f) => f.replace(/\.html$/, ""));

const slugs = new Set(files);

function extractTitle(html) {
  const m = html.match(/<title>([\s\S]*?)<\/title>/);
  return m ? m[1].trim() : "TrueBDD Workspace";
}

function rewriteLinks(html) {
  return html.replace(/href="([a-z0-9-]+)\.html(\?[^"]*)?"/g, (m, slug, qs) => {
    if (!slugs.has(slug)) return m;
    return `href="/${slug}${qs || ""}"`;
  });
}

function extractBody(html, slug) {
  const start = html.indexOf('<nav class="mockup-breadcrumb"');
  const mainCloseTag = "</main>";
  const mainCloseIdx = html.indexOf(mainCloseTag, start);
  if (start === -1 || mainCloseIdx === -1) {
    throw new Error(`could not find breadcrumb/main in ${slug}.html`);
  }
  let body = html.slice(start, mainCloseIdx + mainCloseTag.length);
  if (SCRIM_PAGES.has(slug)) {
    body += '\n<div class="mockup-scrim"></div>';
  }
  return rewriteLinks(body);
}

const contentDir = path.join(APP_ROOT, "content");
const workspaceDir = path.join(APP_ROOT, "app", "(workspace)");
fs.mkdirSync(contentDir, { recursive: true });
fs.mkdirSync(workspaceDir, { recursive: true });

const routes = [];

for (const slug of files) {
  if (slug === "sessions") continue; // no sidebar — handled separately
  const raw = fs.readFileSync(path.join(MOCKUPS_DIR, `${slug}.html`), "utf8");
  const title = extractTitle(raw);
  const body = extractBody(raw, slug);
  fs.writeFileSync(path.join(contentDir, `${slug}.html`), body, "utf8");

  const pageDir = path.join(workspaceDir, slug);
  fs.mkdirSync(pageDir, { recursive: true });
  fs.writeFileSync(
    path.join(pageDir, "page.js"),
    `import fs from "node:fs";
import path from "node:path";

export const metadata = { title: ${JSON.stringify(title)} };

const html = fs.readFileSync(
  path.join(process.cwd(), "content", "${slug}.html"),
  "utf8"
);

export default function Page() {
  return <div dangerouslySetInnerHTML={{ __html: html }} />;
}
`,
    "utf8"
  );
  routes.push(slug);
}

// sessions.html has no sidebar — lives directly under the root layout.
{
  const raw = fs.readFileSync(path.join(MOCKUPS_DIR, "sessions.html"), "utf8");
  const title = extractTitle(raw);
  const bodyMatch = raw.match(/<body[^>]*>([\s\S]*?)<script/);
  const body = rewriteLinks(bodyMatch[1]);
  fs.writeFileSync(path.join(contentDir, "sessions.html"), body, "utf8");
  const pageDir = path.join(APP_ROOT, "app", "sessions");
  fs.mkdirSync(pageDir, { recursive: true });
  fs.writeFileSync(
    path.join(pageDir, "page.js"),
    `import fs from "node:fs";
import path from "node:path";

export const metadata = { title: ${JSON.stringify(title)} };

const html = fs.readFileSync(
  path.join(process.cwd(), "content", "sessions.html"),
  "utf8"
);

export default function Page() {
  return <div className="app-shell" dangerouslySetInnerHTML={{ __html: html }} />;
}
`,
    "utf8"
  );
}

console.log("Generated routes:", routes.join(", "));
console.log("Plus: /sessions (no sidebar)");

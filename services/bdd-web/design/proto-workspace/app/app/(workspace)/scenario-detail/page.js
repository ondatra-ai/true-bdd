import fs from "node:fs";
import path from "node:path";

export const metadata = { title: "E2E-601 — TrueBDD Workspace" };

const html = fs.readFileSync(
  path.join(process.cwd(), "content", "scenario-detail.html"),
  "utf8"
);

export default function Page() {
  return <div dangerouslySetInnerHTML={{ __html: html }} />;
}

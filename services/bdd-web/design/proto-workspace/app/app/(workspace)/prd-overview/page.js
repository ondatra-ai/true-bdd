import fs from "node:fs";
import path from "node:path";

export const metadata = { title: "PRD overview — TrueBDD Workspace" };

const html = fs.readFileSync(
  path.join(process.cwd(), "content", "prd-overview.html"),
  "utf8"
);

export default function Page() {
  return <div dangerouslySetInnerHTML={{ __html: html }} />;
}

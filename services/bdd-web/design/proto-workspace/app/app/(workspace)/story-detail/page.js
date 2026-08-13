import fs from "node:fs";
import path from "node:path";

export const metadata = { title: "60.2 — Summary For Shared Docs — TrueBDD Workspace" };

const html = fs.readFileSync(
  path.join(process.cwd(), "content", "story-detail.html"),
  "utf8"
);

export default function Page() {
  return <div dangerouslySetInnerHTML={{ __html: html }} />;
}

import fs from "node:fs";
import path from "node:path";

export const metadata = { title: "Requirements / Scenarios — TrueBDD Workspace" };

const html = fs.readFileSync(
  path.join(process.cwd(), "content", "scenarios.html"),
  "utf8"
);

export default function Page() {
  return <div dangerouslySetInnerHTML={{ __html: html }} />;
}

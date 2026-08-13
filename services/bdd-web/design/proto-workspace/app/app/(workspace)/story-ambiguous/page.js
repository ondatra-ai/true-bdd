import fs from "node:fs";
import path from "node:path";

export const metadata = { title: "Story lookup: ambiguous match — TrueBDD Workspace" };

const html = fs.readFileSync(
  path.join(process.cwd(), "content", "story-ambiguous.html"),
  "utf8"
);

export default function Page() {
  return <div dangerouslySetInnerHTML={{ __html: html }} />;
}

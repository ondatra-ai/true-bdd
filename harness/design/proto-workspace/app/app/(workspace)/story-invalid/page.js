import fs from "node:fs";
import path from "node:path";

export const metadata = { title: "Story file: parse error — TrueBDD Workspace" };

const html = fs.readFileSync(
  path.join(process.cwd(), "content", "story-invalid.html"),
  "utf8"
);

export default function Page() {
  return <div dangerouslySetInnerHTML={{ __html: html }} />;
}

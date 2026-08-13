import fs from "node:fs";
import path from "node:path";

export const metadata = { title: "Sessions — TrueBDD Workspace" };

const html = fs.readFileSync(
  path.join(process.cwd(), "content", "sessions.html"),
  "utf8"
);

export default function Page() {
  return <div className="app-shell" dangerouslySetInnerHTML={{ __html: html }} />;
}

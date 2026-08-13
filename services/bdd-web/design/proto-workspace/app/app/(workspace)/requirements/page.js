"use client";

import Link from "next/link";
import { useMemo } from "react";
import FileView from "../../../components/FileView";
import { useFile, SCENARIOS_PATH } from "../../../components/FilesStore";
import { FILE_TOP_ID, parseScenariosOutline, storySlugFromId } from "../../../components/ProductFiles";

// Registry table (report F12): prod's Scenarios page shows a
// Scenario / Description / Service / Linked-story table above the whole-file
// YAML view — a strictly richer affordance the mockup was missing. Rendered
// live from the same scenarios.yaml the file view below shows, so editing the
// file (by hand or via chat) re-derives the table. Clicking a scenario id
// jumps to its line in the file; the linked-story cell deep-links to that
// story's own page.
function ScenariosRegistry({ scenarios, onJump }) {
  return (
    <table className="data-table" data-testid="scenarios-registry">
      <thead>
        <tr>
          <th>Scenario</th>
          <th>Description</th>
          <th>Service</th>
          <th>Linked story</th>
        </tr>
      </thead>
      <tbody>
        {scenarios.map((s) => {
          const storyId = s.storyPath ? s.storyPath.match(/stories\/(\d+\.\d+)-/)?.[1] ?? null : null;
          return (
            <tr key={s.id} data-testid="scenario-row">
              <td>
                <Link href="/requirements" scroll={false} onClick={() => onJump(s.anchorId)}>
                  {s.id}
                </Link>
              </td>
              <td>{s.description}</td>
              <td>{s.service || "—"}</td>
              <td>
                {storyId ? <Link href={`/story/${storySlugFromId(storyId)}`}>{storyId}</Link> : "—"}
              </td>
            </tr>
          );
        })}
      </tbody>
    </table>
  );
}

export default function RequirementsPage() {
  const { content, setContent, requestJump } = useFile(SCENARIOS_PATH);

  const outline = useMemo(() => parseScenariosOutline(content), [content]);
  const anchors = useMemo(
    () => [
      { id: FILE_TOP_ID, line: 0 },
      ...outline.scenarios.map((s) => ({ id: s.anchorId, line: s.line })),
    ],
    [outline]
  );

  return (
    <>
      <nav
        className="mockup-breadcrumb"
        data-testid="mockup-breadcrumb"
        aria-label="Breadcrumb"
      >
        <Link href="/sessions">Sessions</Link>
        <span className="crumb-sep">/</span>
        <Link href="/workspace-overview">Workspace overview</Link>
        <span className="crumb-sep">/</span>
        <Link href="/product">Product</Link>
        <span className="crumb-sep">/</span>
        <span aria-current="page">scenarios.yaml</span>
      </nav>

      <FileView
        kicker="02—Product / Scenarios"
        title="scenarios.yaml"
        meta="Whole-file view, GitHub-style. Try a chat message containing “scenario” to add a new one."
        path={SCENARIOS_PATH}
        content={content}
        onChange={setContent}
        anchors={anchors}
        beforeFile={<ScenariosRegistry scenarios={outline.scenarios} onJump={requestJump} />}
      />
    </>
  );
}

"use client";

import Link from "next/link";
import { useMemo } from "react";
import FileView from "../../../components/FileView";
import { useFile, SCENARIOS_PATH } from "../../../components/FilesStore";
import { FILE_TOP_ID, parseScenariosOutline } from "../../../components/ProductFiles";

export default function RequirementsPage() {
  const { content, setContent } = useFile(SCENARIOS_PATH);

  const anchors = useMemo(() => {
    const outline = parseScenariosOutline(content);
    return [{ id: FILE_TOP_ID, line: 0 }, ...outline.scenarios.map((s) => ({ id: s.anchorId, line: s.line }))];
  }, [content]);

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
      />
    </>
  );
}

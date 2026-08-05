"use client";

import Link from "next/link";
import FileView from "../../../components/FileView";
import { useFile, PRD_PATH } from "../../../components/FilesStore";
import { FILE_TOP_ID } from "../../../components/ProductFiles";

const ANCHORS = [{ id: FILE_TOP_ID, line: 0 }];

export default function ProductPage() {
  const { content, setContent } = useFile(PRD_PATH);

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
        <span aria-current="page">prd.yaml</span>
      </nav>

      <FileView
        kicker="02—Product"
        title="prd.yaml"
        meta="Whole-file view, GitHub-style — PRD goals + personas. Edit directly below, or ask the chat to change it."
        path={PRD_PATH}
        content={content}
        onChange={setContent}
        anchors={ANCHORS}
      />
    </>
  );
}

"use client";

import Link from "next/link";
import FileView from "../../../components/FileView";
import { useFile, PRODUCT_PATH } from "../../../components/FilesStore";
import { FILE_TOP_ID } from "../../../components/ProductFiles";

const ANCHORS = [{ id: FILE_TOP_ID, line: 0 }];

export default function ProductPage() {
  const { content, setContent } = useFile(PRODUCT_PATH);

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
        <span aria-current="page">product.yaml</span>
      </nav>

      <FileView
        kicker="02—Product"
        title="product.yaml"
        meta="Whole-file view, GitHub-style — product document goals + roles. Edit directly below, or ask the chat to change it."
        path={PRODUCT_PATH}
        content={content}
        onChange={setContent}
        anchors={ANCHORS}
      />
    </>
  );
}

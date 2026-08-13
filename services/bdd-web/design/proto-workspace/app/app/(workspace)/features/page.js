"use client";

import Link from "next/link";
import FileView from "../../../components/FileView";
import { useFile, FEATURES_PATH } from "../../../components/FilesStore";
import { FILE_TOP_ID } from "../../../components/ProductFiles";

const ANCHORS = [{ id: FILE_TOP_ID, line: 0 }];

// features.yaml itself — deliberately minimal (id/slug + description only,
// steering correction: no name, no embedded story/requirement lists). The
// alignment those imply lives on the STORY/SCENARIO side (`feature:` refs);
// see /feature/<slug> for the derived aggregation view.
export default function FeaturesPage() {
  const { content, setContent } = useFile(FEATURES_PATH);

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
        <span aria-current="page">features.yaml</span>
      </nav>

      <FileView
        kicker="02—Product / Features"
        title="features.yaml"
        meta="Whole-file view, GitHub-style. Each feature is just an id + description — stories and scenarios reference it by id; see a feature's own page for what's aligned to it."
        path={FEATURES_PATH}
        content={content}
        onChange={setContent}
        anchors={ANCHORS}
      />
    </>
  );
}

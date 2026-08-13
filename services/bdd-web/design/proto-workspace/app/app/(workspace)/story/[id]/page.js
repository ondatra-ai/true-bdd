"use client";

import Link from "next/link";
import { useParams } from "next/navigation";
import { useMemo } from "react";
import FileView from "../../../../components/FileView";
import { useFile, useFiles, FEATURES_PATH } from "../../../../components/FilesStore";
import {
  deriveStories,
  parseFeaturesOutline,
  setStoryFeature,
  appendFeatureStub,
  storyIdFromSlug,
  FILE_TOP_ID,
} from "../../../../components/ProductFiles";
import FeaturePicker from "../../../../components/FeaturePicker";

const ANCHORS = [{ id: FILE_TOP_ID, line: 0 }];
// Fallback path so useFile() always has a stable key to call (hook order
// must not depend on whether the story was found) — never rendered.
const MISSING_PATH = "docs/prd/stories/__missing__.yaml";

export default function StoryPage() {
  const params = useParams();
  const filesCtx = useFiles();
  const { files, setFile } = filesCtx;
  // No epic to resolve against — the story is looked up straight from the
  // story FILES living in the store (id/title read from each file's own
  // content), matched against the id encoded in the URL slug.
  const stories = useMemo(() => deriveStories(files), [files]);

  const storyId = storyIdFromSlug(params.id);
  const entry = stories.find((s) => s.id === storyId);
  const filePath = entry?.file || MISSING_PATH;
  const { content, setContent } = useFile(filePath);
  const fileName = filePath.split("/").pop();
  const features = parseFeaturesOutline(files[FEATURES_PATH] || "").features;

  // Slim Feature strip above the file view (steering: the user must be
  // able to CHANGE a story's feature via UI, not just by hand-editing
  // YAML) — picking rewrites the `feature:` line in THIS story's content
  // right away, visible immediately in the FileView below since it shares
  // the same `content`/`setContent` state.
  function handleFeatureSelect(newId) {
    setContent((prev) => setStoryFeature(prev, newId));
  }
  function handleFeatureCreate(name) {
    const { updatedFeatures, id } = appendFeatureStub(files[FEATURES_PATH] || "", name);
    setFile(FEATURES_PATH, updatedFeatures);
    setContent((prev) => setStoryFeature(prev, id));
  }

  if (!entry) {
    return (
      <main className="mockup-canvas canvas-max" data-testid="mockup-canvas">
        <div className="page-header">
          <div>
            <span className="section-label">02—Product</span>
            <h1>Story {storyId} not found</h1>
            <p className="page-header__meta">
              No story file under docs/prd/stories/ has id "{storyId}".{" "}
              <Link href="/product">Back to Product</Link>.
            </p>
          </div>
        </div>
      </main>
    );
  }

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
        <span aria-current="page">{fileName}</span>
      </nav>

      <div className="feature-strip" data-testid="feature-strip">
        <FeaturePicker
          features={features}
          value={entry.feature}
          onSelect={handleFeatureSelect}
          onCreate={handleFeatureCreate}
          testId="story-feature-picker"
        />
      </div>

      <FileView
        kicker={`02—Product / ${entry.id}`}
        title={`${entry.id} — ${entry.title}`}
        meta="Whole-file view, GitHub-style. Try a chat message containing “story” to add another one."
        path={filePath}
        content={content}
        onChange={setContent}
        anchors={ANCHORS}
      />
    </>
  );
}

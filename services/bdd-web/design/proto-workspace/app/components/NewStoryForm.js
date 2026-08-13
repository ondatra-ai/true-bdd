"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { useFiles, FEATURES_PATH } from "./FilesStore";
import { deriveStories, parseFeaturesOutline, appendFeatureStub, createStoryFile, storySlugFromId } from "./ProductFiles";
import FeaturePicker from "./FeaturePicker";

// Sits at the bottom of the sidebar's Stories: section (see sections.js).
// Steering: "when user creates new story from scratch it selects either
// existing feature from list or create new feature" — the same shared
// FeaturePicker used on story pages / aggregation rows, just wired to a
// brand-new story instead of an existing one. Submit is disabled until
// BOTH a title and a resolved feature id exist (picked or freshly created).
export default function NewStoryForm() {
  const filesCtx = useFiles();
  const router = useRouter();
  const [open, setOpen] = useState(false);
  const [title, setTitle] = useState("");
  const [featureId, setFeatureId] = useState(null);

  const features = parseFeaturesOutline(filesCtx.files[FEATURES_PATH] || "").features;

  function reset() {
    setTitle("");
    setFeatureId(null);
    setOpen(false);
  }

  function handleCreateFeature(name) {
    if (!name) return;
    const { updatedFeatures, id } = appendFeatureStub(filesCtx.files[FEATURES_PATH] || "", name);
    filesCtx.setFile(FEATURES_PATH, updatedFeatures);
    setFeatureId(id);
  }

  function submit() {
    const trimmedTitle = title.trim();
    if (!trimmedTitle || !featureId) return;
    const existingIds = deriveStories(filesCtx.files).map((s) => s.id);
    const { filePath, storyStub, newId } = createStoryFile(existingIds, trimmedTitle, featureId);
    filesCtx.setFile(filePath, storyStub);
    reset();
    router.push(`/story/${storySlugFromId(newId)}`);
  }

  if (!open) {
    return (
      <button
        type="button"
        className="sidebar-add-row"
        onClick={() => setOpen(true)}
        data-testid="new-story-toggle"
      >
        + New story
      </button>
    );
  }

  return (
    <div className="new-story-form" data-testid="new-story-form">
      <input
        type="text"
        placeholder="Story title…"
        value={title}
        onChange={(e) => setTitle(e.target.value)}
        aria-label="New story title"
        className="new-story-form__title"
        data-testid="new-story-title"
      />
      <FeaturePicker
        features={features}
        value={featureId}
        onSelect={setFeatureId}
        onCreate={handleCreateFeature}
        testId="new-story-feature-picker"
      />
      <div className="new-story-form__actions">
        <button
          type="button"
          className="btn btn--solid"
          onClick={submit}
          disabled={!title.trim() || !featureId}
          data-testid="new-story-submit"
        >
          Add
        </button>
        <button type="button" className="btn" onClick={reset} data-testid="new-story-cancel">
          Cancel
        </button>
      </div>
    </div>
  );
}

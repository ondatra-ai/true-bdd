"use client";

/**
 * New-story form (P22): the feature picker is REQUIRED (submit is blocked
 * without one). Two paths: pick an EXISTING feature (no extra write), or
 * "+ Create" a novel one — ordered feature-stub-first, THEN the story file
 * (plan r2 #16), so a partial failure leaves at most a benign unreferenced
 * feature, never a story pointing at a missing one.
 */

import { useState } from "react";

import { appendFeatureStub, deriveFeatures, slugify } from "@/app/lib/workspace/derive";
import { effectiveContent, useFiles } from "@/app/lib/workspace/files-context";
import { FEATURES_PATH } from "@/app/lib/workspace/types";

import { FeaturePicker } from "./FeaturePicker";

function nextStorySlug(title: string): string {
  const id = Date.now().toString(36);
  const slug = slugify(title || "untitled");

  return `${id}-${slug}`;
}

export function NewStoryForm({ onClose }: { onClose: () => void }) {
  const { getDoc, commitBufferNow, refetchTree } = useFiles();
  const [title, setTitle] = useState("");
  const [feature, setFeature] = useState<string | undefined>(undefined);
  const [pendingNewFeature, setPendingNewFeature] = useState<string | undefined>(undefined);
  const [submitting, setSubmitting] = useState(false);

  const featuresDoc = getDoc(FEATURES_PATH);
  const featuresContent = effectiveContent(featuresDoc);
  const features = deriveFeatures(featuresContent);

  function handlePickExisting(id: string) {
    setFeature(id);
    setPendingNewFeature(undefined);
  }

  function handleCreateNew(id: string) {
    setFeature(id);
    setPendingNewFeature(id);
  }

  async function handleSubmit() {
    if (feature === undefined || submitting) {
      return; // required — blocked
    }

    setSubmitting(true);
    try {
      if (pendingNewFeature !== undefined) {
        const currentFeaturesDoc = getDoc(FEATURES_PATH);
        const newFeaturesContent = appendFeatureStub(
          effectiveContent(currentFeaturesDoc),
          pendingNewFeature,
          `${title.trim() || pendingNewFeature} — created inline`,
        );
        const ok = await commitBufferNow(FEATURES_PATH, newFeaturesContent, currentFeaturesDoc?.syncedRevision);
        if (!ok) {
          setSubmitting(false);

          return;
        }
      }

      const slug = nextStorySlug(title);
      const path = `docs/prd/stories/${slug}.yaml`;
      const safeTitle = title.trim().replace(/"/g, '\\"');
      const content = `story:\n  id: "${slug}"\n  title: "${safeTitle}"\n  feature: ${feature}\n`;
      const created = await commitBufferNow(path, content, "absent");
      if (created) {
        await refetchTree();
        onClose();
      }
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div data-testid="new-story-form">
      <label>
        Title
        <input
          data-testid="new-story-title"
          value={title}
          onChange={(event) => setTitle(event.target.value)}
        />
      </label>
      <FeaturePicker features={features} value={feature} onPick={handlePickExisting} onCreate={handleCreateNew} />
      {/* Required (P22): the button stays a real, clickable control (never
          HTML `disabled`, which would hang a Playwright `.click()` waiting
          for "enabled") — the guard lives in handleSubmit, which no-ops
          without a feature. */}
      <button type="button" data-testid="new-story-submit" onClick={() => void handleSubmit()}>
        Create story
      </button>
    </div>
  );
}

"use client";

// A single story file page (P18): flat, standalone — no epic grouping (P19).
// Carries the same searchable feature picker as the aggregation page (P23):
// changing it writes straight into the underlying YAML file.

import { use } from "react";

import { FeaturePicker } from "@/app/components/workspace/FeaturePicker";
import { FileView } from "@/app/components/workspace/FileView";
import { appendFeatureStub, deriveFeatures, deriveStory, withFieldSet } from "@/app/lib/workspace/derive";
import { effectiveContent, useFiles } from "@/app/lib/workspace/files-context";
import { FEATURES_PATH, isStoryPath } from "@/app/lib/workspace/types";

export default function StoryPage({ params }: { params: Promise<{ storyId: string }> }) {
  const { storyId } = use(params);
  const { manifest, getDoc, setBuffer, commitBufferNow } = useFiles();

  const storyEntry = manifest.find((entry) => {
    if (!isStoryPath(entry.path)) return false;

    return deriveStory(effectiveContent(getDoc(entry.path)))?.id === storyId;
  });

  const featuresContent = effectiveContent(getDoc(FEATURES_PATH));
  const features = deriveFeatures(featuresContent);

  if (storyEntry === undefined) {
    return <div>Story {storyId} not found.</div>;
  }

  const storyContent = effectiveContent(getDoc(storyEntry.path));
  const storyInfo = deriveStory(storyContent);

  function assignFeature(newId: string) {
    setBuffer(storyEntry!.path, withFieldSet(storyContent, ["story", "feature"], newId));
  }

  async function createAndAssign(newId: string): Promise<void> {
    const currentFeaturesDoc = getDoc(FEATURES_PATH);
    const newFeaturesContent = appendFeatureStub(
      effectiveContent(currentFeaturesDoc),
      newId,
      `${newId} — created inline`,
    );
    await commitBufferNow(FEATURES_PATH, newFeaturesContent, currentFeaturesDoc?.syncedRevision);
    assignFeature(newId);
  }

  return (
    <div>
      <FileView docPath={storyEntry.path} />
      <div style={{ marginTop: "1rem" }}>
        <FeaturePicker
          features={features}
          value={storyInfo?.feature}
          onPick={assignFeature}
          onCreate={(newId) => void createAndAssign(newId)}
        />
      </div>
    </div>
  );
}

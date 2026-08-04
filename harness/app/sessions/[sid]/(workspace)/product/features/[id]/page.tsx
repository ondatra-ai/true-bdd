"use client";

// Feature aggregation page (P21/P24/P25) — derived, no backing file:
// description (from features.yaml) plus every story/scenario whose
// `feature:` matches, re-bucketing live on any reference change (by hand,
// picker, or chat — the chat path is w6.3). Also surfaces the workspace-wide
// unaligned bucket (no-feature OR dangling-ref scenarios, P24/P25) so a
// retro-tag control is always reachable from a feature-alignment page.

import { use } from "react";

import { FeaturePicker } from "@/app/components/workspace/FeaturePicker";
import { appendFeatureStub, deriveFeatures, deriveScenarios, deriveStory, withFieldSet } from "@/app/lib/workspace/derive";
import { effectiveContent, useFiles } from "@/app/lib/workspace/files-context";
import { FEATURES_PATH, SCENARIOS_PATH, isStoryPath } from "@/app/lib/workspace/types";

export default function FeatureAggregationPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  const { manifest, getDoc, setBuffer, commitBufferNow } = useFiles();

  const featuresContent = effectiveContent(getDoc(FEATURES_PATH));
  const features = deriveFeatures(featuresContent);
  const featureIds = new Set(features.map((f) => f.id));
  const current = features.find((f) => f.id === id);

  const storiesWithInfo = manifest
    .filter((entry) => isStoryPath(entry.path))
    .map((entry) => {
      const content = effectiveContent(getDoc(entry.path));

      return { path: entry.path, content, info: deriveStory(content) };
    });
  const matchingStories = storiesWithInfo.filter((s) => s.info?.feature === id);

  const scenariosContent = effectiveContent(getDoc(SCENARIOS_PATH));
  const scenarios = deriveScenarios(scenariosContent);
  const matchingScenarios = scenarios.filter((s) => s.feature === id);
  const unaligned = scenarios.filter(
    (s) => s.feature === undefined || s.feature === "" || !featureIds.has(s.feature),
  );

  function reassignStory(path: string, content: string, newFeature: string) {
    setBuffer(path, withFieldSet(content, ["story", "feature"], newFeature));
  }

  async function createFeatureStub(newFeatureId: string): Promise<void> {
    const currentFeaturesDoc = getDoc(FEATURES_PATH);
    const newFeaturesContent = appendFeatureStub(
      effectiveContent(currentFeaturesDoc),
      newFeatureId,
      `${newFeatureId} — created inline`,
    );
    await commitBufferNow(FEATURES_PATH, newFeaturesContent, currentFeaturesDoc?.syncedRevision);
  }

  function reassignScenario(scenarioId: string, newFeature: string) {
    setBuffer(SCENARIOS_PATH, withFieldSet(scenariosContent, ["scenarios", scenarioId, "feature"], newFeature));
  }

  return (
    <div>
      <h1>{id}</h1>
      <div data-testid="feature-description">{current?.description ?? ""}</div>

      <h2>Stories</h2>
      <div data-testid="feature-stories-list">
        {matchingStories.map((story) => (
          <div key={story.path} data-testid="feature-story-row" data-story-id={story.info?.id}>
            <span>{story.info?.id}</span>
            <FeaturePicker
              features={features}
              value={story.info?.feature}
              onPick={(newId) => reassignStory(story.path, story.content, newId)}
              onCreate={(newId) => {
                void createFeatureStub(newId).then(() => reassignStory(story.path, story.content, newId));
              }}
            />
          </div>
        ))}
      </div>

      <h2>Requirements</h2>
      <div data-testid="feature-scenarios-list">
        {matchingScenarios.map((scenario) => (
          <div key={scenario.id} data-testid="feature-scenario-row" data-scenario-id={scenario.id}>
            {scenario.id}
          </div>
        ))}
      </div>

      <h2>Unaligned</h2>
      <div data-testid="unaligned-bucket">
        {unaligned.map((scenario) => {
          const dangling =
            scenario.feature !== undefined && scenario.feature !== "" && !featureIds.has(scenario.feature);

          return (
            <div
              key={scenario.id}
              data-testid="unaligned-scenario-row"
              data-scenario-id={scenario.id}
              data-dangling={String(dangling)}
              data-dangling-ref={dangling ? scenario.feature : ""}
            >
              <span>{scenario.id}</span>
              <FeaturePicker
                features={features}
                onPick={(newId) => reassignScenario(scenario.id, newId)}
                onCreate={(newId) => {
                  void createFeatureStub(newId).then(() => reassignScenario(scenario.id, newId));
                }}
              />
            </div>
          );
        })}
      </div>
    </div>
  );
}

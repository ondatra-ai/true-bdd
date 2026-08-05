"use client";

import Link from "next/link";
import { useParams } from "next/navigation";
import { useFiles, FEATURES_PATH, SCENARIOS_PATH } from "../../../../components/FilesStore";
import {
  parseFeaturesOutline,
  useFeatureAggregate,
  setStoryFeature,
  setScenarioFeature,
  appendFeatureStub,
  storySlugFromId,
  scnAnchorId,
} from "../../../../components/ProductFiles";
import FeaturePicker from "../../../../components/FeaturePicker";

// /feature/<id> — NOT a file page: a LIVE DERIVED aggregation. features.yaml
// itself only ever holds id+description (steering correction); every story
// and scenario shown here is found by scanning THEIR OWN `feature:`
// reference, so reassigning either one via the FeaturePicker inline on its
// row re-buckets it immediately, no second source of truth to keep in sync.
export default function FeaturePage() {
  const params = useParams();
  const featureId = params.id;
  const filesCtx = useFiles();

  const featuresYaml = filesCtx.files[FEATURES_PATH] || "";
  const features = parseFeaturesOutline(featuresYaml).features;
  const feature = features.find((f) => f.id === featureId);
  const { stories, scenarios, unalignedScenarios } = useFeatureAggregate(featureId);

  function createFeature(name) {
    const { updatedFeatures, id } = appendFeatureStub(filesCtx.files[FEATURES_PATH] || "", name);
    filesCtx.setFile(FEATURES_PATH, updatedFeatures);
    return id;
  }

  function reassignStory(story) {
    return {
      onSelect: (newId) => filesCtx.setFile(story.file, (prev) => setStoryFeature(prev, newId)),
      onCreate: (name) => {
        const newId = createFeature(name);
        filesCtx.setFile(story.file, (prev) => setStoryFeature(prev, newId));
      },
    };
  }

  function reassignScenario(scn) {
    return {
      onSelect: (newId) =>
        filesCtx.setFile(SCENARIOS_PATH, (prev) => setScenarioFeature(prev, scn.id, newId)),
      onCreate: (name) => {
        const newId = createFeature(name);
        filesCtx.setFile(SCENARIOS_PATH, (prev) => setScenarioFeature(prev, scn.id, newId));
      },
    };
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
        <span aria-current="page">{featureId}</span>
      </nav>

      <main className="mockup-canvas canvas-max" data-testid="mockup-canvas">
        <div className="page-header">
          <div>
            <span className="section-label">02—Product / Features / {featureId}</span>
            <h1>{featureId}</h1>
            <p className="page-header__meta">
              {feature ? feature.description : "(no matching entry in features.yaml)"}
            </p>
          </div>
        </div>

        <h2 className="subsection">User stories</h2>
        <div className="row-list" data-testid="feature-stories-list">
          {stories.map((s) => (
            <div className="list-row" key={s.id} data-testid="feature-story-row">
              <div className="list-row__primary">
                <Link href={`/story/${storySlugFromId(s.id)}`} className="list-row__title">
                  {s.id} — {s.title}
                </Link>
              </div>
              <div className="list-row__side">
                <FeaturePicker
                  features={features}
                  value={featureId}
                  testId={`feature-story-${storySlugFromId(s.id)}-picker`}
                  {...reassignStory(s)}
                />
              </div>
            </div>
          ))}
          {stories.length === 0 && (
            <div className="feature-agg-empty">No stories tagged with this feature yet.</div>
          )}
        </div>

        <h2 className="subsection">Requirements</h2>
        <div className="row-list" data-testid="feature-scenarios-list">
          {scenarios.map((s) => (
            <div className="list-row" key={s.id} data-testid="feature-scenario-row">
              <div className="list-row__primary">
                <Link
                  href="/requirements"
                  className="list-row__title"
                  onClick={() => filesCtx.requestJump(scnAnchorId(s.id))}
                >
                  {s.id} — {s.description}
                </Link>
              </div>
              <div className="list-row__side">
                <FeaturePicker
                  features={features}
                  value={featureId}
                  testId={`feature-scenario-${s.id}-picker`}
                  {...reassignScenario(s)}
                />
              </div>
            </div>
          ))}
          {scenarios.length === 0 && (
            <div className="feature-agg-empty">No requirements tagged with this feature yet.</div>
          )}
        </div>

        {/* Every /feature/<slug> page shows the SAME unaligned set — a
            scenario like INT-901 has no feature reference at all yet, so
            it can't "belong" to any one feature page; retrospective
            tagging happens by picking a feature for it from HERE, on
            whichever feature's page you happen to be looking at. */}
        <h2 className="subsection">Unaligned requirements</h2>
        <div className="row-list" data-testid="feature-unaligned-scenarios-list">
          {unalignedScenarios.map((s) => (
            <div className="list-row" key={s.id} data-testid="feature-unaligned-scenario-row">
              <div className="list-row__primary">
                <Link
                  href="/requirements"
                  className="list-row__title"
                  onClick={() => filesCtx.requestJump(scnAnchorId(s.id))}
                >
                  {s.id} — {s.description}
                </Link>
              </div>
              <div className="list-row__side">
                <FeaturePicker
                  features={features}
                  value={null}
                  testId={`feature-unaligned-scenario-${s.id}-picker`}
                  {...reassignScenario(s)}
                />
              </div>
            </div>
          ))}
          {unalignedScenarios.length === 0 && (
            <div className="feature-agg-empty">Every scenario is aligned to a feature.</div>
          )}
        </div>
      </main>
    </>
  );
}

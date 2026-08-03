"use client";

import { useMemo } from "react";
import { useFiles, STORIES_DIR, SCENARIOS_PATH, FEATURES_PATH, slug, topLevelIndex, blockAfter } from "./FilesStore";

// Every Product file page only ever needs ONE "top of file" scroll target
// (unlike Architecture, nothing here jumps to a sub-line WITHIN a page —
// story/epic/PRD pages are single-purpose; only /requirements has several
// scenario rows on one page). Reused across prd/epic/each story page since
// they're never mounted at the same time as each other.
export const FILE_TOP_ID = "file-top";
export function scnAnchorId(id) {
  return `scn-${id}`;
}

export function storySlugFromId(id) {
  return id.replace(".", "-");
}
export function storyIdFromSlug(s) {
  return s.replace("-", ".");
}

// ── story files: derive the Stories outline directly from the story FILES
// themselves (paths in the store, ids/titles read from each file's own
// `story:` block) — no epic/index file involved. Any file living under
// STORIES_DIR is a story, full stop; adding one (by hand or via chat) makes
// it show up here automatically.
export function parseStoryFileMeta(storyYaml) {
  const idMatch = storyYaml.match(/\bid:\s*"?([\d.]+)"?/);
  const titleMatch = storyYaml.match(/\btitle:\s*"?([^"\n]+)"?/);
  return {
    id: idMatch ? idMatch[1].trim() : null,
    title: titleMatch ? titleMatch[1].trim() : "",
  };
}

// The feature TAG a story carries — read straight off its own `feature:`
// line (no feature list embedded anywhere; features.yaml only holds
// id+description, the reference lives here). null when untagged.
export function parseStoryFeature(storyYaml) {
  const m = storyYaml.match(/^\s*feature:\s*"?([\w.-]+)"?\s*$/m);
  return m ? m[1].trim() : null;
}

// Rewrites (or inserts, if absent) the `feature:` line inside a single
// story file's `story:` block — used by the FeaturePicker mounted on the
// story page and on each story row of a /feature/<slug> aggregation page.
export function setStoryFeature(storyYaml, featureId) {
  if (/^\s*feature:\s*.*$/m.test(storyYaml)) {
    return storyYaml.replace(/^(\s*feature:\s*).*$/m, `$1${featureId}`);
  }
  // No existing feature: line — insert right after title: (falls back to
  // right after id: if a story somehow has no title line).
  if (/^(\s*)title:.*$/m.test(storyYaml)) {
    return storyYaml.replace(/^(\s*)(title:.*)$/m, (whole, indent, rest) => `${indent}${rest}\n${indent}feature: ${featureId}`);
  }
  return storyYaml.replace(/^(\s*)(id:.*)$/m, (whole, indent, rest) => `${indent}${rest}\n${indent}feature: ${featureId}`);
}

export function deriveStories(files) {
  return Object.keys(files)
    .filter((path) => path.startsWith(STORIES_DIR) && path.endsWith(".yaml"))
    .map((path) => {
      const { id, title } = parseStoryFileMeta(files[path] || "");
      const feature = parseStoryFeature(files[path] || "");
      return { id, title, file: path, feature };
    })
    .filter((s) => s.id)
    .sort((a, b) => {
      const [aMaj, aMin] = a.id.split(".").map(Number);
      const [bMaj, bMin] = b.id.split(".").map(Number);
      return aMaj - bMaj || aMin - bMin;
    });
}

// ── scenarios.yaml: scenario ids (2-space map keys under scenarios:), each
// with its description + feature tag (both read straight from the same
// per-scenario block; feature is null for a scenario not yet aligned —
// e.g. INT-901 — the "retrospective alignment pending" gap). ──
export function parseScenariosOutline(scenariosYaml) {
  const lines = scenariosYaml.split("\n");
  const scenariosIdx = topLevelIndex(lines, "scenarios");
  const scenarios = [];
  let current = null;
  for (const { line, index } of blockAfter(lines, scenariosIdx)) {
    const idMatch = line.match(/^ {2}([A-Za-z0-9-]+):\s*$/);
    if (idMatch) {
      current = { id: idMatch[1], line: index, anchorId: scnAnchorId(idMatch[1]), description: "", feature: null };
      scenarios.push(current);
      continue;
    }
    if (!current) continue;
    const descMatch = line.match(/^\s*description:\s*"?([^"\n]+)"?/);
    if (descMatch) current.description = descMatch[1].trim();
    const featMatch = line.match(/^\s*feature:\s*"?([\w.-]+)"?/);
    if (featMatch) current.feature = featMatch[1].trim();
  }
  return { scenariosHeaderLine: scenariosIdx, scenarios };
}

// Rewrites (or inserts, if absent) ONE scenario's `feature:` line inside
// scenarios.yaml — the retro-tagging move (e.g. giving INT-901 its first
// feature from a /feature/<slug> aggregation page).
export function setScenarioFeature(scenariosYaml, scenarioId, featureId) {
  const lines = scenariosYaml.split("\n");
  const startIdx = lines.findIndex((l) => new RegExp(`^ {2}${scenarioId}:\\s*$`).test(l));
  if (startIdx === -1) return scenariosYaml;

  let endIdx = lines.length;
  for (let i = startIdx + 1; i < lines.length; i++) {
    const l = lines[i];
    if (l.trim() === "") continue;
    const indent = l.match(/^(\s*)/)[1].length;
    if (indent <= 2) {
      endIdx = i;
      break;
    }
  }

  for (let i = startIdx + 1; i < endIdx; i++) {
    if (/^\s*feature:\s*.*$/.test(lines[i])) {
      lines[i] = lines[i].replace(/^(\s*feature:\s*).*$/, `$1${featureId}`);
      return lines.join("\n");
    }
  }
  lines.splice(startIdx + 1, 0, `    feature: ${featureId}`);
  return lines.join("\n");
}

// ── features.yaml: MINIMAL by design — every feature is only id/slug +
// description (steering correction: no name field, and never any
// embedded story/requirement lists — those are derived live below via
// useFeatureAggregate from the STORY/SCENARIO side's `feature:` refs). ──
export function parseFeaturesOutline(featuresYaml) {
  const lines = featuresYaml.split("\n");
  const idx = topLevelIndex(lines, "features");
  const features = [];
  let current = null;
  for (const { line, index } of blockAfter(lines, idx)) {
    const idMatch = line.match(/-\s*id:\s*"?([\w.-]+)"?/);
    if (idMatch) {
      current = { id: idMatch[1].trim(), line: index, description: "" };
      features.push(current);
      continue;
    }
    if (!current) continue;
    const descMatch = line.match(/^\s*description:\s*"?([^"\n]+)"?/);
    if (descMatch) current.description = descMatch[1].trim();
  }
  return { headerLine: idx, features };
}

// Appends a brand-new minimal feature stub (id/slug + a placeholder
// description) — used by every FeaturePicker's "+ Create" option, whether
// invoked from the new-story form, a story page's Feature strip, or a
// /feature/<slug> aggregation row.
export function appendFeatureStub(featuresYaml, rawName) {
  const id = slug(rawName) || `feature-${Date.now()}`;
  const entry = `  - id: ${id}\n    description: "TODO: describe"\n`;
  const updatedFeatures = `${featuresYaml.replace(/\n+$/, "")}\n${entry}`;
  return { updatedFeatures, id };
}

export function useProductOutline() {
  const { files } = useFiles();
  const scenariosYaml = files[SCENARIOS_PATH] || "";
  const featuresYaml = files[FEATURES_PATH] || "";
  return useMemo(
    () => ({
      stories: deriveStories(files),
      scenariosOutline: parseScenariosOutline(scenariosYaml),
      featuresOutline: parseFeaturesOutline(featuresYaml),
    }),
    [files, scenariosYaml, featuresYaml]
  );
}

// ── /feature/<slug> aggregation — NOT a file, a live derived view: every
// story and every scenario whose `feature:` reference matches this id,
// read straight off the current store (so reassigning either one via a
// FeaturePicker re-buckets it here immediately). Also surfaces every
// scenario with NO feature reference at all (e.g. INT-901) as
// "unaligned" — every /feature/<slug> page lists the same unaligned set,
// each with the SAME FeaturePicker control, so retrospective tagging can
// happen from any feature's view (steering: "retrospective tagging of
// untagged scenarios ... can be done from the feature view"). ──
export function useFeatureAggregate(featureId) {
  const { files } = useFiles();
  const scenariosYaml = files[SCENARIOS_PATH] || "";
  return useMemo(() => {
    const allScenarios = parseScenariosOutline(scenariosYaml).scenarios;
    const stories = deriveStories(files).filter((s) => s.feature === featureId);
    const scenarios = allScenarios.filter((s) => s.feature === featureId);
    const unalignedScenarios = allScenarios.filter((s) => !s.feature);
    return { stories, scenarios, unalignedScenarios };
  }, [files, scenariosYaml, featureId]);
}

// Shared "next id" numbering (major from the first existing id, minor =
// max + 1) — used by both the chat-scripted story stub and the new-story
// form so the two flows never drift apart.
function nextStoryId(existingIds) {
  const ids = existingIds.map((id) => id.split(".").map(Number));
  const major = ids.length ? ids[0][0] : 60;
  const nextMinor = ids.length ? Math.max(...ids.map(([, mi]) => mi)) + 1 : 1;
  return `${major}.${nextMinor}`;
}

function storyYamlFor(newId, title, featureId, iWant) {
  return `story:
    id: "${newId}"
    title: ${title}
    feature: ${featureId}
    as_a: Claude User
    i_want: ${iWant}
    so_that: (not captured yet — refine later)
    status: PLANNED
    acceptance_criteria:
        - id: AC-1
          description: (stub — no detail yet)
`;
}

// ── scripted "LLM edit" #2 (/product or a story page): add a plausible new
// story ── No epic involved: this just picks the next id off the EXISTING
// story ids (read from the story files themselves) and returns a brand-new
// story file to seed into the FilesStore at a fresh path. Tags the new
// story with `defaultFeatureId` (first feature in features.yaml, chosen by
// the caller) — fine for a scripted mock, unlike the real new-story FORM
// which makes the user pick via a FeaturePicker.
export function appendStoryStub(existingIds, message, defaultFeatureId) {
  const newId = nextStoryId(existingIds);

  const titleGuess = message.replace(/^(please\s+)?add\s+(a\s+)?story\s*(about|for|on)?\s*/i, "").trim();
  const title = titleGuess ? titleGuess.charAt(0).toUpperCase() + titleGuess.slice(1) : `New story ${newId}`;
  const fileSlug = slug(title) || `story-${newId}`;
  const filePath = `docs/prd/stories/${newId}-${fileSlug}.yaml`;

  const storyStub = storyYamlFor(newId, title, defaultFeatureId || "(none)", message);

  return { filePath, storyStub, newId, title };
}

// ── new-story FORM flow: user-supplied title + a feature id already
// resolved by the FeaturePicker (existing pick or a freshly created one).
export function createStoryFile(existingIds, title, featureId) {
  const newId = nextStoryId(existingIds);
  const fileSlug = slug(title) || `story-${newId}`;
  const filePath = `docs/prd/stories/${newId}-${fileSlug}.yaml`;
  const storyStub = storyYamlFor(newId, title, featureId, "(not captured yet — added via the new-story form)");
  return { filePath, storyStub, newId };
}

// ── scripted "LLM edit" #3 (/requirements): add a plausible new scenario ──
export function appendScenarioStub(scenariosYaml, message) {
  const nums = [...scenariosYaml.matchAll(/^\s*E2E-(\d+):/gm)].map((m) => Number(m[1]));
  const nextNum = nums.length ? Math.max(...nums) + 1 : 604;
  const newId = `E2E-${nextNum}`;

  const descriptionGuess = message
    .replace(/^(please\s+)?add\s+(a\s+)?scenario\s*(about|for|on)?\s*/i, "")
    .trim();
  const description = descriptionGuess || "New scenario — added via chat, no detail yet";

  const entry = `  ${newId}:\n    description: "${description}"\n    service: "mcp-service"\n    user_stories: []\n`;
  const updatedScenarios = `${scenariosYaml.replace(/\n+$/, "")}\n${entry}`;

  return { updatedScenarios, newId, description };
}

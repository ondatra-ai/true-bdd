/**
 * Client-side derivation: outline entries + per-service details, derived from
 * the LIVE buffer using the real YAML parser (never a line-heuristic regex —
 * plan Challenges). Pure functions over document text; callers pick which
 * text to derive from (the live buffer, or `lastValidContent` on a parse
 * error — P10b).
 */

import { parseDocument } from "yaml";

import { safeParseYAML } from "./yaml-utils";

export interface ServiceInfo {
  name: string;
  type: string;
  isCustom: boolean;
  technologies: string[];
  dockerfile?: string;
  composeRef?: string;
  endpoints: Array<{ method: string; path: string; summary?: string }>;
  connection: Record<string, string | number>;
}

export interface TermInfo {
  name: string;
  description?: string;
}

export interface ArchitectureModel {
  services: ServiceInfo[];
  terms: TermInfo[];
  composeFile: string | null;
}

const CONNECTION_EXCLUDE = new Set([
  "type",
  "dockerfile",
  "compose_ref",
  "technologies",
  "endpoints",
  "dependencies",
  "quality_gate",
  "path",
]);

interface RawArchitecture {
  services?: Record<string, Record<string, unknown>>;
  terms?: Array<Record<string, unknown>>;
  docker?: { compose_file?: unknown };
}

/** Derives the architecture outline model from the buffer text (empty model
 * when the buffer does not parse — P10b keeps the last-valid outline). */
export function deriveArchitecture(text: string): ArchitectureModel {
  const parsed = safeParseYAML<RawArchitecture>(text);
  if (parsed === null || typeof parsed !== "object") {
    return { services: [], terms: [], composeFile: null };
  }

  const servicesRaw = parsed.services ?? {};
  const services: ServiceInfo[] = Object.keys(servicesRaw).map((name) => {
    const svc = servicesRaw[name] ?? {};
    const isCustom = svc.type === "custom";
    const connection: Record<string, string | number> = {};
    if (!isCustom) {
      for (const [key, value] of Object.entries(svc)) {
        if (CONNECTION_EXCLUDE.has(key)) continue;
        if (typeof value === "string" || typeof value === "number") {
          connection[key] = value;
        }
      }
    }

    return {
      name,
      type: String(svc.type ?? ""),
      isCustom,
      technologies: Array.isArray(svc.technologies) ? svc.technologies.map(String) : [],
      dockerfile: typeof svc.dockerfile === "string" ? svc.dockerfile : undefined,
      composeRef: typeof svc.compose_ref === "string" ? svc.compose_ref : undefined,
      endpoints: Array.isArray(svc.endpoints)
        ? (svc.endpoints as Array<Record<string, unknown>>).map((ep) => ({
            method: String(ep.method ?? ""),
            path: String(ep.path ?? ""),
            summary: typeof ep.summary === "string" ? ep.summary : undefined,
          }))
        : [],
      connection,
    };
  });

  const terms: TermInfo[] = Array.isArray(parsed.terms)
    ? parsed.terms.map((t) => ({
        name: String(t.name ?? ""),
        description: typeof t.description === "string" ? t.description : undefined,
      }))
    : [];

  const composeFile = typeof parsed.docker?.compose_file === "string" ? parsed.docker.compose_file : null;

  return { services, terms, composeFile };
}

export interface FeatureInfo {
  id: string;
  description: string;
}

interface RawFeatures {
  features?: Array<Record<string, unknown>>;
}

export function deriveFeatures(text: string): FeatureInfo[] {
  const parsed = safeParseYAML<RawFeatures>(text);
  const list = Array.isArray(parsed?.features) ? parsed.features : [];

  return list.map((f) => ({ id: String(f.id ?? ""), description: String(f.description ?? "") }));
}

export interface StoryInfo {
  id: string;
  title?: string;
  feature?: string;
}

interface RawStory {
  story?: { id?: unknown; title?: unknown; feature?: unknown };
}

/** Derives a single story file's model, or null when it does not parse /
 * carry a `story:` root (P10b: keep the caller's last-valid model). */
export function deriveStory(text: string): StoryInfo | null {
  const parsed = safeParseYAML<RawStory>(text);
  const story = parsed?.story;
  if (story === undefined || typeof story !== "object" || story === null) {
    return null;
  }

  return {
    id: String(story.id ?? ""),
    title: typeof story.title === "string" ? story.title : undefined,
    feature: typeof story.feature === "string" ? story.feature : undefined,
  };
}

export interface ScenarioInfo {
  id: string;
  description?: string;
  feature?: string;
}

interface RawScenarios {
  scenarios?: Record<string, Record<string, unknown>>;
}

export function deriveScenarios(text: string): ScenarioInfo[] {
  const parsed = safeParseYAML<RawScenarios>(text);
  const map = parsed?.scenarios;
  if (map === undefined || typeof map !== "object" || map === null) {
    return [];
  }

  return Object.keys(map).map((id) => ({
    id,
    description: typeof map[id]?.description === "string" ? (map[id].description as string) : undefined,
    feature: typeof map[id]?.feature === "string" ? (map[id].feature as string) : undefined,
  }));
}

/**
 * Sets a nested field's value while preserving the rest of the document's
 * formatting as closely as the `yaml` package's mutable CST allows (used for
 * the picker's "change feature" writes — P21/P23 — and retro-tagging — P24).
 * Falls back to the ORIGINAL content on any parse failure (never corrupts a
 * document it cannot understand).
 */
export function withFieldSet(content: string, path: Array<string | number>, value: unknown): string {
  try {
    const doc = parseDocument(content);
    if (doc.errors.length > 0) {
      return content;
    }
    doc.setIn(path, value);

    return doc.toString();
  } catch {
    return content;
  }
}

/**
 * Appends an id+description-ONLY stub to features.yaml's `features:`
 * sequence (P22 inline "+ Create") — never any other key.
 */
export function appendFeatureStub(content: string, id: string, description: string): string {
  try {
    const doc = parseDocument(content);
    if (doc.errors.length > 0) {
      return content;
    }
    doc.addIn(["features"], { id, description });

    return doc.toString();
  } catch {
    return content;
  }
}

/** Slugifies free text into a feature id (P22 inline "+ Create"). */
export function slugify(text: string): string {
  const slug = text
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/(^-+|-+$)/g, "");

  return slug.length > 0 ? slug : `feature-${Date.now()}`;
}

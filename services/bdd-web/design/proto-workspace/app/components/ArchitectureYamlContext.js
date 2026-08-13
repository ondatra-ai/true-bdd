"use client";

// Architecture-specific outline parsing + scripted chat edits, now sitting
// on top of the generalized FilesStore (components/FilesStore.js) instead
// of owning its own Context — kept as a thin shim so every existing call
// site (sections.js, app/(workspace)/architecture/page.js, ChatDialog.js)
// keeps working unchanged: same function names, same {yaml, setYaml,
// jumpRequest, requestJump} shape.
import { ARCHITECTURE_PATH, useFile, topLevelIndex, blockAfter, slug } from "./FilesStore";

export function useArchitectureYaml() {
  const { content, setContent, jumpRequest, requestJump } = useFile(ARCHITECTURE_PATH);
  return { yaml: content, setYaml: setContent, jumpRequest, requestJump };
}

const REDIS_BLOCK = `  redis:
    type: redis
    image: redis:7-alpine
    port: 6379
    volume: redisdata:/data
    compose_ref: docker-compose.yml#services.redis
`;

// Scripted "LLM edit" #1: insert a redis service block right before the
// terms: key (or just append if the shape changed under editing).
export function appendRedisService(yaml) {
  if (yaml.includes("\n  redis:")) return yaml; // already added once, don't pile up
  if (yaml.includes("\nterms:")) {
    return yaml.replace("\nterms:", `\n${REDIS_BLOCK}terms:`);
  }
  return `${yaml}\n${REDIS_BLOCK}`;
}

// Re-exported so ChatDialog.js's existing import keeps working (generic —
// lives in FilesStore.js now, works on any file).
export { appendChatComment } from "./FilesStore";

// ── sidebar-outline parsing ──────────────────────────────────────────────
// The sidebar's Services:/Terms:/Docker: rows are a FIXED, mandatory schema
// outline (hardcoded — see ArchitectureTree in sections.js), but the
// ENTRIES inside them are derived live from whatever the YAML currently
// says. This is a small structural reader tuned to our synthetic file's
// shape (2-space-indented service keys, `- name:` term entries, a single
// compose_file: path) — not a general YAML parser.
export const ARCH_TOP_ID = "arch-top";
export const ARCH_SERVICES_ID = "arch-services";
export const ARCH_TERMS_ID = "arch-terms";
export const ARCH_DOCKER_ID = "arch-docker";
export const ARCH_DOCKER_COMPOSE_ID = "arch-docker-compose";
export function svcAnchorId(key) {
  return `arch-svc-${slug(key)}`;
}
export function termAnchorId(name) {
  return `arch-term-${slug(name)}`;
}

export function parseArchitectureOutline(yaml) {
  const lines = yaml.split("\n");

  const servicesIdx = topLevelIndex(lines, "services");
  const termsIdx = topLevelIndex(lines, "terms");
  const dockerIdx = topLevelIndex(lines, "docker");

  const services = [];
  for (const { line, index } of blockAfter(lines, servicesIdx)) {
    const m = line.match(/^ {2}([a-zA-Z0-9_-]+):\s*$/);
    if (m) services.push({ key: m[1], line: index, id: svcAnchorId(m[1]) });
  }

  const terms = [];
  for (const { line, index } of blockAfter(lines, termsIdx)) {
    const m = line.match(/- name:\s*(.+?)\s*$/);
    if (m) terms.push({ name: m[1], line: index, id: termAnchorId(m[1]) });
  }

  let dockerCompose = null;
  for (const { line, index } of blockAfter(lines, dockerIdx)) {
    const m = line.match(/compose_file:\s*(.+?)\s*$/);
    if (m) {
      dockerCompose = { path: m[1], line: index };
      break;
    }
  }

  return {
    lineCount: lines.length,
    sectionLines: { services: servicesIdx, terms: termsIdx, docker: dockerIdx },
    services,
    terms,
    dockerCompose,
  };
}

// Every (id, line) marker the /architecture page needs to render as an
// invisible scroll target, derived from the same parse the sidebar uses —
// single source of truth so the two never drift apart.
export function outlineAnchors(outline) {
  const anchors = [{ id: ARCH_TOP_ID, line: 0 }];
  if (outline.sectionLines.services !== -1) {
    anchors.push({ id: ARCH_SERVICES_ID, line: outline.sectionLines.services });
  }
  for (const svc of outline.services) {
    anchors.push({ id: svc.id, line: svc.line });
  }
  if (outline.sectionLines.terms !== -1) {
    anchors.push({ id: ARCH_TERMS_ID, line: outline.sectionLines.terms });
  }
  for (const term of outline.terms) {
    anchors.push({ id: term.id, line: term.line });
  }
  if (outline.sectionLines.docker !== -1) {
    anchors.push({ id: ARCH_DOCKER_ID, line: outline.sectionLines.docker });
  }
  if (outline.dockerCompose) {
    anchors.push({ id: ARCH_DOCKER_COMPOSE_ID, line: outline.dockerCompose.line });
  }
  return anchors;
}

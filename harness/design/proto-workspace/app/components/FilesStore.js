"use client";

import { createContext, useContext, useState } from "react";

// Generalized "files-as-source" store: every file the prototype treats as a
// GitHub-file-view page (Architecture's architecture.yaml + Product's five
// files) lives here, keyed by its conventional docs/ path, in ONE shared
// React Context — per steering: "generalize the single-YAML context into a
// files store keyed by path... one shared provider." In-memory only.

export const ARCHITECTURE_PATH = "docs/architecture/architecture.yaml";
export const PRD_PATH = "docs/prd/prd.yaml";
export const SCENARIOS_PATH = "docs/scenarios.yaml";
export const FEATURES_PATH = "docs/prd/features.yaml";
// No epic file/concept anymore — stories are flat, direct children of
// Product, and the Stories outline derives from the story FILES themselves
// (see ProductFiles.js's deriveStories) rather than any epic's stories: list.
// FEATURES are the new alignment tag: features.yaml is DELIBERATELY minimal
// (just id/slug + description, no name, no embedded story/requirement
// lists) — stories carry a `feature: <id>` field pointing into it, and
// scenarios carry the same field, added retrospectively. The alignment
// lives ONLY on the referencing side; see ProductFiles.js's
// parseFeaturesOutline / useFeatureAggregate for the derived view, and
// setStoryFeature / setScenarioFeature for the picker-driven rewrites.
export const STORIES_DIR = "docs/prd/stories/";
export const STORY_60_1_PATH = "docs/prd/stories/60.1-summary-length-preference.yaml";
export const STORY_60_2_PATH = "docs/prd/stories/60.2-summary-shared-docs.yaml";
export const STORY_60_3_PATH = "docs/prd/stories/60.3-summary-error-messages.yaml";

// ── shared line-parsing utilities ──────────────────────────────────────
// Small structural readers tuned to OUR synthetic files' shape (top-level
// keys at 0 indent, list/map entries at 2-space indent) — not a general
// YAML parser. Used by both the Architecture outline (ArchitectureYamlContext.js)
// and the Product outline (ProductFiles.js) so the two never drift apart.
export function slug(s) {
  return s
    .toLowerCase()
    .trim()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "");
}

export function topLevelIndex(lines, key) {
  return lines.findIndex((l) => l.trim() === `${key}:` && /^\S/.test(l));
}

// Every non-blank line strictly after `startIdx` up to (not including) the
// next top-level (unindented) key.
export function blockAfter(lines, startIdx) {
  if (startIdx === -1) return [];
  const out = [];
  for (let i = startIdx + 1; i < lines.length; i++) {
    const line = lines[i];
    if (line.trim() === "") continue;
    if (/^\S/.test(line)) break; // next top-level key — block ends
    out.push({ line, index: i });
  }
  return out;
}

// Generic scripted "chat edit" fallback that works on ANY file — appends a
// trailing comment recording the request. Enough to demonstrate "the chat
// edits the file" without full natural-language-to-YAML authoring.
export function appendChatComment(content, message) {
  return `${content}\n# chat: ${message}\n`;
}

// ── seed content ────────────────────────────────────────────────────────
const ARCHITECTURE_YAML = `version: 1
services:
  mcp-service:
    type: custom
    path: services/mcp
    dockerfile: services/mcp/Dockerfile
    technologies:
      - go
      - net/http
    dependencies: none
    quality_gate:
      integration_tests:
        path: tests/integration
        framework: jest
        config: tests/jest.config.ts
    endpoints:
      - method: POST
        path: /mcp
        summary: JSON-RPC endpoint for tool calls
      - method: GET
        path: /health
        summary: Liveness probe
      - method: GET
        path: /mcp/tools
        summary: List available tools
  postgres:
    type: postgres
    image: postgres:16-alpine
    port: 5432
    volume: pgdata:/var/lib/postgresql/data
    compose_ref: docker-compose.yml#services.postgres
terms:
  - name: Vocabulary
    description: Shared BDD glossary terms referenced by scenarios and specs.
docker:
  compose_file: docker-compose.yml
  description: >
    Prototype stub — not generated from this spec (yet, undecided). Image-based
    services reference their own section here via compose_ref.
`;

// Mirrors the real fixtures' docs/prd/prd.yaml shape (tests/bdd-cli/fixtures/
// us-create-happy-path/input/docs/prd/prd.yaml) — same title/personas the
// mockup's /prd-overview page already shows. The epic layer was removed from
// the model, so the PRD indexes story files directly.
const PRD_YAML = `title: "MCP Google Docs Editor (harness AI fixture stub)"
summary: "Synthetic PRD for harness E2E AI fixtures — deliberately minimal."

personas:
  - name: Claude User
    definition: "An individual who uses Claude Desktop or Claude Web App and wants to edit their Google Docs through natural language commands"
    primary_use_case: "Document editing through conversational AI interface"
    characteristics:
      - "Non-technical users focused on content creation and editing"
      - "Values seamless integration between Claude AI and Google Docs"
      - "Expects reliable, fast document operations with clear error messaging"
  - name: Developer/Maintainer
    definition: "Technical personnel responsible for building, deploying, monitoring, and maintaining the MCP Google Docs Editor system"
    primary_use_case: "System development, deployment, and operational support"
    characteristics:
      - "Technical expertise in Go, Railway CLI, MCP protocol, and Google APIs"
      - "Focused on system reliability, security, and performance"

stories:
  - id: "60.1"
    file: docs/prd/stories/60.1-summary-length-preference.yaml
  - id: "60.2"
    file: docs/prd/stories/60.2-summary-shared-docs.yaml
  - id: "60.3"
    file: docs/prd/stories/60.3-summary-error-messages.yaml
`;

// Minimal by design (steering correction): just id/slug + description, no
// name field, no embedded stories/requirements lists — those are derived
// live (see ProductFiles.useFeatureAggregate) from the OTHER side's
// `feature:` references. "inventory-view" starts with zero references, to
// demonstrate the empty aggregation state / the new-feature flow.
const FEATURES_YAML = `features:
  - id: summaries
    description: "Generating and tuning Claude's Google Doc summaries — length preferences, defaults, and coverage for shared docs."
  - id: error-handling
    description: "Clear failure messaging when a summary can't be produced normally — timeouts, oversized documents, and other edge cases."
  - id: inventory-view
    description: "The apply-state inventory view (missing / partial / full) surfaced on the workspace overview — no stories or requirements aligned yet."
`;

// 60.2's content is lifted verbatim from the existing story-detail.html
// mockup's own "raw story file" preview (it already showed this exact
// story: block). 60.1 and 60.3 are invented, plausibly splitting the real
// fixture story 96.1's four ACs across our three stories: 60.1 gets a
// fresh "length preference" angle (not in the fixture); 60.3's two ACs are
// the fixture's timeout (AC-3) and oversized-doc (AC-4) ACs, matching what
// scenarios E2E-602/E2E-603 already describe in scenarios.html.
const STORY_60_1_YAML = `story:
    id: "60.1"
    title: Summary Length Preference
    feature: summaries
    as_a: Claude User
    i_want: to tell Claude how long or short my summary should be
    so_that: I get exactly as much detail as I need for the doc at hand
    status: PLANNED
    acceptance_criteria:
        - id: AC-1
          description: Claude must limit the summary to 75 words or fewer when the Claude User asks for a "short" summary.
          steps:
            - given:
                - a Google Doc is shared with the Claude User's account
            - when:
                - the Claude User asks Claude for a short summary of the document
            - then:
                - Claude displays a summary of 75 words or fewer in the conversation
        - id: AC-2
          description: Claude must allow up to 300 words when the Claude User asks for a "detailed" summary.
          steps:
            - given:
                - a Google Doc is shared with the Claude User's account
            - when:
                - the Claude User asks Claude for a detailed summary of the document
            - then:
                - Claude displays a summary of up to 300 words in the conversation
        - id: AC-3
          description: Claude must default to 150 words or fewer when the Claude User states no length preference.
          steps:
            - given:
                - a Google Doc is shared with the Claude User's account
            - when:
                - the Claude User asks Claude to summarize the document without stating a length
            - then:
                - Claude displays a summary of 150 words or fewer in the conversation
`;

const STORY_60_2_YAML = `story:
    id: "60.2"
    title: Summary For Shared Docs
    feature: summaries
    as_a: Claude User
    i_want: Claude to give me a short summary of a shared Google Doc when I ask for one
    so_that: I can decide whether the document needs my attention without reading it in full
    status: PLANNED
    acceptance_criteria:
        - id: AC-1
          description: Claude must display a summary of 150 words or fewer in the conversation when the Claude User asks for a summary of a shared Google Doc.
          steps:
            - given:
                - a Google Doc with at least 500 words of body text is shared with the Claude User's account
            - when:
                - the Claude User asks Claude to summarize the document
            - then:
                - Claude displays a summary of 150 words or fewer in the conversation
                - and: the summary names the document title
        - id: AC-2
          description: Claude must display the message 'This document is not shared with your account' when the Claude User asks for a summary of a document they cannot access.
          steps:
            - given:
                - a Google Doc exists that is not shared with the Claude User's account
            - when:
                - the Claude User asks Claude to summarize that document
            - then:
                - Claude displays the message 'This document is not shared with your account'
                - but: Claude does not display any content from the document
`;

const STORY_60_3_YAML = `story:
    id: "60.3"
    title: Summary Error Messages
    feature: error-handling
    as_a: Claude User
    i_want: Claude to tell me clearly when it could not produce a normal summary
    so_that: I know whether to retry, wait, or read the document myself
    status: PLANNED
    acceptance_criteria:
        - id: AC-1
          description: Claude must display the message 'The summary timed out - please try again' when no summary is produced within 30 seconds.
          steps:
            - given:
                - a Google Doc is shared with the Claude User's account
                - the summary backend does not respond within 30 seconds
            - when:
                - the Claude User asks Claude to summarize the document
            - then:
                - Claude displays the message 'The summary timed out - please try again'
        - id: AC-2
          description: Claude must state that only the first 10000 words were summarized when the document contains more than 10000 words.
          steps:
            - given:
                - a Google Doc with more than 10000 words is shared with the Claude User's account
            - when:
                - the Claude User asks Claude to summarize the document
            - then:
                - Claude displays a summary of the first 10000 words
                - and: Claude states that only the first 10000 words were summarized
`;

// Mirrors tests/bdd-cli/fixtures/build-tests-fix-happy-path's registry
// shape (metadata + scenarios: { ID: {description, service, user_stories}}).
// The four scenarios the mockup already shows (scenarios.html): E2E-601/602
// linked to the story that covers them; INT-901 is an integration scenario
// with no covering user story (judgment call — see report).
const SCENARIOS_YAML = `metadata:
  title: "Requirements Registry (Inventory Spread workspace)"
  version: "1.0"
  description: "Scenario registry — flat by design, no epic/story nesting."

scenarios:
  E2E-601:
    description: "Shared doc summary of 150 words or fewer appears in the conversation"
    service: "mcp-service"
    feature: summaries
    user_stories:
      - story: docs/prd/stories/60.2-summary-shared-docs.yaml
        scenario_id: "60.2-001"
  E2E-602:
    description: "Summary timeout surfaces the timed-out message"
    service: "mcp-service"
    feature: error-handling
    user_stories:
      - story: docs/prd/stories/60.3-summary-error-messages.yaml
        scenario_id: "60.3-001"
  E2E-603:
    description: "Oversized documents state the summarized word range"
    service: "mcp-service"
    feature: error-handling
    user_stories:
      - story: docs/prd/stories/60.3-summary-error-messages.yaml
        scenario_id: "60.3-002"
  INT-901:
    description: "MCP /mcp endpoint responds 200 to a valid initialize POST"
    service: "mcp-service"
    user_stories: []
`;
// INT-901 deliberately ships with NO feature: field — the "retrospective
// alignment pending" gap the user asked to be able to probe (fix it via
// the /feature/<slug> aggregation page's inline picker, see feature/[id]).

const INITIAL_FILES = {
  [ARCHITECTURE_PATH]: ARCHITECTURE_YAML,
  [PRD_PATH]: PRD_YAML,
  [FEATURES_PATH]: FEATURES_YAML,
  [STORY_60_1_PATH]: STORY_60_1_YAML,
  [STORY_60_2_PATH]: STORY_60_2_YAML,
  [STORY_60_3_PATH]: STORY_60_3_YAML,
  [SCENARIOS_PATH]: SCENARIOS_YAML,
};

// ── context/provider ────────────────────────────────────────────────────
const FilesContext = createContext(null);

export function FilesProvider({ children }) {
  const [files, setFiles] = useState(INITIAL_FILES);
  // { id, token } — token always unique so re-requesting the SAME id still
  // re-triggers the jump effect (dependency array needs a real change).
  // Shared across ALL file pages so a cross-page jump (navigate, then
  // scroll once the target page mounts) survives the route change — the
  // provider lives at the workspace layout level and never unmounts.
  const [jumpRequest, setJumpRequest] = useState(null);

  function setFile(path, updater) {
    setFiles((prev) => ({
      ...prev,
      [path]: typeof updater === "function" ? updater(prev[path] ?? "") : updater,
    }));
  }

  function requestJump(id) {
    setJumpRequest({ id, token: `${id}-${Date.now()}-${Math.random()}` });
  }

  return (
    <FilesContext.Provider value={{ files, setFile, jumpRequest, requestJump }}>
      {children}
    </FilesContext.Provider>
  );
}

export function useFiles() {
  return useContext(FilesContext);
}

// Convenience for a single file — mirrors the old useArchitectureYaml()
// shape so existing call sites barely change.
export function useFile(path) {
  const ctx = useFiles();
  const content = ctx.files[path] ?? "";
  const setContent = (updater) => ctx.setFile(path, updater);
  return { content, setContent, jumpRequest: ctx.jumpRequest, requestJump: ctx.requestJump };
}

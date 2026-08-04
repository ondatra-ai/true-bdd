import { describe, expect, it } from "vitest";

import {
  appendFeatureStub,
  deriveArchitecture,
  deriveFeatures,
  deriveScenarios,
  deriveStory,
  slugify,
  withFieldSet,
} from "@/app/lib/workspace/derive";

const ARCH_FIXTURE = `version: 1
services:
  mcp-service:
    type: custom
    dockerfile: services/mcp/Dockerfile
    technologies:
      - go
      - net/http
    endpoints:
      - method: POST
        path: /mcp
        summary: JSON-RPC endpoint
  postgres:
    type: postgres
    image: postgres:16-alpine
    port: 5432
    compose_ref: docker-compose.yml#services.postgres
terms:
  - name: Vocabulary
    description: Shared BDD glossary terms.
docker:
  compose_file: docker-compose.yml
`;

describe("deriveArchitecture", () => {
  it("derives one service entry per service with tech + provenance", () => {
    const model = deriveArchitecture(ARCH_FIXTURE);

    expect(model.services.map((s) => s.name).sort()).toEqual(["mcp-service", "postgres"]);
    expect(model.composeFile).toBe("docker-compose.yml");
    expect(model.terms).toEqual([{ name: "Vocabulary", description: "Shared BDD glossary terms." }]);
  });

  it("puts endpoints on the custom service and connection info on the supporting one (P16)", () => {
    const model = deriveArchitecture(ARCH_FIXTURE);
    const mcp = model.services.find((s) => s.name === "mcp-service")!;
    const postgres = model.services.find((s) => s.name === "postgres")!;

    expect(mcp.isCustom).toBe(true);
    expect(mcp.endpoints).toEqual([{ method: "POST", path: "/mcp", summary: "JSON-RPC endpoint" }]);
    expect(mcp.dockerfile).toBe("services/mcp/Dockerfile");
    expect(mcp.connection).toEqual({});

    expect(postgres.isCustom).toBe(false);
    expect(postgres.endpoints).toEqual([]);
    expect(postgres.connection.port).toBe(5432);
    expect(postgres.composeRef).toBe("docker-compose.yml#services.postgres");
  });

  it("returns an empty model (never throws) on invalid YAML — P10b keeps last-valid", () => {
    expect(deriveArchitecture("services: [")).toEqual({ services: [], terms: [], composeFile: null });
  });
});

describe("deriveFeatures", () => {
  it("derives id + description records", () => {
    const features = deriveFeatures("features:\n  - id: a\n    description: d\n");

    expect(features).toEqual([{ id: "a", description: "d" }]);
  });

  it("returns an empty list when the document has no features", () => {
    expect(deriveFeatures("other: 1\n")).toEqual([]);
  });
});

describe("deriveStory", () => {
  it("derives id/title/feature from a story: root", () => {
    const story = deriveStory('story:\n  id: "60.1"\n  title: Summary\n  feature: summaries\n');

    expect(story).toEqual({ id: "60.1", title: "Summary", feature: "summaries" });
  });

  it("returns null when there is no story: root (e.g. it was overwritten)", () => {
    expect(deriveStory("title: not a story\n")).toBeNull();
  });
});

describe("deriveScenarios", () => {
  it("derives every scenario id with its feature ref", () => {
    const scenarios = deriveScenarios(
      "scenarios:\n  E2E-601:\n    description: d\n    feature: summaries\n  INT-901:\n    description: e\n",
    );

    expect(scenarios).toEqual([
      { id: "E2E-601", description: "d", feature: "summaries" },
      { id: "INT-901", description: "e", feature: undefined },
    ]);
  });
});

describe("withFieldSet", () => {
  it("sets a nested field, preserving the rest of the document", () => {
    const updated = withFieldSet('story:\n  id: "60.2"\n  feature: summaries\n', ["story", "feature"], "error-handling");
    const reparsed = deriveStory(updated);

    expect(reparsed?.feature).toBe("error-handling");
    expect(reparsed?.id).toBe("60.2");
  });

  it("falls back to the original content on a parse failure", () => {
    const broken = "story: [";

    expect(withFieldSet(broken, ["story", "feature"], "x")).toBe(broken);
  });
});

describe("appendFeatureStub", () => {
  it("appends an id+description-ONLY record (P22/P20)", () => {
    const updated = appendFeatureStub("features:\n  - id: a\n    description: d\n", "brand-new", "a fresh feature");
    const features = deriveFeatures(updated);

    expect(features).toContainEqual({ id: "brand-new", description: "a fresh feature" });
    expect(features).toHaveLength(2);
  });
});

describe("slugify", () => {
  it("lowercases and hyphenates free text", () => {
    expect(slugify("Story With A Fresh Feature")).toBe("story-with-a-fresh-feature");
  });

  it("strips punctuation and collapses runs of separators", () => {
    expect(slugify("  Weird!! Input__Text  ")).toBe("weird-input-text");
  });

  it("never returns an empty slug", () => {
    expect(slugify("   ")).not.toBe("");
  });
});

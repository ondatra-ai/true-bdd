import { describe, expect, it } from "vitest";

import { isValidYAML, lineOfMapKey, lineOfSeqItemByField, safeParseYAML } from "@/app/lib/workspace/yaml-utils";

describe("isValidYAML", () => {
  it("accepts well-formed YAML", () => {
    expect(isValidYAML("version: 1\nterms: []\n")).toBe(true);
  });

  it("rejects an unterminated flow sequence (P10a/P10b gate)", () => {
    expect(isValidYAML("services: [")).toBe(false);
  });

  it("accepts an empty document", () => {
    expect(isValidYAML("")).toBe(true);
  });
});

describe("safeParseYAML", () => {
  it("returns the parsed value for valid YAML", () => {
    expect(safeParseYAML<{ a: number }>("a: 1\n")).toEqual({ a: 1 });
  });

  it("returns null (never throws) for invalid YAML", () => {
    expect(safeParseYAML("a: [")).toBeNull();
  });
});

describe("lineOfMapKey", () => {
  const content = "version: 1\nservices:\n  mcp-service:\n    type: custom\n  postgres:\n    type: postgres\nterms:\n  - name: A\n";

  it("finds the 0-based line of a nested map key", () => {
    // "  postgres:" is line index 4 (0-based).
    expect(lineOfMapKey(content, ["services", "postgres"])).toBe(4);
  });

  it("finds a top-level key's line", () => {
    expect(lineOfMapKey(content, ["services", "mcp-service"])).toBe(2);
  });

  it("returns null when the path does not exist", () => {
    expect(lineOfMapKey(content, ["services", "missing"])).toBeNull();
  });

  it("returns null when the document does not parse", () => {
    expect(lineOfMapKey("broken: [", ["services", "mcp-service"])).toBeNull();
  });
});

describe("lineOfSeqItemByField", () => {
  const content = "terms:\n  - name: Vocabulary\n    description: a\n  - name: Scenario\n    description: b\n";

  it("finds the 0-based line of a sequence item matching a field", () => {
    expect(lineOfSeqItemByField(content, ["terms"], "name", "Scenario")).toBe(3);
  });

  it("returns null when no item matches", () => {
    expect(lineOfSeqItemByField(content, ["terms"], "name", "Nope")).toBeNull();
  });
});

/**
 * Component/view-model tables (plan §4.6): every inventory state class —
 * all document chip statuses, epic states + flags, story created states,
 * every story flag, every applied variant (counted x/y + each unknown
 * reason), refined rendering, path-mismatch, and the per-action dispatch
 * identifiers. Pure functions over the scanner's snapshot JSON; no DOM.
 */

import { describe, expect, it } from "vitest";

import {
  appliedCell,
  documentChips,
  epicRow,
  pathMismatch,
  refinedText,
  storyDispatch,
  storyFlags,
  storyRow,
} from "../../app/lib/view-model/inventory";
import type {
  InventoryApplied,
  InventoryEpic,
  InventorySnapshot,
  InventoryStory,
  InventoryStoryFlags,
} from "../../app/lib/view-model/types";

function snapshot(overrides: Partial<InventorySnapshot> = {}): InventorySnapshot {
  return {
    documents: {},
    architecture_path_mismatch: false,
    epics: [],
    ...overrides,
  };
}

const NO_FLAGS: InventoryStoryFlags = {
  duplicate_declared_id: false,
  id_mismatch: false,
  deprecated_format: false,
  no_acs: false,
  empty_internal_id: false,
};

function story(overrides: Partial<InventoryStory> = {}): InventoryStory {
  return {
    create_id: "70.1",
    epic_number: 70,
    position: 1,
    declared_id: "77.5",
    file_id: "88.9",
    created: "one",
    applied: { status: "counted", applied: 1, total: 2 },
    refined: "not_recorded",
    flags: { ...NO_FLAGS },
    ...overrides,
  };
}

describe("documentChips", () => {
  it("renders every document/checklist status class with its data-status", () => {
    const documents = {
      config: "present",
      prd: "missing",
      architecture: "invalid",
      registry: "present_empty",
      "stories-dir": "not_a_dir",
      "epics-dir": "present",
      "checklist-us-create": "present",
      "checklist-us-refine": "invalid",
      "checklist-us-apply": "missing",
      "checklist-build-tests": "present",
      "checklist-build-code": "present",
    };
    const chips = documentChips(snapshot({ documents, document_errors: { architecture: "yaml: line 3: bad" } }));
    const byKey = Object.fromEntries(chips.map((chip) => [chip.key, chip]));

    for (const [key, status] of Object.entries(documents)) {
      expect(byKey[key].status, key).toBe(status);
    }
    expect(byKey.architecture.error).toBe("yaml: line 3: bad");
    expect(byKey.config.error).toBeUndefined();
  });

  it("also covers the ambiguous/unknown chip statuses", () => {
    const chips = documentChips(snapshot({ documents: { registry: "ambiguous", prd: "unknown" } }));
    expect(chips.find((chip) => chip.key === "registry")?.status).toBe("ambiguous");
    expect(chips.find((chip) => chip.key === "prd")?.status).toBe("unknown");
  });

  it("orders known keys canonically and appends unexpected keys last", () => {
    const chips = documentChips(snapshot({ documents: { "extra-key": "present", prd: "present", config: "present" } }));
    expect(chips.map((chip) => chip.key)).toEqual(["config", "prd", "extra-key"]);
  });

  it("returns an empty list for a null snapshot", () => {
    expect(documentChips(null)).toEqual([]);
  });
});

describe("appliedCell", () => {
  it("renders a countable x/y", () => {
    expect(appliedCell({ status: "counted", applied: 1, total: 2 })).toEqual({
      text: "1/2",
      status: "counted",
      applied: 1,
      total: 2,
    });
  });

  it("renders a fully-applied and a zero-applied count", () => {
    expect(appliedCell({ status: "counted", applied: 2, total: 2 }).text).toBe("2/2");
    expect(appliedCell({ status: "counted", applied: 0, total: 3 }).text).toBe("0/3");
  });

  const reasons: InventoryApplied["reason"][] = [
    "missing",
    "ambiguous",
    "invalid",
    "deprecated_format",
    "no_acceptance_criteria",
    "empty_internal_id",
  ];

  for (const reason of reasons) {
    it(`renders unknown (${reason})`, () => {
      const cell = appliedCell({ status: "unknown", reason });
      expect(cell.text).toBe(`unknown (${reason})`);
      expect(cell.status).toBe("unknown");
      expect(cell.reason).toBe(reason);
      expect(cell.applied).toBeUndefined();
      expect(cell.total).toBeUndefined();
    });
  }
});

describe("refinedText", () => {
  it("maps not_recorded to the literal 'not recorded'", () => {
    expect(refinedText("not_recorded")).toBe("not recorded");
  });
});

describe("storyFlags", () => {
  const cases: [keyof InventoryStoryFlags, string][] = [
    ["duplicate_declared_id", "story-flag-duplicate-declared-id"],
    ["id_mismatch", "story-flag-id-mismatch"],
    ["deprecated_format", "story-flag-deprecated-format"],
    ["no_acs", "story-flag-no-acs"],
    ["empty_internal_id", "story-flag-empty-internal-id"],
  ];

  for (const [key, testid] of cases) {
    it(`emits ${testid} when ${key} holds`, () => {
      const flags = storyFlags({ ...NO_FLAGS, [key]: true });
      expect(flags.map((flag) => flag.testid)).toEqual([testid]);
    });
  }

  it("emits nothing when no flag holds and every flag when all hold", () => {
    expect(storyFlags(NO_FLAGS)).toEqual([]);
    const all = storyFlags({
      duplicate_declared_id: true,
      id_mismatch: true,
      deprecated_format: true,
      no_acs: true,
      empty_internal_id: true,
    });
    expect(all).toHaveLength(5);
  });
});

describe("storyRow", () => {
  const createdStates = ["one", "missing", "ambiguous", "invalid"];

  for (const created of createdStates) {
    it(`maps created=${created}`, () => {
      expect(storyRow(story({ created })).created).toBe(created);
    });
  }

  it("maps the full identity tuple and cells", () => {
    const view = storyRow(story());
    expect(view).toMatchObject({
      createId: "70.1",
      epicNumber: 70,
      position: 1,
      declaredId: "77.5",
      fileId: "88.9",
      created: "one",
      refined: "not recorded",
    });
    expect(view.applied.text).toBe("1/2");
  });

  it("defaults a missing file_id to an empty string", () => {
    expect(storyRow(story({ file_id: undefined })).fileId).toBe("");
  });
});

describe("epicRow", () => {
  function epic(overrides: Partial<InventoryEpic> = {}): InventoryEpic {
    return {
      file: "epic-70-x.yaml",
      number: 70,
      status: "parseable",
      doc_id: 70,
      id_mismatch: false,
      duplicate_number: false,
      stories: [],
      ...overrides,
    };
  }

  it("renders a parseable epic with no flags", () => {
    const view = epicRow(epic());
    expect(view.status).toBe("parseable");
    expect(view.flagDuplicateNumber).toBe(false);
    expect(view.flagIdMismatch).toBe(false);
  });

  it("renders an invalid epic carrying its error", () => {
    const view = epicRow(epic({ status: "invalid", error: "yaml: bad" }));
    expect(view.status).toBe("invalid");
    expect(view.error).toBe("yaml: bad");
  });

  it("flags duplicate filename number and id mismatch", () => {
    const view = epicRow(epic({ duplicate_number: true, id_mismatch: true }));
    expect(view.flagDuplicateNumber).toBe(true);
    expect(view.flagIdMismatch).toBe(true);
  });

  it("maps nested stories", () => {
    const view = epicRow(epic({ stories: [story({ create_id: "70.1" }), story({ create_id: "70.2" })] }));
    expect(view.stories.map((entry) => entry.createId)).toEqual(["70.1", "70.2"]);
  });
});

describe("pathMismatch", () => {
  it("reflects the snapshot flag", () => {
    expect(pathMismatch(snapshot({ architecture_path_mismatch: true }))).toBe(true);
    expect(pathMismatch(snapshot({ architecture_path_mismatch: false }))).toBe(false);
    expect(pathMismatch(null)).toBe(false);
  });
});

describe("storyDispatch (per-action identifiers)", () => {
  const view = storyRow(story({ create_id: "42.1", declared_id: "77.5" }));

  it("create sends the position-derived create id", () => {
    expect(storyDispatch(view, "create")).toEqual({ command: "us-create", story_id: "42.1" });
  });

  it("refine sends the epic-declared id", () => {
    expect(storyDispatch(view, "refine")).toEqual({ command: "us-refine", story_id: "77.5" });
  });

  it("apply sends the epic-declared id", () => {
    expect(storyDispatch(view, "apply")).toEqual({ command: "us-apply", story_id: "77.5" });
  });
});

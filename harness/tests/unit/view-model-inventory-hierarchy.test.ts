/**
 * Extended inventory view-model tables (plan §4.6, hierarchy rework). These
 * cover the NEW pure functions the session page derives the epic accordion
 * and the story review modal from:
 *
 *   - `epicSections(snapshot, collapsed)` — epic sections in scanner order,
 *     the panel-existence rule (a toggle exists only when the epic has story
 *     rows), the `Epic.Title` → filename fallback, and the collapsed-set
 *     behavior (absence = expanded; a new epic auto-expands).
 *   - `storyModalModel(storyRow, epicRow)` — tab availability, error mode
 *     (invalid ⇒ Raw + error), the file→declared→identity fallback chain,
 *     the chips, statement/AC/step normalization (incl. the `but` modifier),
 *     and every truncation/omission notice + the changed-on-disk signal.
 *
 * These functions do NOT exist yet: the module is accessed through a pinned
 * TARGET interface (cast), so the file TYPECHECKS today and every table is
 * RED at runtime (the functions are undefined) until the view-model is
 * extended. The implementer builds `app/lib/view-model/inventory.ts` to
 * export exactly these signatures.
 */

import { describe, expect, it } from "vitest";

import * as inventoryVM from "../../app/lib/view-model/inventory";

// ── Pinned target schema (the wire the Go scanner will emit, plan §1) ──

interface TargetStep {
  kind: "given" | "when" | "then" | "and" | "but";
  text: string;
}

interface TargetContent {
  id: string;
  title: string;
  status: string;
  statement: { as_a: string; i_want: string; so_that: string };
  acceptance_criteria: { id: string; description: string; steps: TargetStep[] }[];
}

interface TargetStory {
  create_id: string;
  epic_number: number;
  position: number;
  declared_id: string;
  file_id?: string;
  created: string;
  applied: { status: string; applied?: number; total?: number; reason?: string };
  refined: string;
  flags: {
    duplicate_declared_id: boolean;
    id_mismatch: boolean;
    deprecated_format: boolean;
    no_acs: boolean;
    empty_internal_id: boolean;
  };
  declared_title?: string;
  declared_status?: string;
  match_count?: number;
  source_file?: string;
  error?: string;
  content?: TargetContent | null;
  declared_content?: TargetContent | null;
  raw?: string;
  raw_truncated?: boolean;
  raw_omitted?: boolean;
  content_omitted?: boolean;
  omission_reason?: string;
}

interface TargetEpic {
  file: string;
  number: number;
  status: string;
  title?: string;
  doc_id: number;
  id_mismatch: boolean;
  duplicate_number: boolean;
  noncanonical_filename?: boolean;
  error?: string;
  stories: TargetStory[];
}

interface TargetSnapshot {
  documents: Record<string, string>;
  architecture_path_mismatch: boolean;
  epics: TargetEpic[];
}

// ── Pinned target return shapes ──

interface EpicSectionView {
  file: string;
  number: number;
  title: string;
  status: string;
  hasPanel: boolean;
  collapsed: boolean;
  stories: { createId: string }[];
}

interface StoryModalView {
  storyId: string;
  title: string;
  status: string;
  source: "file" | "declared" | "identity";
  defaultTab: "review" | "raw";
  errorMode: boolean;
  review: {
    enabled: boolean;
    statement: { asA: string; iWant: string; soThat: string } | null;
    acceptanceCriteria: { id: string; description: string; steps: TargetStep[] }[];
  };
  raw: { available: boolean; text: string; truncated: boolean };
  chips: { created: string; applied: { text: string; status: string }; refined: string };
  notices: { reason: string }[];
  changedOnDisk: boolean;
}

interface TargetVM {
  epicSections: (snapshot: TargetSnapshot | null, collapsed?: Set<string>) => EpicSectionView[];
  storyModalModel: (story: TargetStory, epic: TargetEpic, flags?: { changedOnDisk?: boolean }) => StoryModalView;
}

const vm = inventoryVM as unknown as TargetVM;

// ── Builders ──

function step(kind: TargetStep["kind"], text: string): TargetStep {
  return { kind, text };
}

function content(overrides: Partial<TargetContent> = {}): TargetContent {
  return {
    id: "60.2",
    title: "Summary For Shared Docs",
    status: "PLANNED",
    statement: { as_a: "Claude User", i_want: "a short summary", so_that: "I can decide" },
    acceptance_criteria: [
      {
        id: "AC-1",
        description: "a 150-word summary appears",
        steps: [step("given", "a shared doc"), step("when", "I ask"), step("then", "a summary shows"), step("and", "it names the title")],
      },
      {
        id: "AC-2",
        description: "an unshared doc is refused",
        steps: [step("given", "an unshared doc"), step("then", "a refusal shows"), step("but", "no content is shown")],
      },
    ],
    ...overrides,
  };
}

function story(overrides: Partial<TargetStory> = {}): TargetStory {
  return {
    create_id: "60.2",
    epic_number: 60,
    position: 2,
    declared_id: "60.2",
    file_id: "60.2",
    created: "one",
    applied: { status: "counted", applied: 1, total: 2 },
    refined: "not_recorded",
    flags: { duplicate_declared_id: false, id_mismatch: false, deprecated_format: false, no_acs: false, empty_internal_id: false },
    declared_title: "Summary For Shared Docs",
    declared_status: "PLANNED",
    match_count: 1,
    source_file: "60.2-summary-shared-docs.yaml",
    content: content(),
    declared_content: content(),
    raw: "story:\n  id: 60.2\n",
    raw_truncated: false,
    raw_omitted: false,
    content_omitted: false,
    ...overrides,
  };
}

function epic(overrides: Partial<TargetEpic> = {}): TargetEpic {
  return {
    file: "epic-60-inventory-spread.yaml",
    number: 60,
    status: "parseable",
    title: "Inventory Spread Epic",
    doc_id: 60,
    id_mismatch: false,
    duplicate_number: false,
    stories: [story()],
    ...overrides,
  };
}

function snapshot(epics: TargetEpic[]): TargetSnapshot {
  return { documents: {}, architecture_path_mismatch: false, epics };
}

// ── epicSections ──

describe("epicSections", () => {
  it("derives one section per epic in scanner order (no re-sort)", () => {
    const snap = snapshot([
      epic({ file: "epic-60-b.yaml", number: 60 }),
      epic({ file: "epic-07-a.yaml", number: 7 }),
    ]);
    expect(vm.epicSections(snap).map((section) => section.file)).toEqual(["epic-60-b.yaml", "epic-07-a.yaml"]);
  });

  it("panel-existence rule: hasPanel iff the epic carries story rows", () => {
    const withRows = vm.epicSections(snapshot([epic({ stories: [story()] })]))[0];
    const noRows = vm.epicSections(snapshot([epic({ stories: [], status: "invalid" })]))[0];
    expect(withRows.hasPanel).toBe(true);
    expect(noRows.hasPanel).toBe(false);
  });

  it("title falls back to the filename when Epic.Title is empty", () => {
    expect(vm.epicSections(snapshot([epic({ title: "Inventory Spread Epic" })]))[0].title).toBe("Inventory Spread Epic");
    expect(vm.epicSections(snapshot([epic({ title: "" })]))[0].title).toBe("epic-60-inventory-spread.yaml");
  });

  it("collapsed-set: absence = expanded, membership = collapsed, new epics auto-expand", () => {
    const snap = snapshot([epic({ file: "epic-60-inventory-spread.yaml" }), epic({ file: "epic-07-a.yaml", number: 7 })]);
    const collapsed = new Set(["epic-07-a.yaml"]);
    const sections = vm.epicSections(snap, collapsed);
    const byFile = Object.fromEntries(sections.map((section) => [section.file, section]));
    expect(byFile["epic-60-inventory-spread.yaml"].collapsed).toBe(false);
    expect(byFile["epic-07-a.yaml"].collapsed).toBe(true);
    // No collapsed set at all ⇒ everything expanded.
    expect(vm.epicSections(snap).every((section) => section.collapsed === false)).toBe(true);
  });

  it("preserves the nested story order", () => {
    const section = vm.epicSections(snapshot([epic({ stories: [story({ create_id: "60.1" }), story({ create_id: "60.2" })] })]))[0];
    expect(section.stories.map((entry) => entry.createId)).toEqual(["60.1", "60.2"]);
  });
});

// ── storyModalModel ──

describe("storyModalModel", () => {
  it("happy path: file source, Review default+enabled, statement + AC steps incl. and/but", () => {
    const model = vm.storyModalModel(story(), epic());
    expect(model.source).toBe("file");
    expect(model.defaultTab).toBe("review");
    expect(model.errorMode).toBe(false);
    expect(model.review.enabled).toBe(true);
    expect(model.raw.available).toBe(true);
    expect(model.storyId).toBe("60.2");
    const kinds = model.review.acceptanceCriteria.flatMap((ac) => ac.steps.map((entry) => entry.kind));
    expect(kinds).toContain("and");
    expect(kinds).toContain("but");
    expect(model.chips.applied.text).toBe("1/2");
    expect(model.chips.refined).toBe("not recorded");
  });

  it("invalid story: opens on Raw with error mode; Review falls back to declared content", () => {
    const invalid = story({ created: "invalid", content: null, error: "yaml: bad", raw: "story:\n  id: 70.1\n  title: \"broken", declared_content: content() });
    const model = vm.storyModalModel(invalid, epic());
    expect(model.errorMode).toBe(true);
    expect(model.defaultTab).toBe("raw");
    expect(model.raw.available).toBe(true);
    // The epic declares it ⇒ Review is enabled on the declared content.
    expect(model.review.enabled).toBe(true);
    expect(model.review.statement).not.toBeNull();
  });

  it("invalid story WITHOUT declared content: Review disabled, invalid_without_raw notice when raw omitted too", () => {
    const bare = story({ created: "invalid", content: null, declared_content: null, error: "yaml: bad", raw_omitted: true, raw: "", omission_reason: "invalid_without_raw" });
    const model = vm.storyModalModel(bare, epic({ stories: [] }));
    expect(model.review.enabled).toBe(false);
    expect(model.notices.map((notice) => notice.reason)).toContain("invalid_without_raw");
  });

  it("fallback chain: missing file ⇒ declared source with a not_created notice and no Raw", () => {
    const missing = story({ created: "missing", file_id: undefined, source_file: undefined, match_count: 0, content: null, raw: undefined, declared_content: content() });
    const model = vm.storyModalModel(missing, epic());
    expect(model.source).toBe("declared");
    expect(model.raw.available).toBe(false);
    expect(model.notices.map((notice) => notice.reason)).toContain("not_created");
    expect(model.review.enabled).toBe(true);
  });

  it("fallback chain: identity-only when neither file nor declared content exists", () => {
    const identity = story({ created: "missing", content: null, declared_content: null, declared_title: "", declared_status: "" });
    const model = vm.storyModalModel(identity, epic({ stories: [] }));
    expect(model.source).toBe("identity");
    expect(model.status).toBe("unknown");
    expect(model.review.enabled).toBe(false);
  });

  it("ambiguous ⇒ declared content + a match-count notice", () => {
    const ambiguous = story({ created: "ambiguous", match_count: 2, content: null, source_file: undefined, declared_content: content() });
    const model = vm.storyModalModel(ambiguous, epic());
    expect(model.source).toBe("declared");
    expect(model.notices.some((notice) => notice.reason === "ambiguous")).toBe(true);
  });

  const omissionCases: [Partial<TargetStory>, string][] = [
    [{ raw_omitted: true, omission_reason: "raw_omitted" }, "raw_omitted"],
    [{ content_omitted: true, content: null, omission_reason: "content_omitted" }, "content_omitted"],
    [{ raw_omitted: true, content_omitted: true, content: null, omission_reason: "both_omitted" }, "both_omitted"],
    [{ raw_truncated: true }, "raw_truncated"],
  ];

  for (const [overrides, reason] of omissionCases) {
    it(`surfaces the ${reason} notice`, () => {
      const model = vm.storyModalModel(story(overrides), epic());
      expect(model.notices.map((notice) => notice.reason)).toContain(reason);
    });
  }

  it("raw omitted ⇒ Raw tab unavailable", () => {
    const model = vm.storyModalModel(story({ raw_omitted: true, raw: "", omission_reason: "raw_omitted" }), epic());
    expect(model.raw.available).toBe(false);
  });

  it("changed-on-disk signal surfaces a changed_on_disk notice", () => {
    const model = vm.storyModalModel(story(), epic(), { changedOnDisk: true });
    expect(model.changedOnDisk).toBe(true);
    expect(model.notices.map((notice) => notice.reason)).toContain("changed_on_disk");
  });

  it("status falls back declared_status then unknown", () => {
    expect(vm.storyModalModel(story({ content: content({ status: "REFINED" }) }), epic()).status).toBe("REFINED");
    expect(vm.storyModalModel(story({ content: null, declared_content: null, declared_status: "PLANNED" }), epic()).status).toBe("PLANNED");
  });
});

// ── storyModalModel heading fallback chain (finding 5a) ──

describe("storyModalModel heading fallback chain", () => {
  it("storyId: file-content id → declared id → create identity → unknown", () => {
    // File content id wins.
    expect(vm.storyModalModel(story({ content: content({ id: "88.9" }) }), epic()).storyId).toBe("88.9");
    // No file content ⇒ the epic-declared id.
    expect(vm.storyModalModel(story({ content: null, declared_content: null, declared_id: "77.5" }), epic()).storyId).toBe("77.5");
    // Neither ⇒ the create identity.
    expect(
      vm.storyModalModel(story({ content: null, declared_content: null, declared_id: "", create_id: "60.2" }), epic()).storyId,
    ).toBe("60.2");
  });

  it("title: file-content title → declared title → declared-content title → unknown", () => {
    expect(vm.storyModalModel(story({ content: content({ title: "File Title" }) }), epic()).title).toBe("File Title");
    expect(
      vm.storyModalModel(story({ content: null, declared_content: null, declared_title: "Declared Title" }), epic()).title,
    ).toBe("Declared Title");
    expect(
      vm.storyModalModel(
        story({ content: null, declared_title: "", declared_content: content({ title: "Declared Content Title" }) }),
        epic(),
      ).title,
    ).toBe("Declared Content Title");
    // Nothing anywhere ⇒ the explicit "unknown" (never an empty heading).
    expect(vm.storyModalModel(story({ content: null, declared_content: null, declared_title: "" }), epic()).title).toBe("unknown");
  });
});

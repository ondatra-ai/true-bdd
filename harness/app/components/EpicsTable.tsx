"use client";

import { useState } from "react";

import { postJson } from "../lib/use-poll";
import { appliedCell, epicSections, refinedText, statusTone, storyFlags } from "../lib/view-model/inventory";
import { storyRowTestId } from "../lib/view-model/ui-ids";
import type { InventoryEpic, InventorySnapshot, InventoryStory } from "../lib/view-model/types";

type StoryAction = "create" | "refine" | "apply";

/**
 * The (command, story_id) a per-story action dispatches (plan §3.4): create
 * uses the position-derived create id (`42.1`); refine/apply use the
 * epic-declared id (`77.5`).
 */
function actionDispatch(story: InventoryStory, action: StoryAction): { command: string; story_id: string } {
  switch (action) {
    case "create":
      return { command: "us-create", story_id: story.create_id };
    case "refine":
      return { command: "us-refine", story_id: story.declared_id };
    case "apply":
      return { command: "us-apply", story_id: story.declared_id };
  }
}

/**
 * The epic→story accordion (plan §3). Each epic renders as an
 * `epic-section` wrapper (its header `epic-row` + the story-row panel) so
 * story lookups are scoped per epic — collision-safe for duplicate epic
 * numbers. Epics render DEFAULT-EXPANDED; a toggle is shown only when a
 * story panel exists. Every declared story row exposes a `story-title`
 * opener button for the review modal. All existing story-row testids /
 * cells / data-* attributes are preserved exactly (README-testids).
 */
export function EpicsTable({
  snapshot,
  collapsed,
  onToggle,
  onOpenStory,
  sessionId,
  disabled,
}: {
  snapshot: InventorySnapshot | null;
  collapsed: Set<string>;
  onToggle: (epicFile: string) => void;
  onOpenStory: (epicFile: string, position: number, opener: HTMLElement) => void;
  sessionId: string;
  disabled: boolean;
}) {
  const sections = epicSections(snapshot, collapsed);
  const epicsByFile = new Map((snapshot?.epics ?? []).map((epic) => [epic.file, epic]));

  if (sections.length === 0) {
    return <p className="sf-empty">No epics found in this inventory.</p>;
  }

  return (
    <div className="sf-epics">
      {sections.map((section) => {
        const epic = epicsByFile.get(section.file);
        if (epic === undefined) {
          return null;
        }

        return (
          <div
            key={section.file}
            className="sf-epic"
            data-testid="epic-section"
            data-epic-file={section.file}
            data-epic-number={section.number}
          >
            <div
              className="sf-epic-head"
              data-testid="epic-row"
              data-epic-file={section.file}
              data-epic-number={section.number}
              data-status={section.status}
            >
              <span className="sf-epic-num">{section.number}</span>
              <span className="sf-epic-title" data-testid="epic-title">
                {section.title}
              </span>
              <span className="sf-epic-file">{section.file}</span>
              <span className="sf-chip" data-tone={statusTone(section.status)}>
                <span className="sq" />
                {section.status}
              </span>
              <EpicFlags epic={epic} />
              <span className="sf-epic-spacer" />
              {section.hasPanel ? (
                <button
                  type="button"
                  className="sf-btn sf-btn-sm"
                  data-testid="epic-toggle"
                  aria-expanded={!section.collapsed}
                  onClick={() => onToggle(section.file)}
                >
                  {section.collapsed ? "Expand" : "Collapse"}
                </button>
              ) : null}
            </div>

            {section.hasPanel && !section.collapsed ? (
              <table className="sf-stories">
                <thead>
                  <tr>
                    <th>Story</th>
                    <th>Created</th>
                    <th>Applied</th>
                    <th>Refined</th>
                    <th>Actions</th>
                  </tr>
                </thead>
                <tbody>
                  {epic.stories.map((story) => (
                    <StoryRow
                      key={story.create_id}
                      story={story}
                      sessionId={sessionId}
                      disabled={disabled}
                      onOpen={(opener) => onOpenStory(section.file, story.position, opener)}
                    />
                  ))}
                </tbody>
              </table>
            ) : null}
          </div>
        );
      })}
    </div>
  );
}

function EpicFlags({ epic }: { epic: InventoryEpic }) {
  return (
    <>
      {epic.duplicate_number ? (
        <span className="sf-flag" data-testid="epic-flag-duplicate-number">
          duplicate number
        </span>
      ) : null}
      {epic.id_mismatch ? (
        <span className="sf-flag" data-testid="epic-flag-id-mismatch">
          id mismatch
        </span>
      ) : null}
      {epic.noncanonical_filename ? (
        <span className="sf-flag" data-testid="epic-flag-noncanonical-filename">
          noncanonical filename
        </span>
      ) : null}
    </>
  );
}

const FLAG_TESTIDS: { key: keyof InventoryStory["flags"]; testid: string; label: string }[] = [
  { key: "duplicate_declared_id", testid: "story-flag-duplicate-declared-id", label: "duplicate id" },
  { key: "id_mismatch", testid: "story-flag-id-mismatch", label: "id mismatch" },
  { key: "deprecated_format", testid: "story-flag-deprecated-format", label: "deprecated" },
  { key: "no_acs", testid: "story-flag-no-acs", label: "no acs" },
  { key: "empty_internal_id", testid: "story-flag-empty-internal-id", label: "empty id" },
];

function StoryRow({
  story,
  sessionId,
  disabled,
  onOpen,
}: {
  story: InventoryStory;
  sessionId: string;
  disabled: boolean;
  onOpen: (opener: HTMLElement) => void;
}) {
  const [fix, setFix] = useState(false);
  const applied = appliedCell(story.applied);
  const flags = storyFlags(story.flags);
  const ambiguous = story.created === "ambiguous";
  const title = story.declared_title || story.content?.title || story.create_id;

  async function dispatch(action: StoryAction): Promise<void> {
    const { command, story_id } = actionDispatch(story, action);
    await postJson(`/api/sessions/${sessionId}/runs`, {
      command,
      story_id,
      fix,
      client_token: crypto.randomUUID(),
    });
  }

  return (
    <tr
      data-testid={storyRowTestId(story.create_id)}
      data-epic-number={story.epic_number}
      data-position={story.position}
      data-declared-id={story.declared_id}
      data-file-id={story.file_id ?? ""}
    >
      <td>
        <strong>{story.create_id}</strong>
        <div style={{ marginTop: 4 }}>
          <button
            type="button"
            className="sf-story-open"
            data-testid="story-title"
            data-match-count={ambiguous ? story.match_count : undefined}
            onClick={(event) => {
              // Focus the opener and pass it up so close restores focus to it:
              // on macOS a <button> is not focused on click, so the native
              // <dialog> has no element to return focus to on close.
              event.currentTarget.focus();
              onOpen(event.currentTarget);
            }}
          >
            {title}
            {ambiguous ? ` (${story.match_count} matches)` : ""}
          </button>
        </div>
        <div>
          {flags.map((flag) => (
            <span key={flag.testid} className="sf-flag" data-testid={flag.testid}>
              {FLAG_TESTIDS.find((entry) => entry.testid === flag.testid)?.label ?? flag.testid}
            </span>
          ))}
        </div>
      </td>
      <td data-testid="story-created" data-status={story.created}>
        <span className="sf-chip" data-tone={statusTone(story.created)}>
          <span className="sq" />
          {story.created}
        </span>
      </td>
      <td
        data-testid="story-applied"
        data-status={applied.status}
        data-applied={applied.applied}
        data-total={applied.total}
        data-reason={applied.reason}
      >
        <span className="sf-chip" data-tone={statusTone(applied.status)}>
          <span className="sq" />
          {applied.text}
        </span>
      </td>
      <td data-testid="story-refined">{refinedText(story.refined)}</td>
      <td>
        <div className="sf-actions">
          <button
            type="button"
            className="sf-btn sf-btn-sm"
            data-testid="action-create"
            disabled={disabled}
            onClick={() => void dispatch("create")}
          >
            Create
          </button>
          <button
            type="button"
            className="sf-btn sf-btn-sm"
            data-testid="action-refine"
            disabled={disabled}
            onClick={() => void dispatch("refine")}
          >
            Refine
          </button>
          <button
            type="button"
            className="sf-btn sf-btn-sm"
            data-testid="action-apply"
            disabled={disabled}
            onClick={() => void dispatch("apply")}
          >
            Apply
          </button>
          <label className="sf-fix">
            <input type="checkbox" data-testid="fix-toggle" checked={fix} onChange={(event) => setFix(event.target.checked)} />
            fix
          </label>
        </div>
      </td>
    </tr>
  );
}

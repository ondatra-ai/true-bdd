"use client";

import { useState } from "react";

import { postJson } from "../lib/use-poll";
import { storyDispatch, type EpicRowView, type StoryAction, type StoryRowView } from "../lib/view-model/inventory";
import { storyRowTestId } from "../lib/view-model/ui-ids";
import { button, cell, flag as flagStyle, statusColor, table } from "./styles";

/**
 * The epics/stories table (plan §3.5). Each epic row carries
 * data-epic-file / data-epic-number / data-status and its identity flags;
 * each story row carries the identity tuple as data-* attributes plus the
 * created / applied / refined cells, flag chips, and the per-story
 * Create/Refine/Apply actions with a --fix toggle. Selector + attribute
 * vocabulary: README-testids.
 */
export function EpicsTable({
  epics,
  sessionId,
  disabled,
}: {
  epics: EpicRowView[];
  sessionId: string;
  disabled: boolean;
}) {
  if (epics.length === 0) {
    return <p style={{ color: "#888" }}>No epics found in this inventory.</p>;
  }

  return (
    <div style={{ overflowX: "auto" }}>
      <table style={table}>
        <thead>
          <tr>
            <th style={cell}>story</th>
            <th style={cell}>identity</th>
            <th style={cell}>created</th>
            <th style={cell}>applied</th>
            <th style={cell}>refined</th>
            <th style={cell}>actions</th>
          </tr>
        </thead>
        <tbody>
          {epics.map((epic) => (
            <EpicGroup key={epic.file} epic={epic} sessionId={sessionId} disabled={disabled} />
          ))}
        </tbody>
      </table>
    </div>
  );
}

function EpicGroup({ epic, sessionId, disabled }: { epic: EpicRowView; sessionId: string; disabled: boolean }) {
  return (
    <>
      <tr
        data-testid="epic-row"
        data-epic-file={epic.file}
        data-epic-number={epic.number}
        data-status={epic.status}
        style={{ background: "#f2f5f8" }}
      >
        <td style={{ ...cell, fontWeight: 700 }} colSpan={6}>
          <span>epic {epic.number}</span>
          <span style={{ color: "#666", marginLeft: 8 }}>{epic.file}</span>
          <span style={{ color: statusColor(epic.status), marginLeft: 8 }}>[{epic.status}]</span>
          {epic.flagDuplicateNumber ? (
            <span data-testid="epic-flag-duplicate-number" style={flagStyle}>
              duplicate number
            </span>
          ) : null}
          {epic.flagIdMismatch ? (
            <span data-testid="epic-flag-id-mismatch" style={flagStyle}>
              id mismatch
            </span>
          ) : null}
          {epic.flagNoncanonicalFilename ? (
            <span data-testid="epic-flag-noncanonical-filename" style={flagStyle}>
              noncanonical filename
            </span>
          ) : null}
          {epic.error ? <span style={{ color: "#c0341d", marginLeft: 8 }}>{epic.error}</span> : null}
        </td>
      </tr>
      {epic.stories.map((story) => (
        <StoryRow key={story.createId} story={story} sessionId={sessionId} disabled={disabled} />
      ))}
    </>
  );
}

function StoryRow({ story, sessionId, disabled }: { story: StoryRowView; sessionId: string; disabled: boolean }) {
  const [fix, setFix] = useState(false);

  async function dispatch(action: StoryAction): Promise<void> {
    const { command, story_id } = storyDispatch(story, action);
    await postJson(`/api/sessions/${sessionId}/runs`, {
      command,
      story_id,
      fix,
      client_token: crypto.randomUUID(),
    });
  }

  return (
    <tr
      data-testid={storyRowTestId(story.createId)}
      data-epic-number={story.epicNumber}
      data-position={story.position}
      data-declared-id={story.declaredId}
      data-file-id={story.fileId}
    >
      <td style={cell}>
        <strong>{story.createId}</strong>
        <div>
          {story.flags.map((flag) => (
            <span key={flag.testid} data-testid={flag.testid} style={flagStyle}>
              {flag.testid.replace("story-flag-", "").replace(/-/g, " ")}
            </span>
          ))}
        </div>
      </td>
      <td style={{ ...cell, color: "#555" }}>
        <div>declared: {story.declaredId || "—"}</div>
        <div>file: {story.fileId || "—"}</div>
      </td>
      <td style={cell} data-testid="story-created" data-status={story.created}>
        <span style={{ color: statusColor(story.created) }}>{story.created}</span>
      </td>
      <td
        style={cell}
        data-testid="story-applied"
        data-status={story.applied.status}
        data-applied={story.applied.applied}
        data-total={story.applied.total}
        data-reason={story.applied.reason}
      >
        <span style={{ color: statusColor(story.applied.status) }}>{story.applied.text}</span>
      </td>
      <td style={cell} data-testid="story-refined">
        {story.refined}
      </td>
      <td style={cell}>
        <div style={{ display: "flex", gap: 6, alignItems: "center", flexWrap: "wrap" }}>
          <button type="button" data-testid="action-create" disabled={disabled} onClick={() => void dispatch("create")} style={button}>
            Create
          </button>
          <button type="button" data-testid="action-refine" disabled={disabled} onClick={() => void dispatch("refine")} style={button}>
            Refine
          </button>
          <button type="button" data-testid="action-apply" disabled={disabled} onClick={() => void dispatch("apply")} style={button}>
            Apply
          </button>
          <label style={{ display: "inline-flex", alignItems: "center", gap: 3 }}>
            <input
              type="checkbox"
              data-testid="fix-toggle"
              checked={fix}
              onChange={(event) => setFix(event.target.checked)}
            />
            --fix
          </label>
        </div>
      </td>
    </tr>
  );
}

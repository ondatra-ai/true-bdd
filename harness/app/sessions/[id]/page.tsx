"use client";

import Link from "next/link";
import { useParams } from "next/navigation";
import { useCallback, useRef, useState } from "react";

import { EpicsTable } from "../../components/EpicsTable";
import { InventoryChips } from "../../components/InventoryChips";
import { StoryModal } from "../../components/StoryModal";
import { usePoll } from "../../lib/use-poll";
import { documentChips, inventoryLimitTooSmall, inventoryTruncated, pathMismatch } from "../../lib/view-model/inventory";
import { controlsDisabled, siblingWarningVisible } from "../../lib/view-model/session";
import { outcomeBadge } from "../../lib/view-model/run";
import type {
  InventoryEpic,
  InventorySnapshot,
  InventoryStory,
  RunSummary,
  SessionDetail,
} from "../../lib/view-model/types";

interface SelectedStory {
  epicFile: string;
  position: number;
}

/**
 * Session detail (plan §4, view 2): an epic→story accordion with a native-
 * dialog story review modal, the inventory chips, the per-story + session
 * action controls, and the run history. v2 transport (plan §1.5/§3): ONE
 * live poll of GET /api/sessions/:id (`session_detail` = status + inventory
 * in one CLI-side consistent read). Every poll is a fresh CLI scan — there
 * is no generation, no cache. On `session_gone` (404) the page clears its
 * stale data and shows the disconnected/unavailable view; on `unavailable`
 * (504) it keeps the last data but marks it not-current.
 */
export default function SessionDetailPage() {
  const params = useParams<{ id: string }>();
  const sessionId = params.id;

  const [refreshTick, setRefreshTick] = useState(0);
  const { data: detail, status } = usePoll<SessionDetail>(`/api/sessions/${sessionId}`, 1000, refreshTick);

  const [collapsed, setCollapsed] = useState<Set<string>>(new Set());
  const [selected, setSelected] = useState<SelectedStory | null>(null);
  const [modalTab, setModalTab] = useState<"review" | "raw">("review");
  const [buildFix, setBuildFix] = useState(false);
  // The last live (story, epic) the open modal presented. When a fresher
  // snapshot no longer resolves the selected composite identity — the story
  // was removed/renamed on disk — the modal stays OPEN on this retained
  // presentation as a changed-on-disk fallback instead of silently
  // unmounting (P9 changed-on-disk).
  const [retained, setRetained] = useState<{ story: InventoryStory; epic: InventoryEpic } | null>(null);

  // A collapsed set + open modal are per-session, in-page only: reset them
  // when the id changes, using React's adjust-state-during-render pattern.
  const [prevSessionId, setPrevSessionId] = useState(sessionId);
  if (prevSessionId !== sessionId) {
    setPrevSessionId(sessionId);
    setCollapsed(new Set());
    setSelected(null);
    setRetained(null);
  }

  const inventory: InventorySnapshot | null = detail?.inventory ?? null;
  const chips = documentChips(inventory);
  const disabled = controlsDisabled(detail);
  const siblingWarn = siblingWarningVisible(detail);
  const truncated = inventoryTruncated(inventory);

  const toggleEpic = useCallback((epicFile: string) => {
    setCollapsed((prev) => {
      const next = new Set(prev);
      if (next.has(epicFile)) {
        next.delete(epicFile);
      } else {
        next.add(epicFile);
      }

      return next;
    });
  }, []);

  const openerRef = useRef<HTMLElement | null>(null);

  const openStory = useCallback(
    (epicFile: string, position: number, opener: HTMLElement) => {
      openerRef.current = opener;
      const story = findStory(inventory, epicFile, position);
      setModalTab(story?.created === "invalid" ? "raw" : "review");
      setSelected({ epicFile, position });
    },
    [inventory],
  );

  // Closing the modal restores focus to the row's story-title opener (the
  // native <dialog> may not, because a macOS <button> is not focused on
  // click). Runs after the dialog's own close, so it wins.
  const closeStory = useCallback(() => {
    setSelected(null);
    const opener = openerRef.current;
    if (opener !== null) {
      queueMicrotask(() => opener.focus());
    }
  }, []);

  function refresh(): void {
    // An immediate live re-READ (a fresh CLI scan), NOT a mutation — there
    // is no /refresh route in v2 (plan §1.5).
    setRefreshTick((tick) => tick + 1);
  }

  async function dispatchBuild(command: "build-tests" | "build-code"): Promise<void> {
    await fetch(`/api/sessions/${sessionId}/runs`, {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ command, fix: buildFix, client_token: crypto.randomUUID() }),
    });
  }

  const selectedEpic = selected ? findEpic(inventory, selected.epicFile) : undefined;
  const selectedStory = selected ? findStory(inventory, selected.epicFile, selected.position) : undefined;

  // Retain the last LIVE presentation so a story that vanishes from a fresher
  // snapshot keeps the modal open as a changed-on-disk fallback (P9). Uses the
  // adjust-state-during-render pattern: the guards make it converge (resolved
  // story/epic are stable references while the snapshot is unchanged), so it
  // never loops. A vanished story matches neither branch, leaving the last
  // retained value in place.
  if (selected && selectedStory !== undefined && selectedEpic !== undefined) {
    if (retained?.story !== selectedStory || retained?.epic !== selectedEpic) {
      setRetained({ story: selectedStory, epic: selectedEpic });
    }
  } else if (!selected && retained !== null) {
    setRetained(null);
  }

  const modalPresentation =
    selected && selectedStory && selectedEpic
      ? { story: selectedStory, epic: selectedEpic, changedOnDisk: false }
      : selected && retained
        ? { story: retained.story, epic: retained.epic, changedOnDisk: true }
        : null;

  // Honest poll states (plan §4): session_gone clears data → a terminal
  // disconnected view; unavailable keeps the last data but marks it stale.
  const gone = status === "session_gone";
  const stale = status === "unavailable";

  if (gone) {
    return (
      <main className="sf-session">
        <p className="sf-crumb">
          <Link href="/">← Sessions</Link>
        </p>
        <div className="sf-banner" data-tone="error" data-testid="unavailable-state">
          This remote has disconnected — its session is gone. The run history persists in the CLI store and is
          reachable again once the remote reconnects.
        </div>
      </main>
    );
  }

  return (
    <main className="sf-session">
      <p className="sf-crumb">
        <Link href="/">← Sessions</Link>
      </p>

      <header className="sf-header">
        <h1 className="sf-title">{detail?.session_id ?? sessionId}</h1>
        <div className="sf-meta">
          <button type="button" className="sf-btn sf-btn-sm" data-testid="refresh" onClick={refresh}>
            Refresh
          </button>
        </div>
      </header>

      {stale ? (
        <div className="sf-banner" data-tone="warn" data-testid="unavailable-state">
          The CLI did not respond in time — showing the last known state, which may be out of date.
        </div>
      ) : null}

      {pathMismatch(inventory) ? (
        <div className="sf-banner" data-tone="warn" data-testid="path-mismatch-warning">
          Architecture path mismatch: scanning the configured <code>{inventory?.configured_architecture_path}</code>{" "}
          instead of the canonical default <code>{inventory?.canonical_architecture_path}</code>.
        </div>
      ) : null}

      {siblingWarn ? (
        <div className="sf-banner" data-tone="warn" data-testid="folder-warning-banner">
          Another session on this folder has an active run. The host-side flock enforces mutual exclusion; a dispatch
          here may fail fast with <code>error(folder_locked)</code>.
        </div>
      ) : null}

      {truncated ? (
        <div className="sf-banner" data-tone="warn" data-testid="inventory-truncated-banner">
          {truncationText(inventory)}
        </div>
      ) : null}

      <h2 className="sf-section-label">
        <span className="num">01—</span>Inventory
      </h2>
      <InventoryChips chips={chips} />

      <h2 className="sf-section-label">
        <span className="num">02—</span>Build pipeline
      </h2>
      <div className="sf-toggle">
        <button
          type="button"
          className="sf-btn"
          data-testid="action-build-tests"
          disabled={disabled}
          onClick={() => void dispatchBuild("build-tests")}
        >
          build tests
        </button>
        <button
          type="button"
          className="sf-btn"
          data-testid="action-build-code"
          disabled={disabled}
          onClick={() => void dispatchBuild("build-code")}
        >
          build code
        </button>
        <label className="sf-fix">
          <input type="checkbox" checked={buildFix} onChange={(event) => setBuildFix(event.target.checked)} />
          fix
        </label>
      </div>

      <h2 className="sf-section-label">
        <span className="num">03—</span>Epics
      </h2>
      <EpicsTable
        snapshot={inventory}
        collapsed={collapsed}
        onToggle={toggleEpic}
        onOpenStory={openStory}
        sessionId={sessionId}
        disabled={disabled}
      />

      <h2 className="sf-section-label">
        <span className="num">04—</span>Runs
      </h2>
      <RunHistory runs={detail?.runs ?? []} sessionId={sessionId} />

      {selected && modalPresentation ? (
        <StoryModal
          key={`${selected.epicFile}:${selected.position}`}
          story={modalPresentation.story}
          epic={modalPresentation.epic}
          activeTab={modalTab}
          onTab={setModalTab}
          onClose={closeStory}
          changedOnDisk={modalPresentation.changedOnDisk}
        />
      ) : null}
    </main>
  );
}

function RunHistory({ runs, sessionId }: { runs: RunSummary[]; sessionId: string }) {
  if (runs.length === 0) {
    return <p className="sf-empty">No runs yet.</p>;
  }

  return (
    <div data-testid="run-history" className="sf-epics" style={{ overflowX: "auto" }}>
      <table className="sf-stories">
        <thead>
          <tr>
            <th>Command</th>
            <th>Story</th>
            <th>Fix</th>
            <th>State</th>
            <th>Outcome</th>
          </tr>
        </thead>
        <tbody>
          {runs.map((run) => (
            <tr key={run.run_id} data-testid="run-row" data-run-id={run.run_id} data-command={run.command}>
              <td>
                <Link href={`/sessions/${sessionId}/runs/${run.run_id}`}>{run.command}</Link>
              </td>
              <td>{run.story_id ?? "—"}</td>
              <td>{run.fix ? "fix" : ""}</td>
              <td>{run.state}</td>
              <td>{outcomeBadge(run)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

/** Names the degrade mode(s) reached for the inventory-truncated banner. */
function truncationText(snapshot: InventorySnapshot | null): string {
  if (inventoryLimitTooSmall(snapshot)) {
    return "The inventory could not fit the server request budget (limit too small) — showing global counts only.";
  }
  if (snapshot === null) {
    return "The inventory snapshot was degraded to fit the request budget.";
  }

  const modes: string[] = [];
  if ((snapshot.stories_omitted ?? 0) > 0) {
    modes.push(`${snapshot.stories_omitted} story row(s) omitted`);
  }
  const rawOmitted = (snapshot.epics ?? []).some((epic) => (epic.stories ?? []).some((story) => story.raw_omitted));
  const contentOmitted = (snapshot.epics ?? []).some((epic) =>
    (epic.stories ?? []).some((story) => story.content_omitted),
  );
  if (rawOmitted) {
    modes.push("raw omitted");
  }
  if (contentOmitted) {
    modes.push("content omitted");
  }

  return `Inventory degraded to fit the request budget${modes.length > 0 ? `: ${modes.join(", ")}` : ""}.`;
}

function findEpic(snapshot: InventorySnapshot | null, epicFile: string): InventoryEpic | undefined {
  return (snapshot?.epics ?? []).find((epic) => epic.file === epicFile);
}

function findStory(snapshot: InventorySnapshot | null, epicFile: string, position: number): InventoryStory | undefined {
  return findEpic(snapshot, epicFile)?.stories.find((story) => story.position === position);
}

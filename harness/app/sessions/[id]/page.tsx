"use client";

import Link from "next/link";
import { useParams } from "next/navigation";
import { useCallback, useEffect, useRef, useState } from "react";

import { EpicsTable } from "../../components/EpicsTable";
import { InventoryChips } from "../../components/InventoryChips";
import { StoryModal } from "../../components/StoryModal";
import { postJson, usePoll } from "../../lib/use-poll";
import {
  documentChips,
  inventoryStale,
  inventoryTruncated,
  pathMismatch,
  reconcileInventoryGeneration,
  statusTone,
} from "../../lib/view-model/inventory";
import { controlsDisabled, inventoryFreshness, siblingWarningVisible } from "../../lib/view-model/session";
import { outcomeBadge } from "../../lib/view-model/run";
import type {
  InventoryEpic,
  InventorySnapshot,
  InventoryStory,
  RunSummary,
  SessionSummary,
} from "../../lib/view-model/types";

interface SessionStatus extends SessionSummary {
  runs: RunSummary[];
}

interface SessionDetailWire extends SessionSummary {
  runs: RunSummary[];
  inventory: InventorySnapshot | null;
}

interface LoadedInventory {
  generation: number;
  snapshot: InventorySnapshot | null;
}

/** Formats an inventory age (ms) as a compact `Ns` / `Nm` string. */
function formatAge(ms: number | null): string {
  if (ms === null) {
    return "never";
  }
  const seconds = Math.floor(ms / 1000);

  return seconds < 60 ? `${seconds}s ago` : `${Math.floor(seconds / 60)}m ago`;
}

interface SelectedStory {
  epicFile: string;
  position: number;
}

/**
 * Session detail (plan §3.5, view 2) reworked as an epic→story accordion
 * with a native-dialog story review modal (plan §3). Transport (plan §2):
 * the page polls the LIGHTWEIGHT status view at 1 Hz for reachability /
 * active run / run history / generation, and refetches the inventory
 * snapshot ONLY when the promoted generation changes (retry-until-match,
 * never regressing on a reordered response). Visual pass: Peter's S&F system.
 */
export default function SessionDetailPage() {
  const params = useParams<{ id: string }>();
  const sessionId = params.id;

  const status = usePoll<SessionStatus>(`/api/sessions/${sessionId}?view=status`);
  const sessionsData = usePoll<{ sessions: SessionSummary[] }>("/api/sessions");
  const [buildFix, setBuildFix] = useState(false);
  const [loaded, setLoaded] = useState<LoadedInventory>({ generation: 0, snapshot: null });
  const [collapsed, setCollapsed] = useState<Set<string>>(new Set());
  const [selected, setSelected] = useState<SelectedStory | null>(null);
  const [modalTab, setModalTab] = useState<"review" | "raw">("review");
  const openerRef = useRef<HTMLElement | null>(null);
  // The last live (story, epic) the open modal presented. When a fresher
  // snapshot no longer resolves the selected composite identity — the story
  // was removed/renamed on disk — the modal stays OPEN on this retained
  // presentation as a changed-on-disk fallback instead of silently
  // unmounting (finding 4).
  const [retained, setRetained] = useState<{ story: InventoryStory; epic: InventoryEpic } | null>(null);

  // A collapsed set + loaded inventory + open modal are per-session, in-page
  // only: reset them when the id changes, using React's adjust-state-during-
  // render pattern (never a setState-in-effect cascade).
  const [prevSessionId, setPrevSessionId] = useState(sessionId);
  if (prevSessionId !== sessionId) {
    setPrevSessionId(sessionId);
    setCollapsed(new Set());
    setLoaded({ generation: 0, snapshot: null });
    setSelected(null);
    setRetained(null);
  }

  // Refetch the inventory snapshot ONLY when the promoted generation is ahead
  // of what is loaded (plan §2/§2a). The status poll changes reference every
  // tick, so this retries until caught up; a reordered/older response never
  // regresses the loaded generation.
  const targetGeneration = status?.inventory_generation ?? 0;
  useEffect(() => {
    if (!inventoryStale(targetGeneration, loaded.generation)) {
      return;
    }

    let alive = true;
    void fetch(`/api/sessions/${sessionId}`, { cache: "no-store" })
      .then((response) => (response.ok ? (response.json() as Promise<SessionDetailWire>) : null))
      .then((detail) => {
        if (!alive || detail === null) {
          return;
        }
        setLoaded((prev) => {
          const generation = reconcileInventoryGeneration(prev.generation, detail.inventory_generation);
          if (generation <= prev.generation && prev.snapshot !== null) {
            return prev;
          }

          return { generation, snapshot: detail.inventory };
        });
      })
      .catch(() => {
        // Keep the last good snapshot on a transient failure.
      });

    return () => {
      alive = false;
    };
  }, [status, targetGeneration, loaded.generation, sessionId]);

  const inventory = loaded.snapshot;
  const chips = documentChips(inventory);
  const disabled = controlsDisabled(status);
  const siblingWarn = siblingWarningVisible(sessionsData?.sessions, status ?? null);
  const freshness = inventoryFreshness(status);
  // The out-of-band cannot-fit signal (finding 1): reported on the status
  // poll when the server limit is too small for even a floor request, so
  // there is no snapshot to carry the `unavailable` state itself.
  const statusUnavailable = (status?.inventory_unavailable ?? "").length > 0;
  const truncated = inventoryTruncated(inventory) || statusUnavailable;

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

  async function refresh(): Promise<void> {
    await postJson(`/api/sessions/${sessionId}/refresh`, {});
  }

  async function dispatchBuild(command: "build-tests" | "build-code"): Promise<void> {
    await postJson(`/api/sessions/${sessionId}/runs`, {
      command,
      fix: buildFix,
      client_token: crypto.randomUUID(),
    });
  }

  const selectedEpic = selected ? findEpic(inventory, selected.epicFile) : undefined;
  const selectedStory = selected ? findStory(inventory, selected.epicFile, selected.position) : undefined;

  // Retain the last LIVE presentation so a story that vanishes from a fresher
  // snapshot keeps the modal open as a changed-on-disk fallback (finding 4).
  // Uses the same adjust-state-during-render pattern as the id-change reset:
  // the guards make it converge (the resolved story/epic are stable object
  // references while the snapshot is unchanged), so it never loops. A vanished
  // story matches neither branch, leaving the last retained value in place.
  if (selected && selectedStory !== undefined && selectedEpic !== undefined) {
    if (retained?.story !== selectedStory || retained?.epic !== selectedEpic) {
      setRetained({ story: selectedStory, epic: selectedEpic });
    }
  } else if (!selected && retained !== null) {
    setRetained(null);
  }

  // Fall back to the retained presentation (marked changed-on-disk) when the
  // selected identity no longer resolves, so the modal never silently
  // disappears and never surprise-reopens (finding 4).
  const modalPresentation =
    selected && selectedStory && selectedEpic
      ? { story: selectedStory, epic: selectedEpic, changedOnDisk: false }
      : selected && retained
        ? { story: retained.story, epic: retained.epic, changedOnDisk: true }
        : null;

  return (
    <main className="sf-session">
      <p className="sf-crumb">
        <Link href="/">← Sessions</Link>
      </p>

      <header className="sf-header">
        <h1 className="sf-title">{status?.folder ?? sessionId}</h1>
        <div className="sf-meta">
          <span className="sf-chip" data-testid="session-reachability" data-tone={statusTone(status?.reachability ?? "")}>
            <span className="sq" />
            {status?.reachability ?? "unknown"}
          </span>
          <span>
            <span className="sf-meta-label">Generation</span>
            <span data-testid="inventory-generation">{status?.inventory_generation ?? 0}</span>
          </span>
          <span>
            <span className="sf-meta-label">Updated</span>
            <span data-testid="inventory-updated-at" data-stale={freshness.stale ? "true" : "false"}>
              {formatAge(freshness.ageMs)}
            </span>
          </span>
          <button type="button" className="sf-btn sf-btn-sm" data-testid="refresh" onClick={() => void refresh()}>
            Refresh
          </button>
        </div>
      </header>

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
          {truncationText(inventory, statusUnavailable)}
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
        <button type="button" className="sf-btn" data-testid="action-build-tests" disabled={disabled} onClick={() => void dispatchBuild("build-tests")}>
          build tests
        </button>
        <button type="button" className="sf-btn" data-testid="action-build-code" disabled={disabled} onClick={() => void dispatchBuild("build-code")}>
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
      <RunHistory runs={status?.runs ?? []} />

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

function RunHistory({ runs }: { runs: RunSummary[] }) {
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
            <tr key={run.id} data-testid="run-row" data-run-id={run.id} data-command={run.command}>
              <td>
                <Link href={`/runs/${run.id}`}>{run.command}</Link>
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
function truncationText(snapshot: InventorySnapshot | null, statusUnavailable = false): string {
  if (statusUnavailable || (snapshot !== null && (snapshot.unavailable ?? "").length > 0)) {
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
  const contentOmitted = (snapshot.epics ?? []).some((epic) => (epic.stories ?? []).some((story) => story.content_omitted));
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

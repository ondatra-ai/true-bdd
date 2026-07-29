"use client";

import Link from "next/link";
import { useParams } from "next/navigation";
import { useState } from "react";

import { EpicsTable } from "../../components/EpicsTable";
import { InventoryChips } from "../../components/InventoryChips";
import { banner, button, cell, h1, h2, page, statusColor, table } from "../../components/styles";
import { postJson, usePoll } from "../../lib/use-poll";
import { documentChips, epicRows, pathMismatch } from "../../lib/view-model/inventory";
import { controlsDisabled, inventoryFreshness, siblingWarningVisible } from "../../lib/view-model/session";
import { outcomeBadge } from "../../lib/view-model/run";
import type { SessionDetail, SessionSummary } from "../../lib/view-model/types";

/** Formats an inventory age (ms) as a compact `Ns` / `Nm` string. */
function formatAge(ms: number | null): string {
  if (ms === null) {
    return "never";
  }
  const seconds = Math.floor(ms / 1000);

  return seconds < 60 ? `${seconds}s ago` : `${Math.floor(seconds / 60)}m ago`;
}

/**
 * Session detail (plan §3.5, view 2): inventory chips + errors, the
 * architecture path-mismatch warning, the epics/stories table with the
 * per-story actions, the session-level build actions, a Refresh control,
 * and the run history. Controls disable ONLY for this session's own
 * active run or an unreachable session; a sibling session on the same
 * folder shows a warning banner while controls stay enabled (P5).
 */
export default function SessionDetailPage() {
  const params = useParams<{ id: string }>();
  const sessionId = params.id;

  const detail = usePoll<SessionDetail>(`/api/sessions/${sessionId}`);
  const sessionsData = usePoll<{ sessions: SessionSummary[] }>("/api/sessions");
  const [buildFix, setBuildFix] = useState(false);

  const inventory = detail?.inventory ?? null;
  const chips = documentChips(inventory);
  const epics = epicRows(inventory);
  const disabled = controlsDisabled(detail);
  const siblingWarn = siblingWarningVisible(sessionsData?.sessions, detail);
  const freshness = inventoryFreshness(detail);

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

  return (
    <main style={page}>
      <p style={{ margin: 0 }}>
        <Link href="/">&larr; Sessions</Link>
      </p>
      <h1 style={h1}>{detail?.folder ?? sessionId}</h1>

      <p style={{ margin: "0 0 0.5rem" }}>
        Reachability:{" "}
        <span
          data-testid="session-reachability"
          style={{ color: statusColor(detail?.reachability === "connected" ? "present" : "invalid") }}
        >
          {detail?.reachability ?? "unknown"}
        </span>
        {" · "}Inventory generation: <span data-testid="inventory-generation">{detail?.inventory_generation ?? 0}</span>
        {" · "}updated{" "}
        <span data-testid="inventory-updated-at" data-stale={freshness.stale ? "true" : "false"}>
          {formatAge(freshness.ageMs)}
        </span>
        <button type="button" data-testid="refresh" onClick={() => void refresh()} style={{ ...button, marginLeft: "0.75rem" }}>
          Refresh
        </button>
      </p>

      {pathMismatch(inventory) ? (
        <div data-testid="path-mismatch-warning" style={banner}>
          Architecture path mismatch: scanning the configured{" "}
          <code>{inventory?.configured_architecture_path}</code> instead of the canonical default{" "}
          <code>{inventory?.canonical_architecture_path}</code>.
        </div>
      ) : null}

      {siblingWarn ? (
        <div data-testid="folder-warning-banner" style={banner}>
          Another session on this folder has an active run. The host-side flock enforces mutual exclusion; a
          dispatch here may fail fast with <code>error(folder_locked)</code>.
        </div>
      ) : null}

      <h2 style={h2}>Inventory</h2>
      <InventoryChips chips={chips} />

      <h2 style={h2}>Build pipeline</h2>
      <div style={{ display: "flex", gap: 8, alignItems: "center", flexWrap: "wrap" }}>
        <button type="button" data-testid="action-build-tests" disabled={disabled} onClick={() => void dispatchBuild("build-tests")} style={button}>
          build tests
        </button>
        <button type="button" data-testid="action-build-code" disabled={disabled} onClick={() => void dispatchBuild("build-code")} style={button}>
          build code
        </button>
        <label style={{ display: "inline-flex", alignItems: "center", gap: 3 }}>
          <input type="checkbox" checked={buildFix} onChange={(event) => setBuildFix(event.target.checked)} />
          --fix
        </label>
      </div>

      <h2 style={h2}>Stories</h2>
      <EpicsTable epics={epics} sessionId={sessionId} disabled={disabled} />

      <h2 style={h2}>Run history</h2>
      <RunHistory runs={detail?.runs ?? []} />
    </main>
  );
}

function RunHistory({ runs }: { runs: SessionDetail["runs"] }) {
  if (runs.length === 0) {
    return <p style={{ color: "#888" }}>No runs yet.</p>;
  }

  return (
    <div data-testid="run-history" style={{ overflowX: "auto" }}>
      <table style={table}>
        <thead>
          <tr>
            <th style={cell}>command</th>
            <th style={cell}>story</th>
            <th style={cell}>fix</th>
            <th style={cell}>state</th>
            <th style={cell}>outcome</th>
          </tr>
        </thead>
        <tbody>
          {runs.map((run) => (
            <tr key={run.id} data-testid="run-row" data-run-id={run.id} data-command={run.command}>
              <td style={cell}>
                <Link href={`/runs/${run.id}`}>{run.command}</Link>
              </td>
              <td style={cell}>{run.story_id ?? "—"}</td>
              <td style={cell}>{run.fix ? "--fix" : ""}</td>
              <td style={cell}>{run.state}</td>
              <td style={{ ...cell, color: statusColor(run.outcome === "error" ? "invalid" : "present") }}>{outcomeBadge(run)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

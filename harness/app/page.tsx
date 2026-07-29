"use client";

import Link from "next/link";

import { postJson, usePoll } from "./lib/use-poll";
import { inventoryFreshness } from "./lib/view-model/session";
import type { SessionSummary } from "./lib/view-model/types";
import { button, cell, h1, page, statusColor, table } from "./components/styles";

/** Formats an inventory age (ms) as a compact `Ns` / `Nm` string. */
function formatAge(ms: number | null): string {
  if (ms === null) {
    return "never";
  }
  const seconds = Math.floor(ms / 1000);

  return seconds < 60 ? `${seconds}s` : `${Math.floor(seconds / 60)}m`;
}

/**
 * Sessions list (plan §3.5, view 1): one row per connected remote with
 * its canonical folder, reachability, active run, inventory age, and a
 * per-row "Test connection" control that dispatches a `version` run (P8).
 */
export default function SessionsPage() {
  const data = usePoll<{ sessions: SessionSummary[] }>("/api/sessions");
  const sessions = data?.sessions ?? [];

  async function testConnection(sessionId: string): Promise<void> {
    await postJson(`/api/sessions/${sessionId}/runs`, {
      command: "version",
      fix: false,
      client_token: crypto.randomUUID(),
    });
  }

  return (
    <main style={page}>
      <h1 style={h1}>TrueBDD Harness — Sessions</h1>
      {sessions.length === 0 ? <p style={{ color: "#888" }}>No connected remotes yet.</p> : null}
      {sessions.length > 0 ? (
        <div style={{ overflowX: "auto" }}>
          <table style={table}>
            <thead>
              <tr>
                <th style={cell}>folder</th>
                <th style={cell}>pid</th>
                <th style={cell}>reachability</th>
                <th style={cell}>active run</th>
                <th style={cell}>inv. gen</th>
                <th style={cell}>inv. age</th>
                <th style={cell}>connection</th>
              </tr>
            </thead>
            <tbody>
              {sessions.map((session) => (
                <tr key={session.id} data-testid="session-row" data-session-id={session.id} data-folder={session.folder}>
                  <td style={cell}>
                    <Link href={`/sessions/${session.id}`} data-testid="session-folder">
                      {session.folder}
                    </Link>
                  </td>
                  <td style={cell}>{session.pid}</td>
                  <td style={{ ...cell, color: statusColor(session.reachability === "connected" ? "present" : "invalid") }}>
                    <span data-testid="session-reachability">{session.reachability}</span>
                  </td>
                  <td style={cell}>
                    {session.active_run_id ? (
                      <Link href={`/runs/${session.active_run_id}`}>active</Link>
                    ) : (
                      <span style={{ color: "#888" }}>idle</span>
                    )}
                  </td>
                  <td style={cell}>{session.inventory_generation}</td>
                  <td style={cell}>
                    {(() => {
                      const freshness = inventoryFreshness(session);

                      return (
                        <span data-testid="session-inventory-age" data-stale={freshness.stale ? "true" : "false"}>
                          {formatAge(freshness.ageMs)}
                        </span>
                      );
                    })()}
                  </td>
                  <td style={cell}>
                    <button type="button" data-testid="test-connection" onClick={() => void testConnection(session.id)} style={button}>
                      Test connection
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      ) : null}
    </main>
  );
}

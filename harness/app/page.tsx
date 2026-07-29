"use client";

import Link from "next/link";

import { postJson, usePoll } from "./lib/use-poll";
import { inventoryFreshness } from "./lib/view-model/session";
import type { SessionSummary } from "./lib/view-model/types";

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
    <main className="sf-session">
      <div className="sf-header">
        <h1 className="sf-title">TrueBDD Harness</h1>
        <p className="sf-meta">
          <span className="sf-meta-label">Connected remotes</span> {sessions.length}
        </p>
      </div>
      <h2 className="sf-section-label">
        <span className="num">01</span>—Sessions
      </h2>
      {sessions.length === 0 ? <p className="sf-empty">No connected remotes yet.</p> : null}
      {sessions.length > 0 ? (
        <div style={{ overflowX: "auto" }}>
          <table className="sf-stories">
            <thead>
              <tr>
                <th>folder</th>
                <th>pid</th>
                <th>reachability</th>
                <th>active run</th>
                <th>inv. gen</th>
                <th>inv. age</th>
                <th>connection</th>
              </tr>
            </thead>
            <tbody>
              {sessions.map((session) => (
                <tr key={session.id} data-testid="session-row" data-session-id={session.id} data-folder={session.folder}>
                  <td>
                    <Link href={`/sessions/${session.id}`} data-testid="session-folder">
                      {session.folder}
                    </Link>
                  </td>
                  <td>{session.pid}</td>
                  <td>
                    <span
                      className="sf-chip"
                      data-tone={session.reachability === "connected" ? "ok" : "error"}
                      data-testid="session-reachability"
                    >
                      <span className="sq" />
                      {session.reachability}
                    </span>
                  </td>
                  <td>
                    {session.active_run_id ? (
                      <Link href={`/runs/${session.active_run_id}`}>active</Link>
                    ) : (
                      <span className="sf-muted">idle</span>
                    )}
                  </td>
                  <td>{session.inventory_generation}</td>
                  <td>
                    {(() => {
                      const freshness = inventoryFreshness(session);

                      return (
                        <span data-testid="session-inventory-age" data-stale={freshness.stale ? "true" : "false"}>
                          {formatAge(freshness.ageMs)}
                        </span>
                      );
                    })()}
                  </td>
                  <td>
                    <button
                      type="button"
                      data-testid="test-connection"
                      onClick={() => void testConnection(session.id)}
                      className="sf-btn sf-btn-sm"
                    >
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

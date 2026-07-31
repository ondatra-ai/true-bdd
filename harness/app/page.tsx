"use client";

import Link from "next/link";

import { postJson, usePoll } from "./lib/use-poll";
import type { SessionSummary } from "./lib/view-model/types";

/**
 * Sessions list (plan §4, view 1): one row per CONNECTED remote (every
 * listed session is connected by definition — v2 has no reachability, no
 * active-run, no inventory generation). Each row shows the canonical folder
 * (realpath) and the remote version, plus a per-row "Test connection"
 * control that dispatches a `version` run (P8).
 */
export default function SessionsPage() {
  const { data } = usePoll<{ sessions: SessionSummary[] }>("/api/sessions");
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
                <th>version</th>
                <th>connection</th>
              </tr>
            </thead>
            <tbody>
              {sessions.map((session) => (
                <tr
                  key={session.session_id}
                  data-testid="session-row"
                  data-session-id={session.session_id}
                  data-folder={session.folder}
                >
                  <td>
                    <Link href={`/sessions/${session.session_id}`} data-testid="session-folder">
                      {session.folder}
                    </Link>
                  </td>
                  <td>{session.pid}</td>
                  <td data-testid="session-version">{session.version}</td>
                  <td>
                    <button
                      type="button"
                      data-testid="test-connection"
                      onClick={() => void testConnection(session.session_id)}
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

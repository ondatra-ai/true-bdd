"use client";

import Link from "next/link";
import { useParams } from "next/navigation";

import { PromptDialog } from "../../../../components/PromptDialog";
import { RunOutput } from "../../../../components/RunOutput";
import { statusColor } from "../../../../components/styles";
import { postJson, usePoll } from "../../../../lib/use-poll";
import { envelopeDetails, outcomeBadge, promptView, runOutputSegments } from "../../../../lib/view-model/run";
import type { RunDetail } from "../../../../lib/view-model/types";

/**
 * Run view (plan §4, view 3) — SESSION-SCOPED (`/sessions/:sid/runs/:rid`).
 * One live poll of GET /api/sessions/:sid/runs/:rid (RunDetail). Renders the
 * output tail (with retention gap markers), the run state + outcome badge,
 * the terminal-envelope diagnostics once terminal, and — while a prompt is
 * pending — the native `<dialog>` prompt modal. Answers go to
 * POST /api/sessions/:sid/runs/:rid/answer; a failed RPC keeps the dialog
 * open with a visible error (plan §4).
 */
export default function RunPage() {
  const params = useParams<{ id: string; rid: string }>();
  const sessionId = params.id;
  const runId = params.rid;

  const { data: run, status } = usePoll<RunDetail>(`/api/sessions/${sessionId}/runs/${runId}`);

  const segments = runOutputSegments(run?.events);
  const prompt = promptView(run?.pending_prompt);
  const envelope = envelopeDetails(run);
  const terminal = run?.state === "terminal";

  async function answer(promptId: string, value: string): Promise<{ ok: boolean; error: string | null }> {
    try {
      const response = await postJson(`/api/sessions/${sessionId}/runs/${runId}/answer`, {
        prompt_id: promptId,
        value,
      });
      if (response.ok) {
        return { ok: true, error: null };
      }

      const text = (await response.text()).slice(0, 300);

      return { ok: false, error: text.length > 0 ? text : `The answer was rejected (${response.status}).` };
    } catch {
      return { ok: false, error: "The answer request failed — the network may be down." };
    }
  }

  if (status === "session_gone") {
    return (
      <main className="sf-session">
        <p className="sf-crumb">
          <Link href={`/sessions/${sessionId}`}>← Session</Link>
        </p>
        <div className="sf-banner" data-tone="error" data-testid="unavailable-state">
          This remote has disconnected — its session is gone. This run is no longer reachable through it.
        </div>
      </main>
    );
  }

  if (status === "run_gone") {
    // The session is still connected — only THIS run is gone (pruned / never
    // existed). Do not imply a disconnect.
    return (
      <main className="sf-session">
        <p className="sf-crumb">
          <Link href={`/sessions/${sessionId}`}>← Session</Link>
        </p>
        <div className="sf-banner" data-tone="warn" data-testid="run-gone-state">
          This run is no longer available — it was pruned or never existed. The session is still connected.
        </div>
      </main>
    );
  }

  return (
    <main className="sf-session">
      <p className="sf-crumb">
        <Link href={`/sessions/${sessionId}`}>← Session</Link>
      </p>
      <h1 className="sf-title">Run: {run?.command ?? runId}</h1>

      <p className="sf-meta">
        <span>
          <span className="sf-meta-label">State</span>
          <span data-testid="run-state">{run?.state ?? ""}</span>
        </span>
        <span>
          <span className="sf-meta-label">Outcome</span>
          <span
            data-testid="run-outcome"
            style={{ color: statusColor(run?.outcome === "error" ? "invalid" : "present") }}
          >
            {outcomeBadge(run)}
          </span>
        </span>
      </p>

      {status === "unavailable" ? (
        <div className="sf-banner" data-tone="warn" data-testid="unavailable-state">
          The CLI did not respond in time — showing the last known state, which may be out of date.
        </div>
      ) : null}

      <RunOutput segments={segments} />

      {terminal && envelope.present ? (
        <p className="sf-meta" style={{ marginTop: "0.75rem" }}>
          <span>
            <span className="sf-meta-label">engine</span>
            <span data-testid="run-envelope-engine-outcome">{envelope.engineOutcome ?? "—"}</span>
          </span>
          <span>
            <span className="sf-meta-label">finalization</span>
            <span data-testid="run-envelope-finalization">
              {envelope.finalizationOk === null ? "—" : envelope.finalizationOk ? "ok" : "failed"}
            </span>
          </span>
          {envelope.exitCode !== null ? (
            <span>
              <span className="sf-meta-label">exit</span>
              <span data-testid="run-envelope-exit-code">{envelope.exitCode}</span>
            </span>
          ) : null}
          {envelope.signal !== null ? (
            <span>
              <span className="sf-meta-label">signal</span>
              <span data-testid="run-envelope-signal">{envelope.signal}</span>
            </span>
          ) : null}
        </p>
      ) : null}

      {prompt !== null && run?.answerable === true ? (
        <PromptDialog key={prompt.promptId} prompt={prompt} onAnswer={answer} />
      ) : null}
    </main>
  );
}

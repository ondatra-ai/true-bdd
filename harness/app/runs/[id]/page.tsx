"use client";

import Link from "next/link";
import { useParams } from "next/navigation";

import { PromptPanel } from "../../components/PromptPanel";
import { RunOutput } from "../../components/RunOutput";
import { banner, button, h1, page, statusColor } from "../../components/styles";
import { postJson, usePoll } from "../../lib/use-poll";
import {
  envelopeDetails,
  markAbandonedVisible,
  outcomeBadge,
  promptView,
  reachabilityOverlayVisible,
  runOutputSegments,
  runTiming,
} from "../../lib/view-model/run";
import type { RunDetail, SessionSummary } from "../../lib/view-model/types";

/** Formats a millisecond duration as a compact `Ns` / `Nm Ns` string. */
function formatDuration(ms: number | null): string {
  if (ms === null) {
    return "—";
  }
  const seconds = Math.floor(ms / 1000);
  if (seconds < 60) {
    return `${seconds}s`;
  }

  return `${Math.floor(seconds / 60)}m ${seconds % 60}s`;
}

/**
 * Run view (plan §3.5, view 3): the output tail, the prompt panel by
 * kind, the run state + outcome-envelope badge, and — while the owning
 * session is unreachable and the run non-terminal — the reachability
 * overlay and the "Mark abandoned" control (P2). The owning session's
 * reachability is read from GET /api/sessions/:id via run.session_id.
 */
export default function RunPage() {
  const params = useParams<{ id: string }>();
  const runId = params.id;

  const run = usePoll<RunDetail>(`/api/runs/${runId}`);
  const session = usePoll<SessionSummary>(run?.session_id ? `/api/sessions/${run.session_id}` : null);

  const segments = runOutputSegments(run?.events);
  const prompt = promptView(run?.pending_prompt);
  const overlay = reachabilityOverlayVisible(run, session);
  const canAbandon = markAbandonedVisible(run, session);
  const timing = runTiming(run);
  const envelope = envelopeDetails(run);

  async function answer(promptId: string, value: string): Promise<void> {
    await postJson(`/api/runs/${runId}/answer`, { prompt_id: promptId, value });
  }

  async function abandon(): Promise<void> {
    await postJson(`/api/runs/${runId}/abandon`, {});
  }

  return (
    <main style={page}>
      <p style={{ margin: 0 }}>
        <Link href={run ? `/sessions/${run.session_id}` : "/"}>&larr; Session</Link>
      </p>
      <h1 style={h1}>Run: {run?.command ?? runId}</h1>

      <p style={{ margin: "0 0 0.5rem" }}>
        State: <span data-testid="run-state">{run?.state ?? ""}</span>
        {" · "}Outcome:{" "}
        <span data-testid="run-outcome" style={{ color: statusColor(run?.outcome === "error" ? "invalid" : "present") }}>
          {outcomeBadge(run)}
        </span>
      </p>

      <p style={{ margin: "0 0 0.5rem", color: "#888", fontSize: "0.85rem" }}>
        Elapsed <span data-testid="run-elapsed">{formatDuration(timing.elapsedMs)}</span>
        {" · "}Last activity <span data-testid="run-last-activity">{formatDuration(timing.lastActivityMs)} ago</span>
        {envelope.present ? (
          <>
            {" · "}engine{" "}
            <span data-testid="run-envelope-engine-outcome">{envelope.engineOutcome ?? "—"}</span>
            {" · "}finalization{" "}
            <span data-testid="run-envelope-finalization">
              {envelope.finalizationOk === null ? "—" : envelope.finalizationOk ? "ok" : "failed"}
            </span>
            {envelope.exitCode !== null ? (
              <>
                {" · "}exit <span data-testid="run-envelope-exit-code">{envelope.exitCode}</span>
              </>
            ) : null}
            {envelope.signal !== null ? (
              <>
                {" · "}signal <span data-testid="run-envelope-signal">{envelope.signal}</span>
              </>
            ) : null}
          </>
        ) : null}
      </p>

      {overlay ? (
        <div data-testid="reachability-overlay" style={banner}>
          unknown — remote unreachable. The run state is frozen until the remote reconnects.
        </div>
      ) : null}

      {canAbandon ? (
        <p style={{ margin: "0 0 0.75rem" }}>
          <button type="button" data-testid="mark-abandoned" onClick={() => void abandon()} style={button}>
            Mark abandoned
          </button>
        </p>
      ) : null}

      <RunOutput segments={segments} />

      <PromptPanel prompt={prompt} onAnswer={answer} />
    </main>
  );
}

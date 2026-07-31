/**
 * Pure view-model for the run view (plan §4). Maps a RunDetail into render
 * props for the outcome badge, the streaming output tail with retention gap
 * markers, the terminal envelope diagnostics, and the prompt dialog — no
 * React, so the vitest tables (§4.6) cover every terminal envelope form
 * against the identical mapping.
 *
 * v2 (plan §6): sessions are gone on disconnect, so the reachability overlay
 * and the mark-abandoned control are DELETED — there is no unreachable state.
 */

import type { PendingPrompt, RunEvent, TerminalEnvelope } from "./types";

/**
 * The run-outcome badge text (README-testids): empty until terminal, then
 * the bare outcome, or `error(<detail>)` for the error class. Covers
 * ok | converged | not_fixed | user_exit | max_attempts | interrupted |
 * abandoned and error(spawn|no_result|contradiction|folder_locked).
 */
export function outcomeBadge(
  run: { outcome?: RunOutcomeLike; error_detail?: string | null } | null | undefined,
): string {
  if (!run || run.outcome === null || run.outcome === undefined) {
    return "";
  }

  if (run.outcome === "error") {
    return `error(${run.error_detail ?? ""})`;
  }

  return String(run.outcome);
}

/** The badge accepts any string outcome so a future state is never masked. */
type RunOutcomeLike = string | null;

/** A run is non-terminal until its state reaches `terminal`. */
export function isNonTerminal(run: { state?: string } | null | undefined): boolean {
  return run !== null && run !== undefined && run.state !== "terminal";
}

// ── Terminal envelope details (plan §3.2 / finding 7) ──

export interface EnvelopeDetailView {
  /** Whether any diagnostic beyond the classification badge is available. */
  present: boolean;
  engineOutcome: string | null;
  finalizationOk: boolean | null;
  exitCode: number | null;
  signal: string | null;
}

/**
 * The full terminal envelope for the run view: the badge stays the
 * classification (outcomeBadge), while these diagnostics — engine outcome,
 * finalization success, process exit code / signal — are exposed alongside
 * so a converged story whose final write failed is legible rather than
 * flattened to `error(contradiction)`.
 */
export function envelopeDetails(
  run: { envelope?: TerminalEnvelope | null } | null | undefined,
): EnvelopeDetailView {
  const envelope: TerminalEnvelope | null | undefined = run?.envelope;
  if (!envelope) {
    return { present: false, engineOutcome: null, finalizationOk: null, exitCode: null, signal: null };
  }

  const present =
    envelope.engine_outcome !== null ||
    envelope.finalization_ok !== null ||
    envelope.exit_code !== null ||
    envelope.signal !== null;

  return {
    present,
    engineOutcome: envelope.engine_outcome ?? null,
    finalizationOk: envelope.finalization_ok ?? null,
    exitCode: envelope.exit_code ?? null,
    signal: envelope.signal ?? null,
  };
}

// ── Run timing (plan §3.3/§3.5 / finding 8) ──

export interface RunTimingView {
  /** ms since the run was dispatched, or null when unknown. */
  elapsedMs: number | null;
  /** ms since the run's last activity (state change), or null. */
  lastActivityMs: number | null;
}

/**
 * The run's elapsed time and time-since-last-activity, computed against an
 * injectable clock so the view-model tables are deterministic. Both are
 * null when the store did not carry the timestamps (older wire shape).
 */
export function runTiming(
  run: { created_at?: number; updated_at?: number } | null | undefined,
  clockNow: number = Date.now(),
): RunTimingView {
  return {
    elapsedMs: typeof run?.created_at === "number" ? Math.max(0, clockNow - run.created_at) : null,
    lastActivityMs: typeof run?.updated_at === "number" ? Math.max(0, clockNow - run.updated_at) : null,
  };
}

// ── Output tail with retention gap markers (plan §3.6) ──

export interface OutputSegment {
  kind: "output" | "gap";
  seq: number;
  text: string;
}

/**
 * The rendered marker for a retention gap tombstone — a VISIBLE placeholder
 * so a viewer sees that a middle span was elided rather than a silent jump.
 */
export function gapMarker(event: RunEvent): string {
  const dropped = typeof event.dropped_bytes === "number" ? event.dropped_bytes : 0;
  const from = event.seq;
  const to = typeof event.through_seq === "number" ? event.through_seq : event.seq;

  return `⋯ ${dropped} bytes elided (seq ${from}–${to}) ⋯`;
}

/**
 * The output tail as ordered segments: `output` chunks carry their raw
 * data; a `gap` event becomes a visible elision marker. Non-output,
 * non-gap events (prompt/terminal) carry no tail text.
 */
export function runOutputSegments(events: RunEvent[] | undefined): OutputSegment[] {
  const segments: OutputSegment[] = [];
  for (const event of events ?? []) {
    if (event.type === "output") {
      segments.push({ kind: "output", seq: event.seq, text: String(event.data ?? "") });
    } else if (event.type === "gap") {
      segments.push({ kind: "gap", seq: event.seq, text: gapMarker(event) });
    }
  }

  return segments;
}

/** The flattened output tail text (segments joined). */
export function runOutputText(events: RunEvent[] | undefined): string {
  return runOutputSegments(events)
    .map((segment) => segment.text)
    .join("");
}

// ── Prompt dialog (plan §4 / §4.3 A0–A4) ──

export interface ChoicePromptView {
  kind: "choice";
  promptId: string;
  actions: string[];
}

export interface ClarifyPromptView {
  kind: "clarify";
  promptId: string;
  question: string;
  options: string[];
}

export interface FreetextPromptView {
  kind: "freetext";
  promptId: string;
  prompt: string;
}

export type PromptView = ChoicePromptView | ClarifyPromptView | FreetextPromptView | null;

/**
 * Normalizes a pending prompt into a typed view. Choice carries the
 * apply/refine/exit actions; clarify carries the question + numbered
 * options (answered by number); freetext carries the multiline prompt.
 */
export function promptView(prompt: PendingPrompt | null | undefined): PromptView {
  if (!prompt) {
    return null;
  }

  const payload = (prompt.payload ?? {}) as {
    actions?: unknown;
    question?: unknown;
    options?: unknown;
    prompt?: unknown;
  };

  if (prompt.kind === "choice") {
    const actions = Array.isArray(payload.actions) ? payload.actions.map(String) : ["apply", "refine", "exit"];

    return { kind: "choice", promptId: prompt.prompt_id, actions };
  }

  if (prompt.kind === "clarify") {
    const options = Array.isArray(payload.options) ? payload.options.map(String) : [];

    return {
      kind: "clarify",
      promptId: prompt.prompt_id,
      question: typeof payload.question === "string" ? payload.question : "",
      options,
    };
  }

  if (prompt.kind === "freetext") {
    return {
      kind: "freetext",
      promptId: prompt.prompt_id,
      prompt: typeof payload.prompt === "string" ? payload.prompt : "",
    };
  }

  return null;
}

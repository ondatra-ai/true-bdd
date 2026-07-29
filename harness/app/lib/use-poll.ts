"use client";

import { useEffect, useState } from "react";

/**
 * Honest browser poll state (plan §4, critique §13): `usePoll` surfaces a
 * discriminated {data, status, error} so a 404 session_gone CLEARS the data
 * (the page navigates away / shows disconnected) and a 504 cli_timeout marks
 * the last value not-current WITHOUT silently presenting it as fresh.
 */
export type PollStatus = "ok" | "session_gone" | "run_gone" | "unavailable" | "capacity" | "error";

export interface PollView<T> {
  data: T | undefined;
  status: PollStatus;
  error: string | null;
}

/** Run-scoped 404 reasons: the RUN is gone but the serving session is alive —
 * distinct from a `session_gone` disconnect (UI honesty). */
const RUN_GONE_REASONS = new Set(["run_gone", "run_pruned", "not_found"]);

/**
 * Maps a browser-facing HTTP status (+ optional CLI error reason) to the poll
 * status (critique §4). A 404 is `session_gone` by default, but a 404 whose body
 * names a RUN-scoped reason (run_pruned / not_found) is `run_gone` — the session
 * is still connected, only this run is gone.
 */
export function classifyPollStatus(httpStatus: number, reason?: string): PollStatus {
  if (httpStatus >= 200 && httpStatus < 300) {
    return "ok";
  }
  switch (httpStatus) {
    case 404:
      return reason !== undefined && RUN_GONE_REASONS.has(reason) ? "run_gone" : "session_gone";
    case 504:
      return "unavailable";
    case 429:
    case 503:
      return "capacity";
    default:
      return "error";
  }
}

/**
 * Folds a poll result into the honest next state (critique §13): `ok` adopts
 * the fresh data and clears the error; `session_gone` CLEARS the data (never a
 * ghost); `unavailable`/`capacity`/`error` KEEP the last data but mark it
 * not-current with an explicit status/error.
 */
export function nextPollState<T>(
  prev: PollView<T>,
  result: { httpStatus: number; data?: T; reason?: string },
): PollView<T> {
  const status = classifyPollStatus(result.httpStatus, result.reason);
  switch (status) {
    case "ok":
      return { data: result.data, status: "ok", error: null };
    case "session_gone":
      return { data: undefined, status, error: "session gone" };
    case "run_gone":
      // The RUN is gone (pruned / never existed) but the session is alive —
      // clear the run data without implying a disconnect.
      return { data: undefined, status, error: "this run is no longer available" };
    case "unavailable":
      return { data: prev.data, status, error: "CLI did not respond in time — showing the last known state" };
    case "capacity":
      return { data: prev.data, status, error: "the relay is at capacity — retrying" };
    default:
      return { data: prev.data, status: "error", error: "the request failed" };
  }
}

const IDLE: PollView<never> = { data: undefined, status: "ok", error: null };

/**
 * Live-polls a JSON endpoint on an interval and returns the honest
 * {data, status, error} view (plan §4). Each tick reads `response.status`
 * and folds it via `nextPollState`, so:
 *   - a 404 session_gone CLEARS the data (the page navigates away / shows a
 *     disconnected view);
 *   - a 504 cli_timeout KEEPS the last data but marks it not-current;
 *   - a network throw is treated as httpStatus 0 → status "error", last data
 *     retained.
 * A `null` url DISABLES polling and returns an idle {undefined, "ok", null}
 * — used when a URL depends on a value not yet loaded.
 *
 * `refreshSignal` forces an immediate live re-READ when it changes (the
 * Refresh button — a fresh CLI scan, not a mutation; plan §1.5).
 */
export function usePoll<T>(url: string | null, intervalMs = 1000, refreshSignal = 0): PollView<T> {
  const [view, setView] = useState<PollView<T>>(IDLE as PollView<T>);

  useEffect(() => {
    if (url === null) {
      return;
    }

    let alive = true;
    // The next interval is scheduled AFTER each fetch completes, so a slow
    // request never overlaps the next tick. A monotonic request id guards
    // against a late response applying stale state: only the LATEST issued
    // request may commit its result.
    let latest = 0;
    let timer: ReturnType<typeof setTimeout> | undefined;

    async function tick(): Promise<void> {
      const id = ++latest;
      // A network throw is httpStatus 0 → classifyPollStatus → "error",
      // which folds to keeping the last data with an explicit error.
      let result: { httpStatus: number; data?: T; reason?: string } = { httpStatus: 0 };
      try {
        const response = await fetch(url as string, { cache: "no-store" });
        if (response.ok) {
          result = { httpStatus: response.status, data: (await response.json()) as T };
        } else {
          // Read the CLI error reason so a run-scoped 404 (run_pruned/not_found)
          // is distinguished from a session_gone disconnect (UI honesty).
          result = { httpStatus: response.status, reason: await errorReason(response) };
        }
      } catch {
        result = { httpStatus: 0 };
      } finally {
        // `session_gone` / `run_gone` are terminal (a gone session/run never
        // returns under the same id) — commit the view and STOP polling so the
        // page does not spam the 404 endpoint forever.
        const settled = classifyPollStatus(result.httpStatus, result.reason);
        const terminal = settled === "session_gone" || settled === "run_gone";
        if (alive && id === latest) {
          setView((prev) => nextPollState(prev, result));
        }
        if (alive && !terminal) {
          timer = setTimeout(() => void tick(), intervalMs);
        }
      }
    }

    void tick();

    return () => {
      alive = false;
      if (timer !== undefined) {
        clearTimeout(timer);
      }
    };
  }, [url, intervalMs, refreshSignal]);

  // A null url is disabled: report a stable idle state without ever mutating
  // state inside the effect (the last polled view is discarded on disable).
  return url === null ? (IDLE as PollView<T>) : view;
}

/** Extracts the `{error}` reason from a non-2xx JSON body, or undefined. */
async function errorReason(response: Response): Promise<string | undefined> {
  try {
    const body = (await response.json()) as { error?: unknown };

    return typeof body.error === "string" ? body.error : undefined;
  } catch {
    return undefined;
  }
}

/** POSTs JSON to url, ignoring the response body. */
export async function postJson(url: string, body: unknown): Promise<Response> {
  return fetch(url, {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify(body),
  });
}

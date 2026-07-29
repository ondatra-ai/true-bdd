/**
 * Component/view-model tables (plan §4.6): every terminal envelope badge
 * (ok|converged|not_fixed|user_exit|max_attempts|interrupted|abandoned and
 * error(spawn|no_result|contradiction|folder_locked)), the run-output tail
 * with retention gap markers, the reachability-overlay / mark-abandoned
 * visibility rules, and the prompt-panel normalization. Pure functions; no
 * DOM.
 */

import { describe, expect, it } from "vitest";

import {
  envelopeDetails,
  gapMarker,
  markAbandonedVisible,
  outcomeBadge,
  promptView,
  reachabilityOverlayVisible,
  runOutputSegments,
  runOutputText,
  runTiming,
} from "../../app/lib/view-model/run";
import type { PendingPrompt, RunEvent, TerminalEnvelope } from "../../app/lib/view-model/types";

describe("outcomeBadge — every terminal envelope form", () => {
  it("is empty until terminal (null outcome)", () => {
    expect(outcomeBadge({ outcome: null })).toBe("");
    expect(outcomeBadge(null)).toBe("");
    expect(outcomeBadge(undefined)).toBe("");
  });

  const plain = ["ok", "converged", "not_fixed", "user_exit", "max_attempts", "interrupted", "abandoned"];
  for (const outcome of plain) {
    it(`renders the bare outcome '${outcome}'`, () => {
      expect(outcomeBadge({ outcome })).toBe(outcome);
    });
  }

  const errorDetails = ["spawn", "no_result", "contradiction", "folder_locked"];
  for (const detail of errorDetails) {
    it(`renders error(${detail})`, () => {
      expect(outcomeBadge({ outcome: "error", error_detail: detail })).toBe(`error(${detail})`);
    });
  }

  it("renders error() when the detail is absent", () => {
    expect(outcomeBadge({ outcome: "error" })).toBe("error()");
    expect(outcomeBadge({ outcome: "error", error_detail: null })).toBe("error()");
  });
});

describe("runOutputSegments — streaming + retention gap markers", () => {
  it("concatenates output chunks in order and ignores prompt/terminal events", () => {
    const events: RunEvent[] = [
      { seq: 1, type: "output", stream: "stdout", data: "true-bdd version " },
      { seq: 2, type: "output", stream: "stdout", data: "1.2.3\n" },
      { seq: 3, type: "prompt", prompt_id: "p1", kind: "choice" },
      { seq: 4, type: "terminal", outcome: "ok" },
    ];
    expect(runOutputText(events)).toBe("true-bdd version 1.2.3\n");
    expect(runOutputSegments(events).map((segment) => segment.kind)).toEqual(["output", "output"]);
  });

  it("renders a gap tombstone as a VISIBLE elision marker in the tail", () => {
    const events: RunEvent[] = [
      { seq: 1, type: "output", data: "head\n" },
      { seq: 2, type: "gap", through_seq: 40, dropped_bytes: 8192, retention: true },
      { seq: 41, type: "output", data: "tail\n" },
    ];
    const segments = runOutputSegments(events);
    expect(segments.map((segment) => segment.kind)).toEqual(["output", "gap", "output"]);

    const gap = segments[1];
    expect(gap.text).toContain("elided");
    expect(gap.text).toContain("8192");
    expect(gap.text).toContain("2");
    expect(gap.text).toContain("40");

    // The marker text is present in the flattened tail (visible, not silent).
    expect(runOutputText(events)).toContain("elided");
  });

  it("gapMarker formats bytes and the covered seq range", () => {
    expect(gapMarker({ seq: 5, type: "gap", through_seq: 12, dropped_bytes: 300 })).toBe(
      "⋯ 300 bytes elided (seq 5–12) ⋯",
    );
  });

  it("tolerates missing events", () => {
    expect(runOutputSegments(undefined)).toEqual([]);
    expect(runOutputText(undefined)).toBe("");
  });
});

describe("reachabilityOverlayVisible / markAbandonedVisible", () => {
  const cases: { state: string; reachability: string | null; visible: boolean; note: string }[] = [
    { state: "queued", reachability: "unreachable", visible: true, note: "non-terminal + unreachable" },
    { state: "running", reachability: "unreachable", visible: true, note: "non-terminal + unreachable" },
    { state: "prompt_published", reachability: "unreachable", visible: true, note: "non-terminal + unreachable" },
    { state: "queued", reachability: "connected", visible: false, note: "connected hides the overlay" },
    { state: "terminal", reachability: "unreachable", visible: false, note: "terminal never shows it" },
    { state: "running", reachability: null, visible: false, note: "no session ⇒ hidden" },
  ];

  for (const { state, reachability, visible, note } of cases) {
    it(`${note}: state=${state} reachability=${reachability} ⇒ ${visible}`, () => {
      const session = reachability === null ? null : { reachability: reachability as "connected" | "unreachable" };
      expect(reachabilityOverlayVisible({ state }, session)).toBe(visible);
      expect(markAbandonedVisible({ state }, session)).toBe(visible);
    });
  }

  it("is hidden when the run itself is undefined", () => {
    expect(reachabilityOverlayVisible(undefined, { reachability: "unreachable" })).toBe(false);
  });
});

describe("runTiming (finding 8; clock-injected)", () => {
  const clockNow = 1_000_000;

  it("computes elapsed and last-activity from the run timestamps", () => {
    expect(runTiming({ created_at: clockNow - 30_000, updated_at: clockNow - 5_000 }, clockNow)).toEqual({
      elapsedMs: 30_000,
      lastActivityMs: 5_000,
    });
  });

  it("clamps future timestamps and tolerates missing ones", () => {
    expect(runTiming({ created_at: clockNow + 10_000, updated_at: clockNow + 10_000 }, clockNow)).toEqual({
      elapsedMs: 0,
      lastActivityMs: 0,
    });
    expect(runTiming({}, clockNow)).toEqual({ elapsedMs: null, lastActivityMs: null });
    expect(runTiming(null, clockNow)).toEqual({ elapsedMs: null, lastActivityMs: null });
  });
});

describe("envelopeDetails (finding 7)", () => {
  it("exposes the full retained envelope alongside the classification badge", () => {
    const envelope: TerminalEnvelope = {
      classification: "error",
      engine_outcome: "converged",
      finalization_ok: false,
      exit_code: 3,
      signal: null,
    };
    expect(envelopeDetails({ envelope })).toEqual({
      present: true,
      engineOutcome: "converged",
      finalizationOk: false,
      exitCode: 3,
      signal: null,
    });
  });

  it("reports not-present when no envelope facts are set", () => {
    expect(envelopeDetails(null).present).toBe(false);
    expect(
      envelopeDetails({
        envelope: { classification: null, engine_outcome: null, finalization_ok: null, exit_code: null, signal: null },
      }).present,
    ).toBe(false);
  });
});

describe("promptView — prompt panel normalization", () => {
  it("normalizes a choice prompt with its actions", () => {
    const prompt: PendingPrompt = { prompt_id: "p1", kind: "choice", payload: { actions: ["apply", "refine", "exit"] } };
    expect(promptView(prompt)).toEqual({ kind: "choice", promptId: "p1", actions: ["apply", "refine", "exit"] });
  });

  it("defaults choice actions when the payload omits them", () => {
    const view = promptView({ prompt_id: "p1", kind: "choice", payload: {} });
    expect(view).toEqual({ kind: "choice", promptId: "p1", actions: ["apply", "refine", "exit"] });
  });

  it("normalizes a clarify prompt with question + numbered options", () => {
    const prompt: PendingPrompt = {
      prompt_id: "c1",
      kind: "clarify",
      payload: { question: "Which merge?", options: ["Merge A into B", "Keep both"] },
    };
    expect(promptView(prompt)).toEqual({
      kind: "clarify",
      promptId: "c1",
      question: "Which merge?",
      options: ["Merge A into B", "Keep both"],
    });
  });

  it("normalizes a freetext prompt with its prompt text", () => {
    const prompt: PendingPrompt = { prompt_id: "f1", kind: "freetext", payload: { prompt: "Enter feedback." } };
    expect(promptView(prompt)).toEqual({ kind: "freetext", promptId: "f1", prompt: "Enter feedback." });
  });

  it("returns null for no pending prompt or an unknown kind", () => {
    expect(promptView(null)).toBeNull();
    expect(promptView(undefined)).toBeNull();
    expect(promptView({ prompt_id: "x", kind: "mystery", payload: {} })).toBeNull();
  });
});

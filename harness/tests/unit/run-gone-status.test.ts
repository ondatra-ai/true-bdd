/**
 * UI honesty (NICE-TO-HAVE): a run-route 404 whose CLI body names a RUN-scoped
 * reason (run_pruned / not_found) must classify as `run_gone` — the session is
 * still connected, only this run is gone — NOT the blanket `session_gone` that
 * clears the page as a disconnect. A 404 with no reason (or session_gone) stays
 * session_gone, so the change is backward compatible.
 */

import { describe, expect, it } from "vitest";

import { classifyPollStatus, nextPollState } from "../../app/lib/use-poll";

describe("run-scoped 404 vs session_gone (UI honesty)", () => {
  it("a 404 with no reason stays session_gone (backward compatible)", () => {
    expect(classifyPollStatus(404)).toBe("session_gone");
  });

  it("a 404 whose body is a RUN-scoped reason is run_gone", () => {
    expect(classifyPollStatus(404, "run_pruned")).toBe("run_gone");
    expect(classifyPollStatus(404, "not_found")).toBe("run_gone");
    expect(classifyPollStatus(404, "run_gone")).toBe("run_gone");
  });

  it("a 404 explicitly session_gone stays session_gone", () => {
    expect(classifyPollStatus(404, "session_gone")).toBe("session_gone");
  });

  it("nextPollState(run_gone) clears the run data without implying a disconnect", () => {
    const next = nextPollState({ data: { n: 1 }, status: "ok", error: null }, { httpStatus: 404, reason: "run_pruned" });
    expect(next.status).toBe("run_gone");
    expect(next.data).toBeUndefined();
  });

  it("nextPollState(session_gone) still clears data", () => {
    const next = nextPollState({ data: { n: 1 }, status: "ok", error: null }, { httpStatus: 404 });
    expect(next.status).toBe("session_gone");
    expect(next.data).toBeUndefined();
  });
});

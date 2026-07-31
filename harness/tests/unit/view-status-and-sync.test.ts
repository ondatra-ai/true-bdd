/**
 * Client poll-state model (plan §4, FULLY REWRITTEN for v2). The v1 file
 * tested a SERVER-store `?view=status` read, an atomic generation+snapshot
 * pair, and client-side retry-until-match GENERATION logic. v2 deletes all
 * of that (plan §6): the relay holds no DB, `sessionStatus`/`sessionDetail`
 * come from the CLI store (Go tests), and there is NO generation.
 *
 * What remains client-side is the honest {data, status, error} poll model
 * (critique §13): `usePoll` returns a discriminated state, so a 404
 * session_gone CLEARS the data (the page navigates away / shows
 * disconnected) and a 504 cli_timeout renders an explicit unavailable state
 * WITHOUT silently presenting the last value as current. This suite pins the
 * two PURE helpers behind that behavior.
 *
 * The v2 helpers do not exist yet; they are accessed through a pinned cast
 * so the file typechecks and every table is RED until the hook is rebuilt.
 */

import { describe, expect, it } from "vitest";

import * as usePollMod from "../../app/lib/use-poll";

// ── Pinned v2 target surfaces ──

type PollStatus = "ok" | "session_gone" | "unavailable" | "capacity" | "error";

interface PollView<T> {
  data: T | undefined;
  status: PollStatus;
  error: string | null;
}

interface TargetPoll {
  classifyPollStatus: (httpStatus: number) => PollStatus;
  nextPollState: <T>(prev: PollView<T>, result: { httpStatus: number; data?: T }) => PollView<T>;
}

const poll = usePollMod as unknown as TargetPoll;

function state<T>(overrides: Partial<PollView<T>> = {}): PollView<T> {
  return { data: undefined, status: "ok", error: null, ...overrides };
}

describe("classifyPollStatus (browser status mapping — critique §4)", () => {
  it("200 → ok", () => {
    expect(poll.classifyPollStatus(200)).toBe("ok");
  });

  it("404 → session_gone", () => {
    expect(poll.classifyPollStatus(404)).toBe("session_gone");
  });

  it("504 → unavailable (cli_timeout)", () => {
    expect(poll.classifyPollStatus(504)).toBe("unavailable");
  });

  it("429 and 503 → capacity", () => {
    expect(poll.classifyPollStatus(429)).toBe("capacity");
    expect(poll.classifyPollStatus(503)).toBe("capacity");
  });

  it("other non-2xx (400/500/502) → error", () => {
    expect(poll.classifyPollStatus(400)).toBe("error");
    expect(poll.classifyPollStatus(502)).toBe("error");
    expect(poll.classifyPollStatus(500)).toBe("error");
  });
});

describe("nextPollState (honest {data, status, error} — critique §13)", () => {
  it("ok: adopts the fresh data and clears the error", () => {
    const prev = state<{ n: number }>({ data: { n: 1 }, status: "unavailable", error: "was down" });
    const next = poll.nextPollState(prev, { httpStatus: 200, data: { n: 2 } });
    expect(next).toEqual({ data: { n: 2 }, status: "ok", error: null });
  });

  it("session_gone (404): CLEARS the data — never keeps a ghost", () => {
    const prev = state<{ n: number }>({ data: { n: 1 }, status: "ok", error: null });
    const next = poll.nextPollState(prev, { httpStatus: 404 });
    expect(next.status).toBe("session_gone");
    expect(next.data).toBeUndefined();
  });

  it("unavailable (504): KEEPS the last data but marks it not-current with an error", () => {
    const prev = state<{ n: number }>({ data: { n: 1 }, status: "ok", error: null });
    const next = poll.nextPollState(prev, { httpStatus: 504 });
    expect(next.status).toBe("unavailable");
    expect(next.data).toEqual({ n: 1 });
    expect(next.error).not.toBeNull();
  });

  it("capacity (429/503): keeps data, surfaces the capacity status", () => {
    const prev = state<{ n: number }>({ data: { n: 1 }, status: "ok", error: null });
    const next = poll.nextPollState(prev, { httpStatus: 503 });
    expect(next.status).toBe("capacity");
    expect(next.data).toEqual({ n: 1 });
  });
});

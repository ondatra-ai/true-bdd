/**
 * Emergency-cleanup identity guard (plan §4.1 / finding 9). The suite-level
 * "last line of defense" (process-registry.emergencyKillAll) and killTree
 * must NEVER signal a process group whose leader pid was recycled. This
 * exercises the pure decision predicate — including a DELIBERATELY
 * mismatched recorded identity — plus a live-process round trip.
 */

import { describe, expect, it } from "vitest";

// The e2e suite moved to the repo-root self-contained package tests/harness/;
// this unit test still exercises that helper's pure functions.
import {
  maySignalIdentity,
  processStartIdentity,
} from "../../../tests/harness/helpers/start-identity";

describe("maySignalIdentity", () => {
  it("permits signalling only when the recorded identity matches the live one", () => {
    expect(maySignalIdentity("Tue Jan  1 00:00:00 2030", "Tue Jan  1 00:00:00 2030")).toBe(true);
  });

  it("REFUSES a mismatched identity (a recycled pid)", () => {
    expect(maySignalIdentity("Tue Jan  1 00:00:00 2030", "Wed Feb  2 11:11:11 2031")).toBe(false);
  });

  it("REFUSES when the recorded identity is missing", () => {
    expect(maySignalIdentity(undefined, "Tue Jan  1 00:00:00 2030")).toBe(false);
    expect(maySignalIdentity("", "Tue Jan  1 00:00:00 2030")).toBe(false);
  });

  it("REFUSES when the live identity cannot be read", () => {
    expect(maySignalIdentity("Tue Jan  1 00:00:00 2030", undefined)).toBe(false);
    expect(maySignalIdentity("Tue Jan  1 00:00:00 2030", "")).toBe(false);
  });

  it("guards a LIVE process: its own identity signals, a forged one does not", () => {
    const live = processStartIdentity(process.pid);
    expect(live).toBeDefined();

    // The real captured identity matches ⇒ may signal.
    expect(maySignalIdentity(live, processStartIdentity(process.pid))).toBe(true);

    // A DELIBERATELY different recorded identity (as if the pid were
    // recycled) ⇒ must NOT signal, even though the pid is alive.
    expect(maySignalIdentity(`${live}-recycled`, processStartIdentity(process.pid))).toBe(false);
  });
});

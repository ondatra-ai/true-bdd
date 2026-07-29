/**
 * Real-SQLite: idempotent migrations, WAL mode, and restart survival
 * (plan §4.6). Uses the production openDatabase against a temp file.
 */

import fs from "node:fs";
import os from "node:os";
import path from "node:path";

import { afterEach, describe, expect, it } from "vitest";

import { openDatabase } from "../../app/lib/db";
import { appendEvents, dispatchRun, registerSession, sessionDetail } from "../../app/lib/store";

const dirs: string[] = [];

function freshDbPath(): string {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), "harness-vitest-mig-"));
  dirs.push(dir);

  return path.join(dir, "harness.db");
}

afterEach(() => {
  for (const dir of dirs.splice(0)) {
    fs.rmSync(dir, { recursive: true, force: true });
  }
});

describe("db migrations + persistence", () => {
  it("opens in WAL mode", () => {
    const db = openDatabase(freshDbPath());
    try {
      expect(String(db.pragma("journal_mode", { simple: true })).toLowerCase()).toBe("wal");
    } finally {
      db.close();
    }
  });

  it("re-running migrations on the same file is a no-op (idempotent)", () => {
    const dbPath = freshDbPath();

    const first = openDatabase(dbPath);
    const sessionId = registerSession(first, {
      incarnation_id: "inc-idem",
      folder: "/host/a",
      canonical_folder: "/host/a",
      pid: 42,
      version: "v",
    }).session_id;
    first.close();

    // Reopening runs MIGRATIONS again — must not throw and must preserve data.
    const second = openDatabase(dbPath);
    try {
      const again = openDatabase(dbPath); // third pass, concurrent connection
      again.close();
      const detail = sessionDetail(second, sessionId);
      expect(detail?.id).toBe(sessionId);
    } finally {
      second.close();
    }
  });

  it("survives a restart: session + terminal run history persist across reopen", () => {
    const dbPath = freshDbPath();

    let db = openDatabase(dbPath);
    const sessionId = registerSession(db, {
      incarnation_id: "inc-restart",
      folder: "/host/b",
      canonical_folder: "/host/b",
      pid: 7,
      version: "v",
    }).session_id;
    const dispatch = dispatchRun(db, sessionId, { command: "version", fix: false, client_token: "tok-1" });
    expect(dispatch.kind).toBe("created");
    const runId = dispatch.kind === "created" ? dispatch.run_id : "";

    appendEvents(db, runId, [
      { seq: 1, type: "output", stream: "stdout", data: "true-bdd version 9.9.9\n" },
      { seq: 2, type: "terminal", outcome: "ok" },
    ]);
    db.close();

    // Reopen the SAME file (server restart) — same session id, run intact.
    db = openDatabase(dbPath);
    try {
      const detail = sessionDetail(db, sessionId);
      expect(detail?.id).toBe(sessionId);
      const run = detail?.runs.find((entry) => entry.id === runId);
      expect(run?.state).toBe("terminal");
      expect(run?.outcome).toBe("ok");
    } finally {
      db.close();
    }
  });
});

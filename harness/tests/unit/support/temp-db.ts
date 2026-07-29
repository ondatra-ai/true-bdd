/**
 * Real-SQLite test support (plan §4.6): a temp-file DB opened through the
 * PRODUCTION `openDatabase` (real better-sqlite3, real migrations, WAL) —
 * never a mock. Tests get an isolated file per case and can open a SECOND
 * connection to the SAME file to prove cross-connection behavior (the
 * racing-dispatch test).
 */

import { randomUUID } from "node:crypto";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";

import type Database from "better-sqlite3";

import { openDatabase } from "../../../app/lib/db";
import { registerSession } from "../../../app/lib/store";

export interface TempDb {
  db: Database.Database;
  path: string;
  dir: string;
}

/** Creates an isolated temp-file DB (migrated, WAL) plus its own connection. */
export function makeTempDb(): TempDb {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), "harness-vitest-"));
  const dbPath = path.join(dir, "harness.db");

  return { db: openDatabase(dbPath), path: dbPath, dir };
}

/** Opens an additional connection to an existing temp DB's file. */
export function openSecondConnection(temp: TempDb): Database.Database {
  return openDatabase(temp.path);
}

/** Closes the connection(s) and removes the temp directory. */
export function cleanup(temp: TempDb, ...extra: Database.Database[]): void {
  for (const connection of [temp.db, ...extra]) {
    try {
      connection.close();
    } catch {
      // Closing an already-closed connection is uninteresting.
    }
  }
  try {
    fs.rmSync(temp.dir, { recursive: true, force: true });
  } catch {
    // Best-effort cleanup.
  }
}

let seedCount = 0;

/**
 * Registers a session on `db` for `folder` and returns its id. A fresh
 * incarnation id per call yields a distinct session (same folder ⇒
 * sibling sessions). last_poll_at is now(), so the session is reachable.
 */
export function seedSession(db: Database.Database, folder = "/host/project", pid = 1000 + seedCount): string {
  seedCount += 1;
  const { session_id } = registerSession(db, {
    incarnation_id: `inc-${randomUUID()}`,
    folder,
    canonical_folder: folder,
    pid,
    version: "test-1.0",
  });

  return session_id;
}

/**
 * Browser-side thin client for the session-scoped workspace API (plan
 * Slice 0/5): `GET /api/sessions/:sid/docs{,/read}`, `POST
 * /api/sessions/:sid/docs/write`, `POST /api/sessions/:sid/chat`. Every call
 * is same-origin `fetch` — the browser supplies Origin/Host automatically,
 * satisfying the relay's origin/host policy without any manual header (the
 * `Origin` header is a forbidden header name a script cannot set anyway).
 */

import type { ChatMessage, ChatTurnResult, DocReadResult, DocTreeEntry } from "./types";

export interface SessionSummary {
  session_id: string;
  folder: string;
  pid: number;
  version: string;
  connected_at: number;
}

async function asJson<T>(response: Response): Promise<T> {
  return (await response.json()) as T;
}

export async function fetchSessions(): Promise<SessionSummary[]> {
  const res = await fetch("/api/sessions", { cache: "no-store" });
  if (!res.ok) {
    return [];
  }
  const body = await asJson<{ sessions: SessionSummary[] }>(res);

  return body.sessions;
}

export async function fetchDocTree(sid: string): Promise<DocTreeEntry[]> {
  const res = await fetch(`/api/sessions/${sid}/docs`, { cache: "no-store" });
  if (!res.ok) {
    return [];
  }
  const body = await asJson<{ docs: DocTreeEntry[] }>(res);

  return body.docs;
}

export async function fetchDocRead(sid: string, path: string): Promise<DocReadResult | null> {
  const url = `/api/sessions/${sid}/docs/read?path=${encodeURIComponent(path)}`;
  const res = await fetch(url, { cache: "no-store" });
  if (!res.ok) {
    return null;
  }

  return asJson<DocReadResult>(res);
}

export interface DocWriteResult {
  status: number;
  receipt?: { path: string; committed_revision: string; content_hash: string; work_id?: string; bytes?: number };
  error?: string;
}

export async function postDocWrite(
  sid: string,
  path: string,
  content: string,
  baseRevision: string,
  clientToken: string,
): Promise<DocWriteResult> {
  const res = await fetch(`/api/sessions/${sid}/docs/write`, {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify({ path, content, base_revision: baseRevision, client_token: clientToken }),
  });

  const body = (await res.json().catch(() => ({}))) as {
    receipt?: DocWriteResult["receipt"];
    error?: string;
  };

  return { status: res.status, receipt: body.receipt, error: body.error };
}

export async function postChat(
  sid: string,
  conversation: ChatMessage[],
  currentPath: string | null,
  currentContent: string | null,
): Promise<ChatTurnResult> {
  const res = await fetch(`/api/sessions/${sid}/chat`, {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify({ conversation, current_path: currentPath, current_content: currentContent }),
  });

  if (!res.ok) {
    return { reply_text: "", edit: null, error: `http_${res.status}` };
  }

  return asJson<ChatTurnResult>(res);
}

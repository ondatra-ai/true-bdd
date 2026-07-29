"use client";

import { useEffect, useState } from "react";

/**
 * Live-polls a JSON endpoint on an interval and returns the latest
 * value (plan §3.5: every view self-updates without a manual reload).
 * Fetch failures keep the last good value so a transient blip never
 * blanks the page. A `null` url disables polling — used when a URL
 * depends on a value not yet loaded (e.g. the run view derives the
 * session URL from run.session_id).
 */
export function usePoll<T>(url: string | null, intervalMs = 1000): T | undefined {
  const [data, setData] = useState<T | undefined>(undefined);

  useEffect(() => {
    if (url === null) {
      return;
    }

    let alive = true;
    // The next interval is scheduled AFTER each fetch completes (NICE-3),
    // so a slow request never overlaps the next tick. A monotonic request
    // id additionally guards against a late response applying stale state:
    // only the LATEST issued request may commit its result.
    let latest = 0;
    let timer: ReturnType<typeof setTimeout> | undefined;

    async function tick(): Promise<void> {
      const id = ++latest;
      try {
        const response = await fetch(url as string, { cache: "no-store" });
        if (response.ok) {
          const body = (await response.json()) as T;
          if (alive && id === latest) {
            setData(body);
          }
        }
      } catch {
        // Keep the last good value on a transient failure.
      } finally {
        if (alive) {
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
  }, [url, intervalMs]);

  return data;
}

/** POSTs JSON to url, ignoring the response body. */
export async function postJson(url: string, body: unknown): Promise<Response> {
  return fetch(url, {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify(body),
  });
}

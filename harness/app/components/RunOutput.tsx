"use client";

import type { OutputSegment } from "../lib/view-model/run";
import { MONO } from "./styles";

/**
 * The run output tail (plan §3.5/§3.6). Output chunks render verbatim; a
 * retention `gap` tombstone renders as a VISIBLE elision marker so a
 * truncated middle span is legible rather than a silent jump.
 */
export function RunOutput({ segments }: { segments: OutputSegment[] }) {
  return (
    <pre
      data-testid="run-output"
      style={{
        fontFamily: MONO,
        fontSize: 12,
        whiteSpace: "pre-wrap",
        wordBreak: "break-word",
        background: "#0f1720",
        color: "#e6edf3",
        padding: "0.75rem",
        borderRadius: 6,
        minHeight: 60,
        maxHeight: 460,
        overflowY: "auto",
      }}
    >
      {segments.map((segment, index) =>
        segment.kind === "gap" ? (
          <span key={`gap-${segment.seq}-${index}`} style={{ color: "#f0b34a", fontStyle: "italic" }}>
            {"\n"}
            {segment.text}
            {"\n"}
          </span>
        ) : (
          <span key={`out-${segment.seq}-${index}`}>{segment.text}</span>
        ),
      )}
    </pre>
  );
}

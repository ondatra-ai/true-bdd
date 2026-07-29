/**
 * Shared inline-style tokens for the harness views. This is an
 * engineering harness, not a showcase: a small, monospace-friendly,
 * theme-neutral set of styles keeps every state legible without a CSS
 * pipeline. Kept as plain objects so the presentational components stay
 * self-contained.
 */

import type { CSSProperties } from "react";

export const MONO = "ui-monospace, SFMono-Regular, Menlo, Consolas, monospace";

export const page: CSSProperties = {
  fontFamily: MONO,
  fontSize: 13,
  lineHeight: 1.5,
  padding: "1rem",
  maxWidth: 1100,
  margin: "0 auto",
  color: "#1a1a1a",
};

export const h1: CSSProperties = { fontSize: 16, fontWeight: 700, margin: "0.25rem 0 0.75rem" };
export const h2: CSSProperties = { fontSize: 13, fontWeight: 700, margin: "1.25rem 0 0.5rem", textTransform: "uppercase", letterSpacing: 0.5, color: "#555" };

export const chip: CSSProperties = {
  display: "inline-flex",
  alignItems: "center",
  gap: 6,
  border: "1px solid #bbb",
  borderRadius: 4,
  padding: "2px 8px",
  fontSize: 12,
  background: "#fafafa",
};

export const flag: CSSProperties = {
  display: "inline-block",
  border: "1px solid #d09",
  color: "#a05",
  borderRadius: 3,
  padding: "0 5px",
  fontSize: 11,
  marginLeft: 6,
  whiteSpace: "nowrap",
};

export const button: CSSProperties = {
  fontFamily: MONO,
  fontSize: 12,
  padding: "3px 9px",
  border: "1px solid #789",
  borderRadius: 4,
  background: "#eef3f8",
  cursor: "pointer",
};

export const banner: CSSProperties = {
  border: "1px solid #d90",
  background: "#fff8e6",
  color: "#7a5200",
  borderRadius: 4,
  padding: "0.5rem 0.75rem",
  margin: "0.5rem 0",
  fontSize: 12,
};

export const table: CSSProperties = { borderCollapse: "collapse", width: "100%", fontSize: 12 };
export const cell: CSSProperties = { border: "1px solid #ddd", padding: "4px 8px", textAlign: "left", verticalAlign: "top" };

/** A status-driven dot/label color for the document + story cells. */
export function statusColor(status: string): string {
  switch (status) {
    case "present":
    case "one":
    case "counted":
    case "parseable":
      return "#127c2b";
    case "missing":
      return "#888";
    case "invalid":
    case "not_a_dir":
      return "#c0341d";
    case "ambiguous":
    case "unknown":
      return "#b26a00";
    case "present_empty":
      return "#5a5a8a";
    default:
      return "#333";
  }
}

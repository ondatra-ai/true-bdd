import React from "react";

/** Numbered section label, matching the guideline's own "01—Typography" convention. */
export function SectionLabel({ index, children, tone = "dark", style, ...rest }) {
  return (
    <div
      style={{
        display: "flex",
        alignItems: "baseline",
        gap: "var(--space-2)",
        font: "var(--type-caption)",
        fontWeight: "var(--fw-bold)",
        textTransform: "uppercase",
        letterSpacing: "var(--ls-label)",
        color: tone === "light" ? "var(--gray-100)" : "var(--text-primary)",
        ...style
      }}
      {...rest}
    >
      {index != null ? <span>{String(index).padStart(2, "0")}—</span> : null}
      <span>{children}</span>
    </div>
  );
}

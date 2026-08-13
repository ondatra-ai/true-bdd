import React from "react";
import { Wordmark } from "./Wordmark.jsx";

/** Both marks in one horizontal lockup, equal visual weight, separated by a minimal "×". */
export function PartnerLockup({ partner = "Partner", brand = "S&F", size = 40, tone = "dark", style, ...rest }) {
  const ink = tone === "light" ? "var(--white-999)" : "var(--black-999)";
  return (
    <div
      style={{ display: "flex", alignItems: "center", gap: `calc(${size}px * 0.6)`, ...style }}
      {...rest}
    >
      <Wordmark text={brand} size={size} tone={tone} />
      <span
        aria-hidden="true"
        style={{
          fontFamily: "var(--font-display)",
          fontWeight: "var(--fw-medium)",
          fontSize: size * 0.55 + "px",
          lineHeight: 1,
          color: ink,
          opacity: 0.5
        }}
      >
        ×
      </span>
      <Wordmark text={partner} size={size} tone={tone} />
    </div>
  );
}

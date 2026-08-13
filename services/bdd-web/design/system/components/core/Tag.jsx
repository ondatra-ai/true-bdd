import React from "react";

/** Small uppercase metadata marker. Sharp corners, hairline border, no fill by default. */
export function Tag({ children, tone = "neutral", style, ...rest }) {
  const tones = {
    neutral: { color: "var(--text-primary)", borderColor: "var(--black-999)", background: "transparent" },
    inverse: { color: "var(--white-999)", borderColor: "var(--white-999)", background: "transparent" },
    solid: { color: "var(--white-999)", borderColor: "var(--black-999)", background: "var(--black-999)" },
    error: { color: "var(--red-500)", borderColor: "var(--red-500)", background: "transparent" },
    success: { color: "var(--green-500)", borderColor: "var(--green-500)", background: "transparent" },
    warning: { color: "var(--yellow-500)", borderColor: "var(--yellow-500)", background: "transparent" }
  }[tone];
  return (
    <span
      style={{
        display: "inline-flex",
        alignItems: "center",
        font: "var(--type-caption)",
        fontWeight: "var(--fw-bold)",
        textTransform: "uppercase",
        letterSpacing: "var(--ls-label)",
        padding: "6px 12px",
        border: "var(--border-width) solid",
        borderRadius: "var(--radius-none)",
        ...tones,
        ...style
      }}
      {...rest}
    >
      {children}
    </span>
  );
}

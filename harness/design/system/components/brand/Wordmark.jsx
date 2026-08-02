import React from "react";

/** S&F has no supplied logo file — the wordmark is set in plain Poppins Bold type. */
export function Wordmark({ text = "S&F", size = 32, tone = "dark", gradient = false, style, ...rest }) {
  return (
    <span
      style={{
        fontFamily: "var(--font-display)",
        fontWeight: "var(--fw-bold)",
        fontSize: typeof size === "number" ? size + "px" : size,
        lineHeight: "var(--lh-base)",
        letterSpacing: "var(--ls-tight)",
        color: tone === "light" ? "var(--white-999)" : "var(--black-999)",
        ...(gradient
          ? {
              background: "var(--gradient-spiral-full)",
              WebkitBackgroundClip: "text",
              backgroundClip: "text",
              color: "transparent"
            }
          : null),
        ...style
      }}
      {...rest}
    >
      {text}
    </span>
  );
}

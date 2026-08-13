import React from "react";

/** Minimal editorial card: oversized bold headline clamped to two lines + small supporting paragraph. */
export function Card({
  headline,
  body,
  eyebrow,
  headlineSize = "var(--fs-h3)",
  tone = "light",
  padding = "var(--pad-card)",
  children,
  style,
  ...rest
}) {
  const tones = {
    light: { background: "var(--surface-card)", color: "var(--text-body)", borderColor: "var(--black-999)" },
    subtle: { background: "var(--gray-100)", color: "var(--text-body)", borderColor: "var(--black-999)" },
    dark: { background: "var(--black-999)", color: "var(--gray-100)", borderColor: "var(--black-999)" }
  }[tone];
  return (
    <article
      style={{
        display: "flex",
        flexDirection: "column",
        gap: "var(--space-4)",
        padding,
        border: "var(--border-width) solid",
        borderRadius: "var(--radius-none)",
        background: tones.background,
        color: tones.color,
        borderColor: tones.borderColor,
        minWidth: 0,
        ...style
      }}
      {...rest}
    >
      {eyebrow ? (
        <span style={{ font: "var(--type-caption)", textTransform: "uppercase", letterSpacing: "var(--ls-label)", color: tone === "dark" ? "var(--gray-600)" : "var(--text-muted)" }}>
          {eyebrow}
        </span>
      ) : null}
      {headline ? (
        <h3
          style={{
            font: "var(--type-h3)",
            fontSize: headlineSize,
            color: tone === "dark" ? "var(--white-999)" : "var(--text-primary)",
            letterSpacing: "var(--ls-tight)",
            display: "-webkit-box",
            WebkitLineClamp: 2,
            WebkitBoxOrient: "vertical",
            overflow: "hidden",
            margin: 0
          }}
        >
          {headline}
        </h3>
      ) : null}
      {body ? <p style={{ font: "var(--type-body)", margin: 0, maxWidth: "var(--measure-paragraph)" }}>{body}</p> : null}
      {children}
    </article>
  );
}

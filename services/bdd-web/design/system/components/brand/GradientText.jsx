import React from "react";

/** Colours a word or phrase from the spiral gradient. Used sparingly — one accent per view. */
export function GradientText({ children, as = "span", style, ...rest }) {
  const Tag = as;
  return (
    <Tag
      style={{
        background: "var(--gradient-spiral-full)",
        WebkitBackgroundClip: "text",
        backgroundClip: "text",
        color: "transparent",
        ...style
      }}
      {...rest}
    >
      {children}
    </Tag>
  );
}

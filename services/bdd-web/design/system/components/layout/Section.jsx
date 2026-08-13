import React from "react";

/** Full-bleed section band. Sections are separated by inversion and vertical rhythm, never by rules or radii. */
export function Section({ children, tone = "light", pad = "var(--section-y)", divider = false, style, ...rest }) {
  const tones = {
    light: { background: "var(--white-999)", color: "var(--text-body)" },
    subtle: { background: "var(--gray-100)", color: "var(--text-body)" },
    dark: { background: "var(--black-999)", color: "var(--gray-100)" },
    ink: { background: "var(--gray-999)", color: "var(--gray-100)" }
  }[tone];
  return (
    <section
      style={{
        paddingBlock: pad,
        borderTop: divider ? "var(--border-width) solid var(--border-hairline)" : undefined,
        ...tones,
        ...style
      }}
      {...rest}
    >
      {children}
    </section>
  );
}

import React from "react";

/** Swiss 12-column grid: margin 0, gutter 20. Children position with GridItem or their own span. */
export function Grid({ children, columns = 12, gutter = "var(--grid-gutter)", style, ...rest }) {
  return (
    <div
      style={{
        display: "grid",
        gridTemplateColumns: `repeat(${columns}, minmax(0, 1fr))`,
        gap: gutter,
        paddingInline: "var(--grid-margin)",
        ...style
      }}
      {...rest}
    >
      {children}
    </div>
  );
}

export function GridItem({ span = 12, start, children, style, ...rest }) {
  return (
    <div
      style={{
        gridColumn: start ? `${start} / span ${span}` : `span ${span}`,
        minWidth: 0,
        ...style
      }}
      {...rest}
    >
      {children}
    </div>
  );
}

import React from "react";

/** Buttons sit in one horizontal row with NO space between them — they share edges. */
export function ButtonRow({ children, full = false, style, ...rest }) {
  const items = React.Children.toArray(children);
  return (
    <div
      style={{
        display: "flex",
        gap: 0,
        alignItems: "stretch",
        width: full ? "100%" : "fit-content",
        ...style
      }}
      {...rest}
    >
      {items.map((child, i) =>
        React.isValidElement(child)
          ? React.cloneElement(child, {
              key: i,
              style: {
                flex: full ? "1 1 0" : "0 0 auto",
                marginLeft: i === 0 ? 0 : "calc(-1 * var(--border-width))",
                ...child.props.style
              }
            })
          : child
      )}
    </div>
  );
}

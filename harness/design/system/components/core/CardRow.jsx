import React from "react";

/** Equal-width cards butted directly together — no gaps, shared borders. */
export function CardRow({ children, columns, style, ...rest }) {
  const items = React.Children.toArray(children);
  const count = columns || items.length || 1;
  return (
    <div
      style={{
        display: "grid",
        gridTemplateColumns: `repeat(${count}, minmax(0, 1fr))`,
        gap: 0,
        ...style
      }}
      {...rest}
    >
      {items.map((child, i) =>
        React.isValidElement(child)
          ? React.cloneElement(child, {
              key: i,
              style: { marginLeft: i === 0 ? 0 : "calc(-1 * var(--border-width))", ...child.props.style }
            })
          : child
      )}
    </div>
  );
}

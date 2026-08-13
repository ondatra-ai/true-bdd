import React from "react";

const base = {
  font: "var(--type-button)",
  textTransform: "uppercase",
  letterSpacing: "var(--ls-button)",
  border: "var(--border-width) solid var(--black-999)",
  borderRadius: "var(--radius-none)",
  padding: "var(--pad-button-y) var(--pad-button-x)",
  minHeight: "68px",
  display: "inline-flex",
  alignItems: "center",
  justifyContent: "center",
  gap: "var(--space-2)",
  cursor: "pointer",
  textAlign: "center",
  transition: "var(--transition-interactive)",
  appearance: "none",
  whiteSpace: "nowrap"
};

const variants = {
  primary: { background: "var(--black-999)", color: "var(--white-999)" },
  secondary: { background: "var(--white-999)", color: "var(--black-999)" },
  ghost: { background: "transparent", color: "var(--black-999)", borderColor: "transparent" },
  inverse: { background: "var(--white-999)", color: "var(--black-999)", borderColor: "var(--white-999)" }
};

const hovers = {
  primary: { background: "var(--white-999)", color: "var(--black-999)" },
  secondary: { background: "var(--black-999)", color: "var(--white-999)" },
  ghost: { background: "var(--gray-100)", color: "var(--black-999)" },
  inverse: { background: "transparent", color: "var(--white-999)" }
};

const sizes = {
  sm: { minHeight: "48px", padding: "14px 24px", fontSize: "14px" },
  md: {},
  lg: { minHeight: "88px", padding: "32px 56px", fontSize: "20px" }
};

export function Button({
  children,
  variant = "primary",
  size = "md",
  full = false,
  disabled = false,
  href,
  onClick,
  style,
  ...rest
}) {
  const [hover, setHover] = React.useState(false);
  const [press, setPress] = React.useState(false);
  const Tag = href ? "a" : "button";
  const resolved = {
    ...base,
    ...variants[variant],
    ...sizes[size],
    ...(hover && !disabled ? hovers[variant] : null),
    ...(press && !disabled ? { transform: "translateY(1px)" } : null),
    ...(full ? { display: "flex", width: "100%" } : null),
    ...(disabled ? { opacity: 0.35, cursor: "not-allowed", pointerEvents: "none" } : null),
    ...style
  };
  return (
    <Tag
      href={href}
      onClick={onClick}
      disabled={Tag === "button" ? disabled : undefined}
      style={resolved}
      onMouseEnter={() => setHover(true)}
      onMouseLeave={() => { setHover(false); setPress(false); }}
      onMouseDown={() => setPress(true)}
      onMouseUp={() => setPress(false)}
      {...rest}
    >
      {children}
    </Tag>
  );
}

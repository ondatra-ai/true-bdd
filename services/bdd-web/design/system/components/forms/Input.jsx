import React from "react";

/** Square-cornered text field. Border is the only chrome; focus darkens it rather than glowing. */
export function Input({ label, placeholder, type = "text", tone = "light", value, onChange, style, ...rest }) {
  const [focus, setFocus] = React.useState(false);
  const light = tone === "light";
  return (
    <label style={{ display: "flex", flexDirection: "column", gap: "var(--space-2)", ...style }}>
      {label ? (
        <span style={{ font: "var(--type-caption)", textTransform: "uppercase", letterSpacing: "var(--ls-label)", fontWeight: "var(--fw-bold)", color: light ? "var(--text-muted)" : "var(--gray-600)" }}>
          {label}
        </span>
      ) : null}
      <input
        type={type}
        placeholder={placeholder}
        value={value}
        onChange={onChange}
        onFocus={() => setFocus(true)}
        onBlur={() => setFocus(false)}
        style={{
          font: "var(--type-body)",
          padding: "var(--space-3)",
          background: "transparent",
          color: light ? "var(--text-primary)" : "var(--white-999)",
          border: "var(--border-width) solid",
          borderColor: focus ? (light ? "var(--black-999)" : "var(--white-999)") : (light ? "var(--gray-600)" : "var(--gray-999)"),
          borderRadius: "var(--radius-none)",
          outline: "none",
          transition: "var(--transition-interactive)"
        }}
        {...rest}
      />
    </label>
  );
}

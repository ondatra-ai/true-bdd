import React from "react";

/** Blurred radial colour field — the only place colour beyond the monochrome ramp appears at scale. */
export function GradientField({
  variant = "radial",
  intensity = 1,
  children,
  height = 480,
  style,
  ...rest
}) {
  // Asset paths live here, not in a token: a url() inside a custom property resolves
  // against the stylesheet, which 404s for consumers whose page is not at the root.
  const root = React.useMemo(() => {
    const link = Array.from(document.querySelectorAll('link[rel="stylesheet"]')).find((l) =>
      /styles\.css(\?|$)/.test(l.getAttribute("href") || "")
    );
    return link ? new URL(".", link.href).href : "./";
  }, []);
  const assets = {
    radial: root + "assets/gradients/gradient-field-radial.png",
    soft: root + "assets/gradients/gradient-field-soft.png"
  };
  const css = {
    radial: "var(--gradient-spiral)",
    soft: "var(--gradient-spiral-soft)"
  };
  const src = assets[variant];
  const layer = src
    ? { backgroundImage: `url("${src}")`, backgroundSize: "cover", backgroundPosition: "center" }
    : { background: css[variant] || css.radial };
  return (
    <div
      style={{
        position: "relative",
        overflow: "hidden",
        background: "var(--white-999)",
        height: typeof height === "number" ? height + "px" : height,
        ...style
      }}
      {...rest}
    >
      <div
        aria-hidden="true"
        style={{
          position: "absolute",
          inset: 0,
          opacity: intensity,
          ...layer
        }}
      />
      <div style={{ position: "relative", height: "100%" }}>{children}</div>
    </div>
  );
}

import * as React from "react";

/** Full-bleed vertical band. Sections separate by tone inversion and rhythm, not by rules. */
export interface SectionProps extends React.HTMLAttributes<HTMLElement> {
  children?: React.ReactNode;
  tone?: "light" | "subtle" | "dark" | "ink";
  /** Vertical padding; defaults to var(--section-y) = 120px. */
  pad?: string;
  divider?: boolean;
}
export declare function Section(props: SectionProps): JSX.Element;

import * as React from "react";

/**
 * Minimal editorial card: oversized bold headline clamped to two lines + a smaller supporting paragraph.
 */
export interface CardProps extends React.HTMLAttributes<HTMLElement> {
  /** Bold oversized headline. Write it so it breaks at two lines. */
  headline?: React.ReactNode;
  /** Supporting paragraph, left aligned, ~44ch measure. */
  body?: React.ReactNode;
  /** Optional small uppercase label above the headline. */
  eyebrow?: React.ReactNode;
  /** Override the headline size for hero-scale cards, e.g. "var(--fs-h2)". */
  headlineSize?: string;
  tone?: "light" | "subtle" | "dark";
  /** Inner padding; defaults to var(--pad-card) = 40px. */
  padding?: string;
  children?: React.ReactNode;
}
export declare function Card(props: CardProps): JSX.Element;

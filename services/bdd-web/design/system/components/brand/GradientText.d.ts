import * as React from "react";

/** Fills a word or short phrase with the spiral gradient. Selective typographic colouring only. */
export interface GradientTextProps extends React.HTMLAttributes<HTMLElement> {
  children?: React.ReactNode;
  /** Element to render; defaults to "span". */
  as?: keyof JSX.IntrinsicElements;
}
export declare function GradientText(props: GradientTextProps): JSX.Element;

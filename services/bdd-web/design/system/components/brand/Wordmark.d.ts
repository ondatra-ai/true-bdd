import * as React from "react";

/**
 * The S&F wordmark, set in Poppins Bold type. No logo file exists in the supplied sources.
 */
export interface WordmarkProps extends React.HTMLAttributes<HTMLSpanElement> {
  /** Defaults to "S&F". */
  text?: string;
  size?: number | string;
  tone?: "dark" | "light";
  /** Fill the letterforms with the spiral gradient — one accent per view, maximum. */
  gradient?: boolean;
}
export declare function Wordmark(props: WordmarkProps): JSX.Element;

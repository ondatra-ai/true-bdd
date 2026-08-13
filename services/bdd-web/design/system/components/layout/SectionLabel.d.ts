import * as React from "react";

/** Numbered section label in the guideline's own "01—Typography" style. */
export interface SectionLabelProps extends React.HTMLAttributes<HTMLDivElement> {
  /** Zero-padded automatically: 1 renders "01—". */
  index?: number | string;
  children?: React.ReactNode;
  tone?: "dark" | "light";
}
export declare function SectionLabel(props: SectionLabelProps): JSX.Element;

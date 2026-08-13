import * as React from "react";

/** Horizontal row of buttons with no space between them — adjacent buttons share a single border. */
export interface ButtonRowProps extends React.HTMLAttributes<HTMLDivElement> {
  children?: React.ReactNode;
  /** Buttons divide the full container width equally. */
  full?: boolean;
}
export declare function ButtonRow(props: ButtonRowProps): JSX.Element;

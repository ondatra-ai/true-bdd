import * as React from "react";

/** Equal-width cards butted directly together: no gutter, shared borders. */
export interface CardRowProps extends React.HTMLAttributes<HTMLDivElement> {
  children?: React.ReactNode;
  /** Column count; defaults to the number of children. */
  columns?: number;
}
export declare function CardRow(props: CardRowProps): JSX.Element;

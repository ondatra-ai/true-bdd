import * as React from "react";

/** Swiss 12-column grid: margin 0, gutter 20px. */
export interface GridProps extends React.HTMLAttributes<HTMLDivElement> {
  children?: React.ReactNode;
  columns?: number;
  gutter?: string;
}
export declare function Grid(props: GridProps): JSX.Element;

/** A column span inside Grid. */
export interface GridItemProps extends React.HTMLAttributes<HTMLDivElement> {
  span?: number;
  /** 1-based start column. */
  start?: number;
  children?: React.ReactNode;
}
export declare function GridItem(props: GridItemProps): JSX.Element;

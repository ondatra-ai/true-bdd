import * as React from "react";

/** Small uppercase metadata marker: hairline border, sharp corners, usually unfilled. */
export interface TagProps extends React.HTMLAttributes<HTMLSpanElement> {
  children?: React.ReactNode;
  tone?: "neutral" | "inverse" | "solid" | "error" | "success" | "warning";
}
export declare function Tag(props: TagProps): JSX.Element;

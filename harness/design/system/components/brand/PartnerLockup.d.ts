import * as React from "react";

/** Both marks in one horizontal lockup at equal visual weight, separated by a minimal "×". */
export interface PartnerLockupProps extends React.HTMLAttributes<HTMLDivElement> {
  /** Partner name, set in the same face and size as the brand. */
  partner?: string;
  brand?: string;
  size?: number;
  tone?: "dark" | "light";
}
export declare function PartnerLockup(props: PartnerLockupProps): JSX.Element;

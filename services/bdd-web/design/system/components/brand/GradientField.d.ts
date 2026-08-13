import * as React from "react";

/**
 * Blurred radial colour field used as a background band. Edgeless, desaturated, calm.
 */
export interface GradientFieldProps extends React.HTMLAttributes<HTMLDivElement> {
  /** radial = brand asset with a saturated lower-right focal area; soft = lighter, more diffuse. */
  variant?: "radial" | "soft";
  /** 0–1 opacity of the field over white. Keep at or below 1; lower it behind dense text. */
  intensity?: number;
  height?: number | string;
  children?: React.ReactNode;
}
export declare function GradientField(props: GradientFieldProps): JSX.Element;
